package main

import (
	"os"

	"github.com/Tox/ToxStatus/cmd/toxstatus/cmd"
)

func main() {
	if err := cmd.Root.Execute(); err != nil {
		os.Exit(1)
	}
}
