package commands

import (
	"fmt"
	"github.com/KYVENetwork/ksync/app"
	"github.com/KYVENetwork/ksync/flags"
	"github.com/KYVENetwork/ksync/logger"
	"github.com/spf13/cobra"
)

func init() {
	resetCmd.Flags().StringVar(&flags.HomePath, "home", "", "home directory")
	if err := resetCmd.MarkFlagRequired("home"); err != nil {
		panic(fmt.Errorf("flag 'home' should be required: %w", err))
	}

	resetCmd.Flags().BoolVar(&flags.KeepAddrBook, "keep-addr-book", true, "keep the address book intact")

	resetCmd.Flags().BoolVar(&flags.OptOut, "opt-out", false, "disable the collection of anonymous usage data")
	resetCmd.Flags().BoolVarP(&flags.Debug, "debug", "d", false, "run KSYNC in debug mode")

	RootCmd.AddCommand(resetCmd)
}

var resetCmd = &cobra.Command{
	Use:   "reset-all",
	Short: "Removes all the data and WAL, reset this node's validator to genesis state",
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := app.NewCosmosApp()
		if err != nil {
			return fmt.Errorf("failed to init cosmos app: %w", err)
		}

		if err := app.ConsensusEngine.ResetAll(true); err != nil {
			return fmt.Errorf("failed to reset cosmos app: %w", err)
		}

		logger.Logger.Info().Msg("successfully reset cosmos app")
		return nil
	},
}
