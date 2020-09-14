package stakinggenesis

import (
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"math"
	"os"
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
	csvLabel                    string
	outboundDelegations         map[string]*quantity.Quantity
	testOnlyOutboundDelegations map[string]*quantity.Quantity
}

func (g *GenesisAccount) UnmarshalYAML(unmarshal func(interface{}) error) error {
	raw := struct {
		Amount                      string            `yaml:"amount"`
		Address                     string            `yaml:"address"`
		CsvLabel                    string            `yaml:"csv_label"`
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
	g.csvLabel = raw.CsvLabel

	g.testOnlyOutboundDelegations = make(map[string]*quantity.Quantity)

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

type GenesisEntityAllocations map[string]*Allocation

func (g *GenesisEntityAllocations) UnmarshalYAML(unmarshal func(interface{}) error) error {
	raw := make(map[string]*Allocation)

	err := unmarshal(&raw)
	if err != nil {
		return err
	}

	allocations := make(map[string]*Allocation)

	// Normalize entity names
	for entityName, allocation := range raw {
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
	TestOnlyEntities   GenesisEntityAllocations `yaml:"test_only_entities"`
	CommissionRateMax  uint64                   `yaml:"commission_rate_max"`
	CommissionRateMin  uint64                   `yaml:"commission_rate_min"`
	CommissionRate     uint64                   `yaml:"commission_rate"`
	CSVOptions         GenesisCSVOptions        `yaml:"csv_options"`
}

type GenesisCSVOptions struct {
	KycLabel                    string `yaml:"kyc_label"`
	EntityPackageSubmittedLabel string `yaml:"entity_package_submitted_label"`
	EntityPackageNameLabel      string `yaml:"entity_package_name_label"`
	FundingLabel                string `yaml:"funding_label"`
}

type Allocation struct {
	Delegations map[string]uint64 `yaml:"delegations"`
	Funds       uint64            `yaml:"funds"`
}

type EntityAllocationTable interface {
	All() GenesisEntityAllocations
}

type genesisCSV struct {
	options                     GenesisCSVOptions
	accounts                    GenesisAccounts
	kycIndex                    int
	entityPackageSubmittedIndex int
	entityPackageNameIndex      int
	fundingIndex                int
	accountIndices              map[string]int
	records                     [][]string
	allocations                 GenesisEntityAllocations
}

func loadGenesisCSV(path string, options GenesisCSVOptions, accounts GenesisAccounts) (*genesisCSV, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = 0

	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	g := &genesisCSV{
		options:        options,
		accounts:       accounts,
		records:        records,
		accountIndices: make(map[string]int),
		allocations:    make(map[string]*Allocation),
	}

	g.mapIndices()
	g.process()

	return g, nil
}

func (g *genesisCSV) mapIndices() error {
	found := 0
	accountLookup := make(map[string]string)
	// Create a lookup for account labels
	for name, account := range g.accounts {
		accountLookup[account.csvLabel] = name
	}
	// Iterate through the top row and determine the indexes
	for index, label := range g.records[0] {
		switch label {
		case g.options.KycLabel:
			g.kycIndex = index
			found++
		case g.options.EntityPackageSubmittedLabel:
			g.entityPackageSubmittedIndex = index
			found++
		case g.options.EntityPackageNameLabel:
			g.entityPackageNameIndex = index
			found++
		case g.options.FundingLabel:
			g.fundingIndex = index
			found++
		default:
			// Check if it's one of the accounts
			if accountName, ok := accountLookup[label]; ok {
				g.accountIndices[accountName] = index
			}
		}
	}
	return nil
}

func (g *genesisCSV) process() error {
	allocations := g.allocations

	for row, record := range g.records[1:] {
		// Skip if no entity package has been submitted
		if record[g.entityPackageSubmittedIndex] != "TRUE" {
			continue
		}

		entityName := strings.ToLower(record[g.entityPackageNameIndex])
		// if the entity name is blank we need to skip this
		if entityName == "" {
			logger.Warn("skipping row due to blank entity name", "row", row)
			continue
		}

		var funding uint64

		// Non-KYC cannot receive funds
		if record[g.kycIndex] == "TRUE" {
			value, err := parseHumanReadableNumberToUint64(record[g.fundingIndex])
			if err != nil {
				return err
			}
			funding = value
		}

		// Build delegations
		delegations := make(map[string]uint64)
		for accountName, accountIndex := range g.accountIndices {
			value, err := parseHumanReadableNumberToUint64(record[accountIndex])
			if err != nil {
				return err
			}
			delegations[accountName] = value
		}

		allocations[entityName] = &Allocation{
			Delegations: delegations,
			Funds:       funding,
		}
	}
	return nil
}

// Handle numbers with commas
func parseHumanReadableNumberToUint64(s string) (uint64, error) {
	noCommas := strings.ReplaceAll(s, ",", "")
	return strconv.ParseUint(noCommas, 10, 64)
}

func (g *genesisCSV) All() GenesisEntityAllocations {
	return g.allocations
}

// genesisCreator an implementation of a basic genesis allocations
// interface
type genesisCreator struct {
	options               GenesisOptions
	config                GenesisConfig
	entityMappings        map[string]staking.Address
	entityAllocationTable EntityAllocationTable
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

	// Load the allocations table from a CSV
	allocations, err := loadGenesisCSV(options.AllocationsPath, config.CSVOptions, config.Accounts)
	if err != nil {
		return nil, err
	}

	creator := genesisCreator{
		config:                config,
		options:               options,
		entityMappings:        make(map[string]staking.Address),
		entityAllocationTable: allocations,
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

		funds := quantity.NewFromUint64(allocation.Funds)

		// initialize account
		err := genesis.AddAccount(entityAddress, funds)
		if err != nil {
			return err
		}

		// Ensure that we don't self stake if we have less than the minimum
		// balance. Skip this entity
		if funds.Cmp(quantity.NewFromUint64(g.config.MinimumBalance)) > 0 {
			// Clone because of potentially odd mutability bugs
			escrowBalance := funds.Clone()

			// subtract minimum_balance on the all
			escrowBalance.Sub(quantity.NewFromUint64(g.config.MinimumBalance))

			// Stake to self
			err = genesis.AddDelegation(entityAddress, entityAddress, escrowBalance)
			if err != nil {
				return err
			}
		}

		err = g.setupEntityDelegations(genesis, entityAddress, allocation.Delegations)
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *genesisCreator) setupEntityDelegations(genesis *AccountingGenesis, delegateAddress staking.Address, delegations map[string]uint64) error {
	for accountName, amount := range delegations {
		account, ok := g.config.Accounts[accountName]
		if !ok {
			return fmt.Errorf("received unexpected account name %s", accountName)
		}
		if amount == 0 {
			continue
		}
		err := genesis.AddDelegation(account.address, delegateAddress, quantity.NewFromUint64(amount))
		if err != nil {
			return err
		}
	}
	return nil
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
		logger.Info(`adding entity name and address mapping`,
			"entity_name", name, "address", addressTxt)
		g.addEntityMapping(name, address)
	}

	err = g.setupAccountsForEntities(genesis, g.entityAllocationTable.All())
	if err != nil {
		return nil, err
	}

	if g.options.IsTestGenesis {
		err = g.setupAccountsForEntities(genesis, g.config.TestOnlyEntities)
		if err != nil {
			return nil, err
		}
	}

	//g.processAccountDelegations(genesis)

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
