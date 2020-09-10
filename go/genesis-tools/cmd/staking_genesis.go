package cmd

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/oasisprotocol/mainnet-entities/go/genesis-tools/stakinggenesis"
	nodeCmdCommon "github.com/oasisprotocol/oasis-core/go/oasis-node/cmd/common"
)

const (
	cfgEntitiesDirPaths      = "staking.entities_dir"
	cfgStakingParametersPath = "staking.params"
	cfgGenesisConfigPath     = "staking.config"
	cfgTestOnlyGenesis       = "staking.test_only_genesis"
	cfgOutputPath            = "output-path"
)

var (
	stakingGenesisCmd = &cobra.Command{
		Use:   "staking_genesis",
		Short: "Generates a staking ledger for genesis",
		Long: `Generates a staking ledger for genesis

        Uses a directory of unpacked Entity Packages.
        Amounts are configured in whole tokens`,
		Run: doStakingGenesis,
	}

	stakingGenesisFlags = flag.NewFlagSet("", flag.ContinueOnError)
)

func doStakingGenesis(cmd *cobra.Command, args []string) {
	if err := nodeCmdCommon.Init(); err != nil {
		nodeCmdCommon.EarlyLogAndExit(err)
	}

	entitiesDirPaths := viper.GetStringSlice(cfgEntitiesDirPaths)
	if len(entitiesDirPaths) < 1 {
		logger.Error("must define an entities directory path")
		os.Exit(1)
	}
	entitiesDir, err := stakinggenesis.LoadEntitiesDirectory(entitiesDirPaths)
	if err != nil {
		logger.Error("Cannot load entities",
			"err", err,
		)
		os.Exit(1)
	}

	options := stakinggenesis.GenesisOptions{
		Entities:                entitiesDir,
		ConsensusParametersPath: viper.GetString(cfgStakingParametersPath),
		ConfigurationPath:       viper.GetString(cfgGenesisConfigPath),
		IsTestGenesis:           viper.GetBool(cfgTestOnlyGenesis),
	}

	outputPath := viper.GetString(cfgOutputPath)
	if outputPath == "" {
		logger.Error("must set output path for staking genesis file")
		os.Exit(1)
	}

	stakingGenesis, err := stakinggenesis.Create(options)
	if err != nil {
		logger.Error("failed to create a staking genesis file",
			"err", err,
		)
		os.Exit(1)
	}

	b, err := json.Marshal(stakingGenesis)
	err = ioutil.WriteFile(outputPath, b, 0644)
	if err != nil {
		logger.Error("failed to write staking genesis to json",
			"err", err,
		)
		os.Exit(1)
	}
}

// RegisterStakingGenesisCmd registers the for-testing subcommand.
func RegisterStakingGenesisCmd(parentCmd *cobra.Command) {
	stakingGenesisFlags.StringSlice(cfgEntitiesDirPaths, []string{}, "a directory entities")
	stakingGenesisFlags.String(cfgStakingParametersPath, "",
		"a consensus params json file (defaults to using ./consensus_params.json relative to entities directory)")
	stakingGenesisFlags.String(cfgGenesisConfigPath, "",
		"a yaml file used to establish fund and delegation allocation on the staking ledger")
	stakingGenesisFlags.String(cfgOutputPath, "", "output path for the staking ledger")
	stakingGenesisFlags.Bool(cfgTestOnlyGenesis, false, "generate a test staking ledger")
	_ = viper.BindPFlags(stakingGenesisFlags)

	stakingGenesisCmd.Flags().AddFlagSet(stakingGenesisFlags)

	parentCmd.AddCommand(stakingGenesisCmd)
}
