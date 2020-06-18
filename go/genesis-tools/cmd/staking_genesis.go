package cmd

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/oasisprotocol/amber-network-entities/go/genesis-tools/stakinggenesis"
	nodeCmdCommon "github.com/oasisprotocol/oasis-core/go/oasis-node/cmd/common"
)

const (
	cfgTotalSupply            = "staking.total_supply"
	cfgPrecisionConstant      = "staking.precision_constant"
	cfgEntitiesDirectoryPaths = "staking.entities_dir"
	cfgStakingParametersPath  = "staking.params"
	cfgEntitiesToFund         = "staking.entities_to_fund"
	cfgGenesisAllocationsPath = "staking.allocations"
	cfgOutputPath             = "output-path"
	defaultPrecisionConstant  = 1_000_000_000
	defaultTotalSupply        = 10_000_000_000
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
	precisionConstant := viper.GetInt64(cfgPrecisionConstant)

	entitiesToFundUnParsed := viper.GetStringSlice(cfgEntitiesToFund)
	entitiesToFund := make(map[string]int64)
	for _, value := range entitiesToFundUnParsed {
		split := strings.SplitN(value, ":", 2)
		fundAmount, err := strconv.ParseInt(split[1], 10, 64)
		if err != nil {
			panic("entities to fund need to have numbers")
		}
		entitiesToFund[split[0]] = fundAmount
	}

	options := stakinggenesis.GenesisOptions{
		TotalSupply:              viper.GetInt64(cfgTotalSupply),
		PrecisionConstant:        precisionConstant,
		AdditionalEntitiesToFund: entitiesToFund,
		MinimumStake:             1,
		EntitiesDirectoryPaths:   viper.GetStringSlice(cfgEntitiesDirectoryPaths),
		ConsensusParametersPath:  viper.GetString(cfgStakingParametersPath),
	}

	if err := nodeCmdCommon.Init(); err != nil {
		nodeCmdCommon.EarlyLogAndExit(err)
	}

	outputPath := viper.GetString(cfgOutputPath)
	if outputPath == "" {
		logger.Error("must set output path for staking genesis file")
		os.Exit(1)
	}

	genesisAllocationsPath := viper.GetString(cfgGenesisAllocationsPath)
	allocations, err := stakinggenesis.NewGenesisAllocationsFromFile(genesisAllocationsPath, uint64(precisionConstant))
	if err != nil {
		logger.Error("error loading genesis allocations")
		os.Exit(1)
	}

	if len(options.EntitiesDirectoryPaths) < 1 {
		logger.Error("must define an entities directory path")
		os.Exit(1)
	}

	entitiesDir, err := stakinggenesis.LoadEntitiesDirectory(allocations, options.EntitiesDirectoryPaths)
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
	stakingGenesisFlags.Int64(cfgTotalSupply, defaultTotalSupply, "Total supply of tokens (in whole tokens)")
	stakingGenesisFlags.Int64(cfgPrecisionConstant, defaultPrecisionConstant,
		"the precision constant for a single token defaults to 10^18")
	stakingGenesisFlags.StringSlice(cfgEntitiesDirectoryPaths, []string{}, "a directory entities")
	stakingGenesisFlags.String(cfgStakingParametersPath, "",
		"a consensus params json file (defaults to using ./consensus_params.json relative to entities directory)")
	stakingGenesisFlags.String(cfgGenesisAllocationsPath, "",
		"a yaml file used to setup the funding allocations per account")
	stakingGenesisFlags.String(cfgOutputPath, "", "output path for the staking ledger")
	stakingGenesisFlags.StringSlice(cfgEntitiesToFund, []string{}, "public key to funding amount")
	_ = viper.BindPFlags(stakingGenesisFlags)

	stakingGenesisCmd.Flags().AddFlagSet(stakingGenesisFlags)

	parentCmd.AddCommand(stakingGenesisCmd)
}
