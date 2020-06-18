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
	entities map[string]*stakinggenesis.EntityInfo
}

func MakeFakeEntities(count int, allocation uint64) *fakeEntities {
	e := fakeEntities{
		count:    count,
		entities: make(map[string]*stakinggenesis.EntityInfo),
	}
	e.generateAll(allocation)
	return &e
}

func (e *fakeEntities) generateAll(allocation uint64) {
	for i := 0; i < e.count; i++ {
		ent, err := e.generateEntity(allocation)
		if err != nil {
			panic(err)
		}
		e.entities[fmt.Sprintf("%d", i)] = ent
	}
}

func (e *fakeEntities) generateEntity(allocation uint64) (*stakinggenesis.EntityInfo, error) {
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

	info := stakinggenesis.NewEntityInfo(quantity.NewFromUint64(allocation), ent)
	if err != nil {
		return nil, err
	}
	return info, nil
}

func (e *fakeEntities) All() map[string]*stakinggenesis.EntityInfo {
	return e.entities
}

func (e *fakeEntities) ResolveEntity(name string) (*stakinggenesis.EntityInfo, error) {
	return nil, nil
}

func genericGenesisOptions(entCount int) stakinggenesis.GenesisOptions {
	entities := MakeFakeEntities(entCount, 2500)
	return stakinggenesis.GenesisOptions{
		Entities:          entities,
		TotalSupply:       10_000_000_000,
		PrecisionConstant: 10,
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
	require.Equal(t, "99999975000", genesis.CommonPool.String())
}

func TestGenerateStakingLedgerWithFaucet(t *testing.T) {
	options := genericGenesisOptions(10)
	options.AdditionalEntitiesToFund = map[string]int64{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa=": 1_000_000,
	}
	genesis, err := stakinggenesis.Create(options)
	if err != nil {
		require.NoError(t, err)
	}
	require.Equal(t, "99989975000", genesis.CommonPool.String())
}

func TestLoadStakingParameters(t *testing.T) {
	// This is a bit brittle
	params, err := stakinggenesis.LoadStakingConsensusParameters("fixtures/staking_params.json")

	require.NoError(t, err)

	require.Equal(t, params.Thresholds[staking.KindEntity], *quantity.NewFromUint64(100_000_000_000))
}
