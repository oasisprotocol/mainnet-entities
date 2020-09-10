package stakinggenesis

import (
	"fmt"
	"io/ioutil"
	"math"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
)

// Accounts convenience wrapper around accounts
type StakingAccounts map[staking.Address]*staking.Account

type StakingDelegations map[staking.Address]map[staking.Address]*staking.Delegation

func parseUintStrToQuantity(s string) (*quantity.Quantity, error) {
	uintValue, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return nil, err
	}

	return quantity.NewFromUint64(uintValue), nil
}

type GenesisAccount struct {
	amount                      *quantity.Quantity
	address                     staking.Address
	outboundDelegations         map[string]*quantity.Quantity
	testOnlyOutboundDelegations map[string]*quantity.Quantity
}

func (g *GenesisAccount) UnmarshalYAML(unmarshal func(interface{}) error) error {
	raw := struct {
		Amount                      string            `yaml:"amount"`
		Address                     string            `yaml:"address"`
		OutboundDelegations         map[string]string `yaml:"outbound_delegations"`
		TestOnlyOutboundDelegations map[string]string `yaml:"test_only_outbound_delegations"`
	}{}

	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	g.amount, err = parseUintStrToQuantity(raw.Amount)
	if err != nil {
		return err
	}

	var address staking.Address
	err = address.UnmarshalText([]byte(raw.Address))
	if err != nil {
		return err
	}

	g.address = address

	g.outboundDelegations = make(map[string]*quantity.Quantity)
	g.testOnlyOutboundDelegations = make(map[string]*quantity.Quantity)

	for name, rawAmount := range raw.OutboundDelegations {
		amount, err := parseUintStrToQuantity(rawAmount)
		if err != nil {
			return err
		}
		g.outboundDelegations[name] = amount
	}
	for name, rawAmount := range raw.TestOnlyOutboundDelegations {
		amount, err := parseUintStrToQuantity(rawAmount)
		if err != nil {
			return err
		}
		g.testOnlyOutboundDelegations[name] = amount
	}
	return nil
}

type GenesisAccounts map[string]*GenesisAccount

func (g *GenesisAccounts) UnmarshalYAML(unmarshal func(interface{}) error) error {
	raw := make(map[string]*GenesisAccount)

	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	accounts := make(map[string]*GenesisAccount)

	// Convert each of the values into a quantity
	for name, account := range raw {
		// Normalize entity names
		accounts[strings.ToLower(name)] = account
	}

	*g = accounts
	return nil
}

type GenesisEntityAllocations map[string]*quantity.Quantity

func (g *GenesisEntityAllocations) UnmarshalYAML(unmarshal func(interface{}) error) error {
	raw := make(map[string]string)

	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	allocations := make(map[string]*quantity.Quantity)

	// Convert each of the values into a quantity
	for entityName, allocationString := range raw {
		allocation, err := parseUintStrToQuantity(allocationString)
		if err != nil {
			return err
		}
		// Normalize entity names
		allocations[strings.ToLower(entityName)] = allocation
	}

	*g = allocations
	return nil
}

type GenesisConfig struct {
	MinimumBalance     uint64                   `yaml:"minimum_balance"`
	TotalSupply        uint64                   `yaml:"total_supply"`
	TokenSymbol        string                   `yaml:"token_symbol"`
	TokenValueExponent uint8                    `yaml:"token_value_exponent"`
	Accounts           GenesisAccounts          `yaml:"accounts"`
	Entities           GenesisEntityAllocations `yaml:"entities"`
	TestOnlyEntities   GenesisEntityAllocations `yaml:"test_only_entities"`
	CommissionRateMax  uint64                   `yaml:"commission_rate_max"`
	CommissionRateMin  uint64                   `yaml:"commission_rate_min"`
	CommissionRate     uint64                   `yaml:"commission_rate"`
}

// genesisCreator an implementation of a basic genesis allocations
// interface
type genesisCreator struct {
	options        GenesisOptions
	config         GenesisConfig
	entityMappings map[string]staking.Address
}

// Create loads a genesis allocation from a yaml file
func Create(options GenesisOptions) (*staking.Genesis, error) {
	bytes, err := ioutil.ReadFile(options.ConfigurationPath)
	if err != nil {
		return nil, err
	}

	var config GenesisConfig
	err = yaml.Unmarshal(bytes, &config)
	if err != nil {
		return nil, err
	}

	var creator = genesisCreator{
		config:         config,
		options:        options,
		entityMappings: make(map[string]staking.Address),
	}

	return creator.GenerateGenesis()
}

func (g *genesisCreator) initializeAccountingGenesis() (*AccountingGenesis, error) {
	var precision uint64
	precision = uint64(math.Pow(10.0, float64(g.config.TokenValueExponent)))

	genesis := NewAccountingGenesis(
		precision,
		g.config.TotalSupply,
		g.config.CommissionRateMax,
		g.config.CommissionRateMin,
		g.config.CommissionRate,
	)

	// Loop through the main accounts defined in the document
	for _, account := range g.config.Accounts {
		err := genesis.AddAccount(account.address, account.amount)
		if err != nil {
			return nil, err
		}
	}
	return genesis, nil
}

// addEntityMapping Adds an entity name to address mapping
func (g *genesisCreator) addEntityMapping(name string, address staking.Address) error {
	if _, ok := g.entityMappings[name]; ok {
		return fmt.Errorf("duplicate definitions of entity's account named %s", name)
	}
	g.entityMappings[name] = address
	return nil
}

func (g *genesisCreator) setupAccountsForEntities(genesis *AccountingGenesis, entities GenesisEntityAllocations) error {
	// Setup entity accounts and establish self delegation
	for name, allocation := range entities {
		entityAddress, ok := g.entityMappings[name]
		if !ok {
			return fmt.Errorf(`account name "%s" is missing from processed entity packages`, name)
		}

		// initialize account
		err := genesis.AddAccount(entityAddress, allocation)
		if err != nil {
			return err
		}

		// Ensure that we don't self stake if we have less than the minimum
		// balance. Skip this entity
		if allocation.Cmp(quantity.NewFromUint64(g.config.MinimumBalance)) <= 0 {
			continue
		}

		// Clone because of potentially odd mutability bugs
		escrowBalance := allocation.Clone()

		// subtract minimum_balance on the all
		escrowBalance.Sub(quantity.NewFromUint64(g.config.MinimumBalance))

		// Stake to self
		err = genesis.AddDelegation(entityAddress, entityAddress, escrowBalance)
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *genesisCreator) processAccountDelegations(genesis *AccountingGenesis) {
	for _, account := range g.config.Accounts {
		for dest, amount := range account.outboundDelegations {
			genesis.AddDelegation(account.address, g.entityMappings[dest], amount)
		}

		if g.options.IsTestGenesis {
			for dest, amount := range account.testOnlyOutboundDelegations {
				genesis.AddDelegation(account.address, g.entityMappings[dest], amount)
			}
		}
	}
}

// Ledger returns the created ledger
func (g *genesisCreator) generateAccountingGenesis() (*staking.Genesis, error) {
	// Start by adding the defined accounts in the genesis allocations document
	genesis, err := g.initializeAccountingGenesis()
	if err != nil {
		return nil, err
	}

	// Setup entity mappings
	for name, entity := range g.options.Entities.All() {
		address := staking.NewAddress(entity.ID)
		addressTxt, err := address.MarshalText()
		if err != nil {
			return nil, err
		}
		logger.Info(`mapping %s to %s`, name, addressTxt)
		g.addEntityMapping(name, address)
	}

	fmt.Printf("%+v\n", g.config.Entities)
	err = g.setupAccountsForEntities(genesis, g.config.Entities)
	if err != nil {
		return nil, err
	}

	if g.options.IsTestGenesis {
		fmt.Printf("%+v\n", g.config.TestOnlyEntities)
		err = g.setupAccountsForEntities(genesis, g.config.TestOnlyEntities)
		if err != nil {
			return nil, err
		}
	}

	g.processAccountDelegations(genesis)

	partial := genesis.GetPartialGenesis()
	return &partial, nil
}

func (g *genesisCreator) GenerateGenesis() (*staking.Genesis, error) {
	// Load consensus params
	params, err := g.options.LoadConsensusParameters()
	if err != nil {
		return nil, err
	}

	// Load accounting
	genesis, err := g.generateAccountingGenesis()
	if err != nil {
		return nil, err
	}

	genesis.Parameters = *params
	genesis.TokenSymbol = g.config.TokenSymbol
	genesis.TokenValueExponent = g.config.TokenValueExponent
	return genesis, nil
}
