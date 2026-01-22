package main

import (
	"os"

	"github.com/yagizdagabak/vibespace/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
