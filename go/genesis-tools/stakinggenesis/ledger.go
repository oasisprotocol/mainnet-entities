package stakinggenesis

import (
	"encoding/json"
	"io/ioutil"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
)

// GenesisOptions options for the staking genesis document.
type GenesisOptions struct {
	AdditionalEntitiesToFund  map[string]int64
	TotalSupply               int64
	PrecisionConstant         int64
	EntitiesDirectoryPaths    []string
	GenesisAllocations        GenesisAllocations
	MinimumStake              int64
	ConsensusParametersPath   string
	ConsensusParametersLoader func() staking.ConsensusParameters
	Entities                  Entities
}

type genesisCreator struct {
	genesis           staking.Genesis
	options           GenesisOptions
	precisionConstant *quantity.Quantity
}

// Create creates a staking ledger file to be used in a genesis
// document. This handles proper accounting of token amounts.
func Create(options GenesisOptions) (*staking.Genesis, error) {
	creator := genesisCreator{options: options}
	return creator.create()
}

func (g *genesisCreator) create() (*staking.Genesis, error) {
	g.genesis.Ledger = make(map[staking.Address]*staking.Account)
	g.genesis.Delegations = make(map[staking.Address]map[staking.Address]*staking.Delegation)

	g.precisionConstant = quantity.NewQuantity()
	_ = g.precisionConstant.FromInt64(g.options.PrecisionConstant)

	logger.Debug("Setup total supply")

	// Setup total supply
	totalSupply := g.toStakingQuantity(g.options.TotalSupply)
	g.genesis.TotalSupply = *totalSupply

	logger.Debug("Loading Consensus Params")
	// Setup Consensus Parameters
	err := g.loadConsensusParameters()
	if err != nil {
		return nil, err
	}

	logger.Debug("Setting up the faucet")
	// Setup Faucet
	err = g.setupAdditionalEntitiesToFund()
	if err != nil {
		return nil, err
	}

	logger.Debug("Loading all entitiy")
	// TODO Add a way to load a custom ledger amount
	// Load all entities and fund them
	for _, info := range g.options.Entities.All() {
		g.setupEntity(info)
	}

	logger.Debug("Calculate the common pool amount")
	err = g.calculateCommonPool()
	if err != nil {
		return nil, err
	}

	return &g.genesis, nil
}

// ToStakingQuantity translates a human specified whole token amount
// to the proper amount for the staking ledger.
func (g *genesisCreator) toStakingQuantity(v int64) *quantity.Quantity {
	q := g.toQuantity(v)
	err := q.Mul(g.precisionConstant)
	if err != nil {
		panic(err)
	}
	return q
}

func (g *genesisCreator) toQuantity(v int64) *quantity.Quantity {
	q := quantity.NewQuantity()
	err := q.FromInt64(v)
	if err != nil {
		panic(err)
	}
	return q
}

func (g *genesisCreator) setupEntity(info *EntityInfo) {
	entityAddr := staking.NewAddress(info.descriptor.ID)

	if info.ledgerAllocation.Cmp(g.toStakingQuantity(g.options.MinimumStake)) < 0 {
		// Skip this entity
		return
	}

	// Keep a single token in the general balance (unstaked)
	// Stake the remaining balance
	unstaked := g.toStakingQuantity(0)
	staked := info.ledgerAllocation.Clone()

	// subtract the token from the staked balance
	staked.Sub(unstaked)

	g.setLedgerForEntity(entityAddr, unstaked, staked)
	g.setDelegation(entityAddr, entityAddr, staked)
}

func (g *genesisCreator) setLedgerForEntity(entityAddr staking.Address, tokenBalance *quantity.Quantity, tokensInEscrow *quantity.Quantity) {
	g.genesis.Ledger[entityAddr] = &staking.Account{
		General: staking.GeneralAccount{
			Balance: *tokenBalance,
			Nonce:   0,
		},
		Escrow: staking.EscrowAccount{
			Active: staking.SharePool{
				Balance:     *tokensInEscrow,
				TotalShares: *tokensInEscrow,
			},
			Debonding: staking.SharePool{
				Balance:     *g.toStakingQuantity(0),
				TotalShares: *g.toStakingQuantity(0),
			},
		},
	}
}

func (g *genesisCreator) setDelegation(fromEntityAddr staking.Address, toEntityAddr staking.Address, tokensToEscrow *quantity.Quantity) {
	delegations, ok := g.genesis.Delegations[toEntityAddr]
	if !ok {
		delegations = make(map[staking.Address]*staking.Delegation)
	}
	delegations[fromEntityAddr] = &staking.Delegation{
		Shares: *tokensToEscrow,
	}
	g.genesis.Delegations[toEntityAddr] = delegations
}

func (g *genesisCreator) setupAdditionalEntitiesToFund() error {
	for pubKeyString, amount := range g.options.AdditionalEntitiesToFund {
		var pubKey signature.PublicKey
		err := pubKey.UnmarshalText([]byte(pubKeyString))
		if err != nil {
			logger.Error("error loading public key",
				"err", err,
				"pubKey", pubKeyString,
			)
			return err
		}
		stakingAddr := staking.NewAddress(pubKey)
		logger.Info(
			"funding key on ledger",
			"pubKey", pubKeyString,
			"addr", stakingAddr.String(),
		)
		g.setLedgerForEntity(stakingAddr, g.toStakingQuantity(amount), g.toStakingQuantity(0))
	}
	return nil
}

func (g *genesisCreator) calculateCommonPool() error {
	var entityTotalBalances map[staking.Address]*quantity.Quantity
	var err error

	allocatedTokens := quantity.NewQuantity()

	// Iterate through all of the accounts on the ledger
	for entityAddress, account := range g.genesis.Ledger {
		q, ok := entityTotalBalances[entityAddress]
		if !ok {
			q = quantity.NewQuantity()
		}
		err = q.Add(&account.General.Balance)
		if err != nil {
			return err
		}
		err = allocatedTokens.Add(&account.General.Balance)
		if err != nil {
			return err
		}
		err = allocatedTokens.Add(&account.Escrow.Active.Balance)
		if err != nil {
			return err
		}
		err = allocatedTokens.Add(&account.Escrow.Debonding.Balance)
		if err != nil {
			return err
		}
	}

	commonPool := g.genesis.TotalSupply.Clone()

	err = commonPool.Sub(allocatedTokens)
	if err != nil {
		return err
	}

	g.genesis.CommonPool = *commonPool
	return nil
}

func (g *genesisCreator) resolveEntityPublicKey(name string) (*signature.PublicKey, error) {
	info, err := g.options.Entities.ResolveEntity(name)
	if err != nil {
		return nil, err
	}
	return &info.descriptor.ID, nil
}

func (g *genesisCreator) loadConsensusParameters() error {
	if g.options.ConsensusParametersLoader != nil {
		params := g.options.ConsensusParametersLoader()
		g.genesis.Parameters = params
		return nil
	}
	params, err := LoadStakingConsensusParameters(g.options.ConsensusParametersPath)
	if err != nil {
		return err
	}
	g.genesis.Parameters = *params
	return nil
}

// LoadStakingConsensusParameters - Load Staking Consensus Params from a file
func LoadStakingConsensusParameters(path string) (*staking.ConsensusParameters, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var params staking.ConsensusParameters
	err = json.Unmarshal(b, &params)
	if err != nil {
		return nil, err
	}
	return &params, nil
}
