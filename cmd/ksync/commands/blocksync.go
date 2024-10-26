package commands

import (
	"fmt"
	"github.com/KYVENetwork/ksync/backup"
	"github.com/KYVENetwork/ksync/blocksync"
	"github.com/KYVENetwork/ksync/engines"
	"github.com/KYVENetwork/ksync/sources"
	"github.com/KYVENetwork/ksync/utils"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

func init() {
	blockSyncCmd.Flags().StringVarP(&engine, "engine", "e", "", fmt.Sprintf("consensus engine of the binary by default %s is used, list all engines with \"ksync engines\"", utils.DefaultEngine))

	blockSyncCmd.Flags().StringVarP(&binaryPath, "binary", "b", "", "binary path of node to be synced, if not provided the binary has to be started externally with --with-tendermint=false")

	blockSyncCmd.Flags().StringVarP(&homePath, "home", "h", "", "home directory")

	blockSyncCmd.Flags().StringVarP(&chainId, "chain-id", "c", utils.DefaultChainId, fmt.Sprintf("KYVE chain id [\"%s\",\"%s\",\"%s\"]", utils.ChainIdMainnet, utils.ChainIdKaon, utils.ChainIdKorellia))

	blockSyncCmd.Flags().StringVar(&chainRest, "chain-rest", "", "rest endpoint for KYVE chain")
	blockSyncCmd.Flags().StringVar(&storageRest, "storage-rest", "", "storage endpoint for requesting bundle data")

	blockSyncCmd.Flags().StringVarP(&source, "source", "s", "", "chain-id of the source")
	blockSyncCmd.Flags().StringVar(&registryUrl, "registry-url", utils.DefaultRegistryURL, "URL to fetch latest KYVE Source-Registry")

	blockSyncCmd.Flags().StringVar(&blockPoolId, "block-pool-id", "", "pool-id of the block-sync pool")

	blockSyncCmd.Flags().Int64VarP(&targetHeight, "target-height", "t", 0, "target height (including)")

	blockSyncCmd.Flags().BoolVar(&rpcServer, "rpc-server", false, "rpc server serving /status, /block and /block_results")
	blockSyncCmd.Flags().Int64Var(&rpcServerPort, "rpc-server-port", utils.DefaultRpcServerPort, fmt.Sprintf("port for rpc server"))

	blockSyncCmd.Flags().Int64Var(&backupInterval, "backup-interval", 0, "block interval to write backups of data directory")
	blockSyncCmd.Flags().Int64Var(&backupKeepRecent, "backup-keep-recent", 3, "number of latest backups to be keep (0 to keep all backups)")
	blockSyncCmd.Flags().StringVar(&backupCompression, "backup-compression", "", "compression type used for backups (\"tar.gz\",\"zip\")")
	blockSyncCmd.Flags().StringVar(&backupDest, "backup-dest", "", fmt.Sprintf("path where backups should be stored (default = %s)", utils.DefaultBackupPath))

	blockSyncCmd.Flags().StringVarP(&appFlags, "app-flags", "f", "", "custom flags which are applied to the app binary start command. Example: --app-flags=\"--x-crisis-skip-assert-invariants,--iavl-disable-fastnode\"")

	blockSyncCmd.Flags().BoolVarP(&reset, "reset-all", "r", false, "reset this node's validator to genesis state")
	blockSyncCmd.Flags().BoolVar(&optOut, "opt-out", false, "disable the collection of anonymous usage data")
	blockSyncCmd.Flags().BoolVarP(&debug, "debug", "d", false, "show logs from tendermint app")
	blockSyncCmd.Flags().BoolVarP(&y, "yes", "y", false, "automatically answer yes for all questions")

	rootCmd.AddCommand(blockSyncCmd)
}

var blockSyncCmd = &cobra.Command{
	Use:   "block-sync",
	Short: "Start fast syncing blocks with KSYNC",
	Run: func(cmd *cobra.Command, args []string) {
		chainRest = utils.GetChainRest(chainId, chainRest)
		storageRest = strings.TrimSuffix(storageRest, "/")

		// if no binary was provided at least the home path needs to be defined
		if binaryPath == "" && homePath == "" {
			logger.Error().Msg(fmt.Sprintf("flag 'home' is required"))
			os.Exit(1)
		}

		if binaryPath == "" {
			logger.Info().Msg("To start the syncing process, start your chain binary with --with-tendermint=false")
		}

		if homePath == "" {
			homePath = utils.GetHomePathFromBinary(binaryPath)
			logger.Info().Msgf("Loaded home path \"%s\" from binary path", homePath)
		}

		defaultEngine := engines.EngineFactory(engine, homePath, rpcServerPort)

		if source == "" {
			s, err := defaultEngine.GetChainId()
			if err != nil {
				logger.Error().Msgf("Failed to load chain-id from engine: %s", err.Error())
				os.Exit(1)
			}
			source = s
			logger.Info().Msgf("Loaded source \"%s\" from genesis file", source)
		}

		if engine == "" && binaryPath != "" {
			engine = utils.GetEnginePathFromBinary(binaryPath)
			logger.Info().Msgf("Loaded engine \"%s\" from binary path", engine)
		}

		bId, _, err := sources.GetPoolIds(chainId, source, blockPoolId, "", registryUrl, true, false)
		if err != nil {
			logger.Error().Msg(fmt.Sprintf("failed to load pool-ids: %s", err))
			os.Exit(1)
		}

		backupCfg, err := backup.GetBackupConfig(homePath, backupInterval, backupKeepRecent, backupCompression, backupDest)
		if err != nil {
			logger.Error().Str("err", err.Error()).Msg("could not get backup config")
			return
		}

		if reset {
			if err := defaultEngine.ResetAll(true); err != nil {
				logger.Error().Msg(fmt.Sprintf("failed to reset tendermint application: %s", err))
				os.Exit(1)
			}
		}

		if err := defaultEngine.OpenDBs(); err != nil {
			logger.Error().Msg(fmt.Sprintf("failed to open dbs in engine: %s", err))
			os.Exit(1)
		}

		// perform validation checks before booting state-sync process
		continuationHeight, err := blocksync.PerformBlockSyncValidationChecks(defaultEngine, chainRest, nil, &bId, targetHeight, true, !y)
		if err != nil {
			logger.Error().Msg(fmt.Sprintf("block-sync validation checks failed: %s", err))
			os.Exit(1)
		}

		if err := defaultEngine.CloseDBs(); err != nil {
			logger.Error().Msg(fmt.Sprintf("failed to close dbs in engine: %s", err))
			os.Exit(1)
		}

		sources.IsBinaryRecommendedVersion(binaryPath, registryUrl, source, continuationHeight, !y)

		consensusEngine := engines.EngineSourceFactory(engine, homePath, registryUrl, source, rpcServerPort, continuationHeight)

		blocksync.StartBlockSyncWithBinary(consensusEngine, binaryPath, homePath, chainId, chainRest, storageRest, nil, &bId, targetHeight, backupCfg, appFlags, rpcServer, optOut, debug)
	},
}
