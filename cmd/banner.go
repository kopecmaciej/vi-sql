package cmd

import (
	"fmt"

	"github.com/kopecmaciej/vi-sql/internal/build"
)

func printVersion() {
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
}
