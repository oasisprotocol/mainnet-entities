package stakinggenesis_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/amber-network-entities/go/genesis-tools/stakinggenesis"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	fileSigner "github.com/oasisprotocol/oasis-core/go/common/crypto/signature/signers/file"
	"github.com/oasisprotocol/oasis-core/go/common/entity"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
)

type fakeEntities struct {
	count    int
	entities map[string]*entity.Entity
}

func MakeFakeEntities(count int) *fakeEntities {
	e := fakeEntities{
		count:    count,
		entities: make(map[string]*entity.Entity),
	}
	e.generateAll()
	return &e
}

func (e *fakeEntities) generateAll() {
	for i := 0; i < e.count; i++ {
		ent, err := e.generateEntity()
		if err != nil {
			panic(err)
		}
		e.entities[fmt.Sprintf("%d", i)] = ent
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
	if err != nil {
		return nil, err
	}
	return ent, nil
}

func (e *fakeEntities) All() map[string]*entity.Entity {
	return e.entities
}

func (e *fakeEntities) ResolveEntity(name string) (*entity.Entity, error) {
	return nil, nil
}

func genericGenesisOptions(entCount int) stakinggenesis.GenesisOptions {
	entities := MakeFakeEntities(entCount)
	return stakinggenesis.GenesisOptions{
		Entities:                entities,
		TotalSupply:             10_000_000_000,
		PrecisionConstant:       10,
		DefaultSelfEscrowAmount: 250,
		DefaultFundingAmount:    250,
		ConsensusParametersLoader: func() staking.ConsensusParameters {
			return staking.ConsensusParameters{}
		},
	}
}

func TestGenerateStakingLedger(t *testing.T) {
	options := genericGenesisOptions(10)
	genesis, err := stakinggenesis.Create(options)
	if err != nil {
		require.NoError(t, err)
	}
	require.Equal(t, "99999950000", genesis.CommonPool.String())
}

func TestGenerateStakingLedgerWithFaucet(t *testing.T) {
	options := genericGenesisOptions(10)
	options.FaucetBase64Address = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa="
	options.FaucetAmount = 1_000_000
	genesis, err := stakinggenesis.Create(options)
	if err != nil {
		require.NoError(t, err)
	}
	require.Equal(t, "99989950000", genesis.CommonPool.String())
}

func TestLoadStakingParameters(t *testing.T) {
	// This is a bit brittle
	params, err := stakinggenesis.LoadStakingConsensusParameters("fixtures/staking_ledger.json")

	require.NoError(t, err)

	require.Equal(t, params.Thresholds[staking.KindEntity], *quantity.NewFromUint64(100_000_000_000))
}
