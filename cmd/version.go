package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Version string

func newVersionCmd() *cobra.Command {
	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Run: func(cmd *cobra.Command, args []string) {
			if Version == "" {
				Version = "unknown"
			}
			fmt.Println("Version: ", Version)
		},
	}
	return versionCmd
}
