package main

import (
	"fmt"
	"os"

	"github.com/kopecmaciej/vi-sql/cmd"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "\nERROR: Application crashed unexpectedly: %v\n", r)
			fmt.Fprintf(os.Stderr, "Please check the log file for details\n")
			os.Exit(1)
		}
	}()

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
