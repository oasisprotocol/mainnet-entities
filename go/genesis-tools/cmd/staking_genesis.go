package cmd

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/oasisprotocol/amber-network-entities/go/genesis-tools/stakinggenesis"
	nodeCmdCommon "github.com/oasisprotocol/oasis-core/go/oasis-node/cmd/common"
)

const (
	cfgFaucetAddress           = "staking.faucet.address"
	cfgFaucetAmount            = "staking.faucet.amount"
	cfgTotalSupply             = "staking.total_supply"
	cfgPrecisionConstant       = "staking.precision_constant"
	cfgEntitiesDirectoryPaths  = "staking.entities_dir"
	cfgStakingParametersPath   = "staking.params"
	cfgDefaultFundingAmount    = "staking.default_funding"
	cfgDefaultSelfEscrowAmount = "staking.default_self_escrow"
	cfgOutputPath              = "output-path"
	defaultPrecisionConstant   = 1_000_000_000
	defaultTotalSupply         = 10_000_000_000
	defaultSelfEscrowAmount    = 100
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
	options := stakinggenesis.GenesisOptions{
		FaucetBase64Address:     viper.GetString(cfgFaucetAddress),
		FaucetAmount:            viper.GetInt64(cfgFaucetAmount),
		TotalSupply:             viper.GetInt64(cfgTotalSupply),
		PrecisionConstant:       viper.GetInt64(cfgPrecisionConstant),
		EntitiesDirectoryPaths:  viper.GetStringSlice(cfgEntitiesDirectoryPaths),
		ConsensusParametersPath: viper.GetString(cfgStakingParametersPath),
		DefaultFundingAmount:    viper.GetInt64(cfgDefaultFundingAmount),
		DefaultSelfEscrowAmount: viper.GetInt64(cfgDefaultSelfEscrowAmount),
	}

	if err := nodeCmdCommon.Init(); err != nil {
		nodeCmdCommon.EarlyLogAndExit(err)
	}

	outputPath := viper.GetString(cfgOutputPath)
	if outputPath == "" {
		logger.Error("must set output path for staking genesis file")
		os.Exit(1)
	}

	if len(options.EntitiesDirectoryPaths) < 1 {
		logger.Error("must define an entities directory path")
		os.Exit(1)
	}
	entitiesDir, err := stakinggenesis.LoadEntitiesDirectory(options.EntitiesDirectoryPaths)
	if err != nil {
		logger.Error("Cannot load entities",
			"err", err,
		)
		os.Exit(1)
	}
	options.Entities = entitiesDir

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

// RegisterForTestingCmd registers the for-testing subcommand.
func RegisterStakingGenesisCmd(parentCmd *cobra.Command) {
	stakingGenesisFlags.Int64(cfgFaucetAmount, 0, "amount to fund (in whole tokens)")
	stakingGenesisFlags.String(cfgFaucetAddress, "", "faucet address (base64 encoded)")
	stakingGenesisFlags.Int64(cfgTotalSupply, defaultTotalSupply, "Total supply of tokens (in whole tokens)")
	stakingGenesisFlags.Int64(cfgPrecisionConstant, defaultPrecisionConstant,
		"the precision constant for a single token defaults to 10^18")
	stakingGenesisFlags.StringSlice(cfgEntitiesDirectoryPaths, []string{}, "a directory entities")
	stakingGenesisFlags.String(cfgStakingParametersPath, "",
		"a consensus params json file (defaults to using ./consensus_params.json relative to entities directory)")
	stakingGenesisFlags.Int64(cfgDefaultFundingAmount, 0, "Default funding amount")
	stakingGenesisFlags.Int64(cfgDefaultSelfEscrowAmount, defaultSelfEscrowAmount, "Default amount to self escrow")
	stakingGenesisFlags.String(cfgOutputPath, "", "output path for the staking ledger")
	_ = viper.BindPFlags(stakingGenesisFlags)

	stakingGenesisCmd.Flags().AddFlagSet(stakingGenesisFlags)

	parentCmd.AddCommand(stakingGenesisCmd)
}
