package commands

import (
	"github.com/spf13/cobra"
)

// RootCmd is the root command for KSYNC.
var rootCmd = &cobra.Command{
	Use:   "ksync",
	Short: "Fast Sync validated and archived blocks from KYVE to every Tendermint based Blockchain Application",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}