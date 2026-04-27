package cmd

import (
	"bufio"
	"os"
	runtimeDebug "runtime/debug"
	"strings"

	"fmt"

	"github.com/kopecmaciej/vi-sql/internal/build"
	"github.com/kopecmaciej/vi-sql/internal/config"
	_ "github.com/kopecmaciej/vi-sql/internal/postgres"
	_ "github.com/kopecmaciej/vi-sql/internal/sqlite"
	"github.com/kopecmaciej/vi-sql/internal/tui"
	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	cfgFile             string
	showVersion         bool
	debug               bool
	optionsPage         bool
	connectionPage      bool
	connectionName      string
	listConnections     bool
	jumpInto            string
	resetMasterPassword bool
	rootCmd             = &cobra.Command{
		Use:   "vi-sql",
		Short: "SQL TUI client",
		Long:  `A Terminal User Interface (TUI) client for SQL databases (PostgreSQL)`,
		Run:   runApp,
	}
)

func Execute() error {
	err := rootCmd.Execute()
	if err != nil {
		return err
	}
	return nil
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/.config/vi-sql/config.yaml)")
	rootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "Show version")
	rootCmd.Flags().BoolVarP(&debug, "debug", "d", false, "Enable debug mode")
	rootCmd.Flags().BoolVarP(&optionsPage, "options-page", "o", false, "Show options page on startup")
	rootCmd.Flags().BoolVarP(&connectionPage, "connection-page", "p", false, "Show connection page on startup")
	rootCmd.Flags().StringVarP(&connectionName, "connection-name", "n", "", "Connect to a specific connection by name")
	rootCmd.Flags().BoolVarP(&listConnections, "connection-list", "l", false, "List all available connections")
	rootCmd.Flags().Bool("gen-key", false, "Generate a random encryption key for use with VI_SQL_SECRET_KEY")
	rootCmd.Flags().StringVarP(&jumpInto, "jump", "j", "", "Jump directly to schema/table (format: schema-name/table-name)")
	rootCmd.Flags().BoolVar(&resetMasterPassword, "reset-master-password", false, "Reset master password (clears wrapped key and erases encrypted connection passwords)")
}

func runApp(cmd *cobra.Command, args []string) {
	if showVersion {
		greenColor := "\033[32m"
		resetColor := "\033[0m"
		fmt.Printf("%s\n", greenColor)
		fmt.Print(`
╔═══════════════════════════════════════╗
║  ██╗   ██╗██╗ ███████╗ ██████╗ ██╗    ║
║  ██║   ██║██║ ██╔════╝██╔═══██╗██║    ║
║  ██║   ██║██║ ███████╗██║   ██║██║    ║
║  ╚██╗ ██╔╝██║ ╚════██║██║▄▄ ██║██║    ║
║   ╚████╔╝ ██║ ███████║╚██████╔╝███████║
║    ╚═══╝  ╚═╝ ╚══════╝ ╚══▀▀═╝ ╚══════║
╚═══════════════════════════════════════╝
`)
		fmt.Printf("Version %s%s\n", build.Version, resetColor)
		os.Exit(0)
	}

	if err := util.ValidateConfigPath(cfgFile); err != nil {
		fatalf("%v", err)
	}

	cfg, err := config.LoadConfigWithVersion(build.Version, cfgFile)
	if err != nil {
		fatalf("loading config: %v", err)
	}

	if cfg.FirstLaunch {
		cfg.ShowOptionsPage = true
		cfg.ShowConnectionPage = false
	}

	cmd.Flags().Visit(func(f *pflag.Flag) {
		switch f.Name {
		case "options-page":
			cfg.ShowOptionsPage = optionsPage
		case "connection-page":
			cfg.ShowConnectionPage = connectionPage
		case "connection-list":
			listAvailableConnections(cfg)
			os.Exit(0)
		case "connection-name":
			found := false
			for _, conn := range cfg.Connections {
				if conn.Name == connectionName {
					found = true
					cfg.CurrentConnection = connectionName
					cfg.ShowConnectionPage = false
					break
				}
			}
			if !found {
				fatalf("Connection '%s' not found. Use --list or -l to see available connections.", connectionName)
			}
		case "gen-key":
			util.PrintEncryptionKeyInstructions()
			os.Exit(0)
		case "jump":
			if jumpInto != "" {
				if err := validateDirectNavigateFormat(jumpInto); err != nil {
					fatalf("invalid jump format: %v", err)
				}
				cfg.JumpInto = jumpInto
				cfg.ShowConnectionPage = false
				cfg.ShowOptionsPage = false
			} else {
				fatalf("jump value cannot be empty")
			}
		case "reset-master-password":
			runResetMasterPassword(cfg)
			os.Exit(0)
		}
	})

	// Master-mode loading is deferred to the TUI so the user is prompted via
	// an in-app modal instead of a raw terminal prompt.
	if cfg.Security.Method != config.SecurityMethodMaster {
		if err := cfg.LoadEncryptionKey(); err != nil {
			fatalf("loading encryption key: %v", err)
		}
	}

	logLevel := zerolog.InfoLevel
	if debug {
		logLevel = zerolog.DebugLevel
	}

	logFile := logging(cfg.Log.Path, logLevel)
	defer func() {
		err := logFile.Close()
		if err != nil {
			fmt.Printf("\nError closing log file %s, error: %s", cfg.Log.Path, err)
		}
	}()
	defer func() {
		if r := recover(); r != nil {
			log.Error().
				Interface("panic", r).
				Str("stack", string(runtimeDebug.Stack())).
				Msg("Application panicked")

			fmt.Fprintf(os.Stderr, "\nERROR: Application crashed unexpectedly\n")
			fmt.Fprintf(os.Stderr, "Details have been logged to: %s\n", cfg.Log.Path)
			os.Exit(1)
		}
	}()

	if debug {
		log.Debug().Msg("Debug mode enabled")
	}
	log.Info().Msg("Vi-SQL started")

	if os.Getenv("ENV") == "vi-dev" {
		log.Info().Msg("Dev mode enabled, keys and styles will be loaded from default values")
	}

	app := tui.NewApp(cfg)
	err = app.Init()
	if err != nil {
		log.Fatal().Err(err).Msg("Error initializing app")
	}
	app.Render()
	err = app.Run()
	if err != nil {
		log.Fatal().Err(err).Msg("Error running app")
	}
}

func listAvailableConnections(cfg *config.Config) {
	if len(cfg.Connections) == 0 {
		fmt.Println("No connections available. Use the app to add connections.")
		return
	}

	maxNameLength := 4
	for _, conn := range cfg.Connections {
		if len(conn.Name) > maxNameLength {
			maxNameLength = len(conn.Name)
		}
	}

	maxNameLength += 2

	fmt.Println("Available connections:")
	fmt.Printf("%-2s %-*s %s\n", "", maxNameLength, "NAME", "URL")
	fmt.Printf("%-2s %-*s %s\n", "", maxNameLength, "----", "---")

	for _, conn := range cfg.Connections {
		currentMark := " "
		if cfg.CurrentConnection == conn.Name {
			currentMark = "*"
		}
		fmt.Printf("%s %-*s %s\n", currentMark, maxNameLength, conn.Name, conn.GetSafeDSN())
	}

	fmt.Println("\n* Current connection")
}

func logging(path string, logLevel zerolog.Level) *os.File {
	logFile, err := os.OpenFile(path, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		if os.IsNotExist(err) {
			logFile, err = os.Create(path)
			if err != nil {
				log.Fatal().Err(err).Msg("Error creating log file")
			}
		} else {
			log.Fatal().Err(err).Msg("Error opening log file")
		}
	}

	zerolog.SetGlobalLevel(logLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: logFile}).With().Caller().Logger()

	return logFile
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", args...)
	os.Exit(1)
}

func runResetMasterPassword(cfg *config.Config) {
	if !cfg.IsMasterConfigured() {
		fmt.Println("Master password is not configured — nothing to reset.")
		return
	}

	masterCount := 0
	for _, conn := range cfg.Connections {
		if util.ParseMethodTag(conn.Password) == config.SecurityMethodMaster {
			masterCount++
		}
	}

	fmt.Printf("Reset master password? This clears the wrapped key.\n")
	fmt.Printf("%d master-encrypted connection password(s) will be erased; host/user/db are kept. Passwords stored under other methods (keyring, env) are unaffected.\n", masterCount)
	fmt.Print("Type 'y' or 'yes' to confirm: ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
	default:
		fmt.Println("Aborted.")
		return
	}

	if err := cfg.ApplyMasterReset(); err != nil {
		fatalf("resetting master password: %v", err)
	}
	fmt.Println("Master password reset. Run vi-sql to set a new one.")
}

func validateDirectNavigateFormat(format string) error {
	parts := strings.Split(format, "/")
	if len(parts) != 2 {
		return fmt.Errorf("format should be schema-name/table-name")
	}
	if len(parts[0]) == 0 || len(parts[1]) == 0 {
		return fmt.Errorf("both schema-name and table-name are required")
	}
	return nil
}
