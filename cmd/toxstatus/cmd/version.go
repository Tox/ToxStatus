package cmd

import (
	"fmt"
	"os"

	"github.com/Tox/ToxStatus/internal/version"
	sqlite3 "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
)

var (
	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Version information",
		Run:   startVersion,
	}
)

func init() {
	Root.AddCommand(versionCmd)
}

func startVersion(cmd *cobra.Command, args []string) {
	vs, err := version.String()
	if err != nil {
		exitWithError(err.Error())
		return
	}

	sqliteVersion, _, _ := sqlite3.Version()

	fmt.Print(vs)
	if ts := version.HumanRevisionTime(); ts != "" {
		fmt.Printf(" (%s)", ts)
	}
	fmt.Println()
	fmt.Printf("sqlite: %s \n", sqliteVersion)
	fmt.Println("https://github.com/Tox/ToxStatus (GPLv3)")
}

func exitWithError(s string) {
	fmt.Fprintf(os.Stderr, "error: %s\n", s)
	os.Exit(1)
}
