package commands

import (
	"fmt"
	"github.com/KYVENetwork/ksync/engines"
	"github.com/KYVENetwork/ksync/utils"
	"github.com/spf13/cobra"
	"os"
)

func init() {
	resetCmd.Flags().StringVarP(&engine, "engine", "e", "", fmt.Sprintf("consensus engine of the binary by default %s is used, list all engines with \"ksync engines\"", utils.DefaultEngine))

	resetCmd.Flags().StringVar(&homePath, "home", "", "home directory")
	if err := resetCmd.MarkFlagRequired("home"); err != nil {
		panic(fmt.Errorf("flag 'home' should be required: %w", err))
	}

	resetCmd.Flags().BoolVar(&keepAddrBook, "keep-addr-book", true, "keep the address book intact")

	resetCmd.Flags().BoolVar(&optOut, "opt-out", false, "disable the collection of anonymous usage data")

	rootCmd.AddCommand(resetCmd)
}

var resetCmd = &cobra.Command{
	Use:   "reset-all",
	Short: "Removes all the data and WAL, reset this node's validator to genesis state",
	Run: func(cmd *cobra.Command, args []string) {
		utils.TrackResetEvent(optOut)

		if err := engines.EngineFactory(engine, homePath, rpcServerPort).ResetAll(keepAddrBook); err != nil {
			logger.Error().Msg(fmt.Sprintf("failed to reset tendermint application: %s", err))
			os.Exit(1)
		}

		logger.Info().Msg("successfully reset tendermint application")
	},
}
