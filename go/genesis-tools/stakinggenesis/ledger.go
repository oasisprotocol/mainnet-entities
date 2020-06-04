package stakinggenesis

import (
	"encoding/json"
	"io/ioutil"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-core/go/common/entity"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
)

// GenesisOptions options for the staking genesis document.
type GenesisOptions struct {
	FaucetBase64Address       string
	FaucetAmount              int64
	TotalSupply               int64
	PrecisionConstant         int64
	EntitiesDirectoryPaths    []string
	ConsensusParametersPath   string
	ConsensusParametersLoader func() staking.ConsensusParameters
	DefaultFundingAmount      int64
	DefaultSelfEscrowAmount   int64
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
	g.genesis.Ledger = make(map[signature.PublicKey]*staking.Account)
	g.genesis.Delegations = make(map[signature.PublicKey]map[signature.PublicKey]*staking.Delegation)

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
	err = g.setupFaucet()
	if err != nil {
		return nil, err
	}

	logger.Debug("Loading all entitiy")
	// TODO Add a way to load a custom ledger amount
	// Load all entities and fund them
	for _, ent := range g.options.Entities.All() {
		g.setupEntity(ent, g.options.DefaultFundingAmount, g.options.DefaultSelfEscrowAmount)
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

func (g *genesisCreator) setupEntity(ent *entity.Entity, tokenBalance int64, tokensInEscrow int64) {
	g.setLedgerForEntity(ent.ID, tokenBalance, tokensInEscrow)
	g.setDelegation(ent.ID, ent.ID, tokensInEscrow)
}

func (g *genesisCreator) setLedgerForEntity(entityPubKey signature.PublicKey, tokenBalance int64, tokensInEscrow int64) {
	g.genesis.Ledger[entityPubKey] = &staking.Account{
		General: staking.GeneralAccount{
			Balance: *g.toStakingQuantity(tokenBalance),
			Nonce:   0,
		},
		Escrow: staking.EscrowAccount{
			Active: staking.SharePool{
				Balance:     *g.toStakingQuantity(tokensInEscrow),
				TotalShares: *g.toStakingQuantity(tokensInEscrow),
			},
			Debonding: staking.SharePool{
				Balance:     *g.toStakingQuantity(0),
				TotalShares: *g.toStakingQuantity(0),
			},
		},
	}
}

func (g *genesisCreator) setDelegation(fromEntityPubKey signature.PublicKey, toEntityPubKey signature.PublicKey, tokensToEscrow int64) {
	delegations, ok := g.genesis.Delegations[toEntityPubKey]
	if !ok {
		delegations = make(map[signature.PublicKey]*staking.Delegation)
	}
	delegations[fromEntityPubKey] = &staking.Delegation{
		Shares: *g.toStakingQuantity(tokensToEscrow),
	}
	g.genesis.Delegations[toEntityPubKey] = delegations
}

func (g *genesisCreator) setupFaucet() error {
	if g.options.FaucetBase64Address == "" {
		return nil
	}

	// Load faucet public key from the base64 string
	var faucetPubKey signature.PublicKey
	err := faucetPubKey.UnmarshalText([]byte(g.options.FaucetBase64Address))
	if err != nil {
		logger.Error("error loading faucet public key",
			"err", err,
		)
		return err
	}

	g.setLedgerForEntity(faucetPubKey, g.options.FaucetAmount, 0)
	return nil
}

func (g *genesisCreator) calculateCommonPool() error {
	var entityTotalBalances map[signature.PublicKey]*quantity.Quantity
	var err error

	allocatedTokens := quantity.NewQuantity()

	// Iterate through all of the accounts on the ledger
	for entityPubKey, account := range g.genesis.Ledger {
		q, ok := entityTotalBalances[entityPubKey]
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
	ent, err := g.options.Entities.ResolveEntity(name)
	if err != nil {
		return nil, err
	}
	return &ent.ID, nil
}

func (g *genesisCreator) loadConsensusParameters() error {
	if g.options.ConsensusParametersLoader != nil {
		params := g.options.ConsensusParametersLoader()
		g.genesis.Parameters = params
		return nil
	}
	b, err := ioutil.ReadFile(g.options.ConsensusParametersPath)
	if err != nil {
		return err
	}

	var params staking.ConsensusParameters
	err = json.Unmarshal(b, &params)
	if err != nil {
		return err
	}
	g.genesis.Parameters = params
	return nil
}
