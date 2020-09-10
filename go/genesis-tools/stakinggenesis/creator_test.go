package stakinggenesis_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"testing"

	"github.com/oasisprotocol/mainnet-entities/go/genesis-tools/stakinggenesis"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	fileSigner "github.com/oasisprotocol/oasis-core/go/common/crypto/signature/signers/file"
	"github.com/oasisprotocol/oasis-core/go/common/entity"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/stretchr/testify/require"
)

type fakeEntities struct {
	names    []string
	entities map[string]*entity.Entity
}

func MakeFakeEntities(names []string) *fakeEntities {
	e := fakeEntities{
		names:    names,
		entities: make(map[string]*entity.Entity),
	}
	e.generateAll()
	return &e
}

func (e *fakeEntities) generateAll() {
	for _, name := range e.names {
		ent, err := e.generateEntity()
		if err != nil {
			panic(err)
		}
		e.entities[name] = ent
	}
}

func (e *fakeEntities) generateEntity() (*entity.Entity, error) {
	dir, err := ioutil.TempDir("", "prefix")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	signerFactory, err := fileSigner.NewFactory(dir, signature.SignerEntity)

	if err != nil {
		return nil, err
	}

	ent, _, err := entity.Generate(dir, signerFactory, &entity.Entity{
		AllowEntitySignedNodes: false,
	})

	return ent, nil
}

func (e *fakeEntities) All() map[string]*entity.Entity {
	return e.entities
}

func (e *fakeEntities) ResolveEntity(name string) *entity.Entity {
	ent, ok := e.entities[name]
	if !ok {
		return nil
	}
	return ent
}

func genericGenesisOptions(entityNames []string) stakinggenesis.GenesisOptions {
	entities := MakeFakeEntities(entityNames)
	return stakinggenesis.GenesisOptions{
		Entities: entities,
		ConsensusParametersLoader: func() staking.ConsensusParameters {
			return staking.ConsensusParameters{}
		},
	}
}

type genesisTestValidator struct {
	entities stakinggenesis.Entities
	genesis  *staking.Genesis
	accounts map[string]staking.Address
}

func newValidator(genesis *staking.Genesis, entities stakinggenesis.Entities) genesisTestValidator {
	validator := genesisTestValidator{
		genesis:  genesis,
		entities: entities,
		accounts: make(map[string]staking.Address),
	}

	// Add account1/account2 test accounts
	validator.addAccount("account1", "oasis1qz2kz3zkgf6trclyajtyg4jecw7es7p5tutfqaz0")
	validator.addAccount("account2", "oasis1qz6hdmtth24x5udlvmavufwvy5ac6pvh2cdlehnx")
	return validator
}

func (g *genesisTestValidator) addAccount(name, addressStr string) {
	var address staking.Address
	err := address.UnmarshalText([]byte(addressStr))
	if err != nil {
		panic(err)
	}
	g.accounts[name] = address
}

func (g *genesisTestValidator) entityAddress(name string) staking.Address {
	// Check accounts first
	account, ok := g.accounts[name]
	entity := g.entities.ResolveEntity(name)

	if ok && entity != nil {
		panic(fmt.Errorf("test misconfigured: duplicate names in accounts and entities"))
	}

	if ok {
		return account
	}

	if entity == nil {
		panic(fmt.Errorf("entity address not found for %s", name))
	}
	return staking.NewAddress(entity.ID)
}

func (g *genesisTestValidator) requireCorrectTotals(t *testing.T, commonPool, totalSupply uint64) {
	requireQuantityEqual(t, g.genesis.TotalSupply, totalSupply)
	requireQuantityEqual(t, g.genesis.CommonPool, commonPool)
}

func (g *genesisTestValidator) requireEscrowBalance(t *testing.T, name string, expected uint64) {
	address := g.entityAddress(name)

	balance := g.genesis.Ledger[address].Escrow.Active.Balance
	shares := g.genesis.Ledger[address].Escrow.Active.TotalShares
	if expected == 0 {
		require.True(t, balance.IsZero())
		require.True(t, shares.IsZero())
		return
	}
	requireQuantityEqual(t, balance, expected)
	requireQuantityEqual(t, shares, expected)
}

func (g *genesisTestValidator) requireGeneralBalance(t *testing.T, name string, expected uint64) {
	address := g.entityAddress(name)

	balance := g.genesis.Ledger[address].General.Balance
	if expected == 0 {
		require.True(t, balance.IsZero())
		return
	}
	requireQuantityEqual(t, balance, expected)
}

func (g *genesisTestValidator) requireDelegationShares(t *testing.T, from string, to string, expected uint64) {
	fromAddress := g.entityAddress(from)
	toAddress := g.entityAddress(to)

	if expected == 0 {
		_, ok := g.genesis.Delegations[toAddress][fromAddress]
		require.False(t, ok, "should not have a set of delegation shares")
	} else {
		shares := g.genesis.Delegations[toAddress][fromAddress].Shares
		requireQuantityEqual(t, shares, expected)
	}
}

func TestMetaTestGenericGenesisOptions(t *testing.T) {
	options := genericGenesisOptions([]string{
		"test1",
		"test2",
	})

	names := make([]string, 0)

	for name, _ := range options.Entities.All() {
		names = append(names, name)
	}

	sort.Strings(names)

	require.Equal(t, names, []string{"test1", "test2"})
}

func TestGenerateStakingLedger(t *testing.T) {
	options := genericGenesisOptions([]string{
		"test1",
		"test2",
		"test3",
		"test4",
	})
	options.ConfigurationPath = "fixtures/staking_ledger_config.yaml"
	genesis, err := stakinggenesis.Create(options)
	require.NoError(t, err)

	validator := newValidator(genesis, options.Entities)

	validator.requireCorrectTotals(t,
		6_699_999_000_000_000_000,
		10_000_000_000_000_000_000,
	)

	validator.requireGeneralBalance(t, "test1", 100_000_000_000)
	validator.requireEscrowBalance(t, "test1", 199_999_900_000_000_000)
	validator.requireDelegationShares(t, "test1", "test1", 199_999_900_000_000_000)

	validator.requireGeneralBalance(t, "test2", 100_000_000_000)
	validator.requireEscrowBalance(t, "test2", 199_999_900_000_000_000)

	validator.requireGeneralBalance(t, "test3", 100_000_000_000)
	validator.requireEscrowBalance(t, "test3", 100_000_900_000_000_000)

	validator.requireGeneralBalance(t, "test4", 0)
	validator.requireEscrowBalance(t, "test4", 1_000_000_000_000)

	validator.requireDelegationShares(t, "account1", "test1", 0)
	validator.requireDelegationShares(t, "account1", "test2", 100_000_000_000_000_000)
	validator.requireDelegationShares(t, "account1", "test3", 0)
	validator.requireDelegationShares(t, "account1", "test4", 1_000_000_000_000)

	validator.requireDelegationShares(t, "account2", "test1", 0)
	validator.requireDelegationShares(t, "account2", "test2", 0)
	validator.requireDelegationShares(t, "account2", "test3", 100_000_000_000_000_000)
	validator.requireDelegationShares(t, "account2", "test4", 0)
}

func TestGenerateTestStakingLedger(t *testing.T) {
	options := genericGenesisOptions([]string{
		"test1",
		"test2",
		"test3",
		"test4",
		"test5",
	})
	options.ConfigurationPath = "fixtures/staking_ledger_config.yaml"
	options.IsTestGenesis = true
	genesis, err := stakinggenesis.Create(options)
	require.NoError(t, err)

	validator := newValidator(genesis, options.Entities)

	validator.requireCorrectTotals(t,
		6_399_999_000_000_000_000,
		10_000_000_000_000_000_000,
	)

	validator.requireGeneralBalance(t, "test1", 100_000_000_000)
	validator.requireEscrowBalance(t, "test1", 499_999_900_000_000_000)
	validator.requireDelegationShares(t, "test1", "test1", 199_999_900_000_000_000)

	validator.requireGeneralBalance(t, "test2", 100_000_000_000)
	validator.requireEscrowBalance(t, "test2", 199_999_900_000_000_000)

	validator.requireGeneralBalance(t, "test3", 100_000_000_000)
	validator.requireEscrowBalance(t, "test3", 300_000_900_000_000_000)

	validator.requireGeneralBalance(t, "test4", 0)
	validator.requireEscrowBalance(t, "test4", 1_000_000_000_000)

	validator.requireGeneralBalance(t, "test5", 100_000_000_000)
	validator.requireEscrowBalance(t, "test5", 299_999_900_000_000_000)

	validator.requireDelegationShares(t, "account1", "test1", 0)
	validator.requireDelegationShares(t, "account1", "test2", 100_000_000_000_000_000)
	validator.requireDelegationShares(t, "account1", "test3", 200_000_000_000_000_000)
	validator.requireDelegationShares(t, "account1", "test4", 1_000_000_000_000)

	validator.requireDelegationShares(t, "account2", "test1", 300_000_000_000_000_000)
	validator.requireDelegationShares(t, "account2", "test2", 0)
	validator.requireDelegationShares(t, "account2", "test3", 100_000_000_000_000_000)
	validator.requireDelegationShares(t, "account2", "test4", 0)
}

func TestLoadStakingParameters(t *testing.T) {
	// This is a bit brittle
	params, err := stakinggenesis.LoadStakingConsensusParameters("fixtures/staking_params.json")

	require.NoError(t, err)

	require.Equal(t, params.Thresholds[staking.KindEntity], *quantity.NewFromUint64(100_000_000_000))
}
