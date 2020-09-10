package stakinggenesis_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/mainnet-entities/go/genesis-tools/stakinggenesis"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
)

const hexCharset = "0123456789abcdef"

var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixNano()))

func randomStakingAddress() staking.Address {
	b := make([]byte, 64)
	for i := range b {
		b[i] = hexCharset[seededRand.Intn(len(hexCharset))]
	}

	pub := signature.NewPublicKey(string(b))

	return staking.NewAddress(pub)
}

func requireQuantityEqual(t *testing.T, actual quantity.Quantity, expected uint64) {
	if expected == 0 {
		require.True(t, actual.IsZero())
	} else {
		require.Equal(t, actual, *quantity.NewFromUint64(expected))
	}
}

func baseAccountingGenesis() *stakinggenesis.AccountingGenesis {
	return stakinggenesis.NewAccountingGenesis(1_000_000_000, 10_000_000_000, 10000, 0, 5000)
}

func TestLoadAccountingGenesis(t *testing.T) {
	genesis := baseAccountingGenesis()

	partial := genesis.GetPartialGenesis()

	require.Equal(t, partial.TotalSupply, *quantity.NewFromUint64(10_000_000_000_000_000_000))
	require.Equal(t, partial.CommonPool, *quantity.NewFromUint64(10_000_000_000_000_000_000))

}

func TestAddAccountsToAccountingGenesis(t *testing.T) {
	genesis := baseAccountingGenesis()

	testAddress1 := randomStakingAddress()
	testAddress2 := randomStakingAddress()

	genesis.AddAccount(testAddress1, quantity.NewFromUint64(1_000_000_000))
	genesis.AddAccount(testAddress2, quantity.NewFromUint64(3_000_000_000))

	partial := genesis.GetPartialGenesis()

	require.Equal(t, partial.TotalSupply, *quantity.NewFromUint64(10_000_000_000_000_000_000))
	require.Equal(t, partial.CommonPool, *quantity.NewFromUint64(6_000_000_000_000_000_000))
}

func TestAddAccountsAndDelegationToAccountingGenesis(t *testing.T) {
	genesis := baseAccountingGenesis()

	testAddress1 := randomStakingAddress()
	testAddress2 := randomStakingAddress()
	testAddress3 := randomStakingAddress()
	testAddress4 := randomStakingAddress()

	genesis.AddAccount(testAddress1, quantity.NewFromUint64(1_000_000_000))
	genesis.AddAccount(testAddress2, quantity.NewFromUint64(1_000_000_000))
	genesis.AddAccount(testAddress3, quantity.NewFromUint64(0))
	genesis.AddAccount(testAddress4, quantity.NewFromUint64(0))

	genesis.AddDelegation(testAddress1, testAddress1, quantity.NewFromUint64(100_000_000))
	genesis.AddDelegation(testAddress1, testAddress2, quantity.NewFromUint64(100_000_000))
	genesis.AddDelegation(testAddress1, testAddress3, quantity.NewFromUint64(100_000_000))
	genesis.AddDelegation(testAddress1, testAddress4, quantity.NewFromUint64(100_000_000))

	genesis.AddDelegation(testAddress2, testAddress1, quantity.NewFromUint64(200_000_000))
	genesis.AddDelegation(testAddress2, testAddress3, quantity.NewFromUint64(200_000_000))
	genesis.AddDelegation(testAddress2, testAddress4, quantity.NewFromUint64(200_000_000))

	partial := genesis.GetPartialGenesis()

	// Check balances
	requireQuantityEqual(t, partial.Ledger[testAddress1].General.Balance, 600_000_000_000_000_000)
	requireQuantityEqual(t, partial.Ledger[testAddress2].General.Balance, 400_000_000_000_000_000)
	requireQuantityEqual(t, partial.Ledger[testAddress3].General.Balance, 0)
	requireQuantityEqual(t, partial.Ledger[testAddress4].General.Balance, 0)

	// Check delegations
	requireQuantityEqual(t, partial.Ledger[testAddress1].Escrow.Active.Balance, 300_000_000_000_000_000)
	requireQuantityEqual(t, partial.Ledger[testAddress2].Escrow.Active.Balance, 100_000_000_000_000_000)
	requireQuantityEqual(t, partial.Ledger[testAddress3].Escrow.Active.Balance, 300_000_000_000_000_000)
	requireQuantityEqual(t, partial.Ledger[testAddress4].Escrow.Active.Balance, 300_000_000_000_000_000)

	// Check delegations
	requireQuantityEqual(t, partial.Delegations[testAddress1][testAddress1].Shares, 100_000_000_000_000_000)
	requireQuantityEqual(t, partial.Delegations[testAddress1][testAddress2].Shares, 200_000_000_000_000_000)
	require.Equal(t, len(partial.Delegations[testAddress1]), 2)

	requireQuantityEqual(t, partial.Delegations[testAddress2][testAddress1].Shares, 100_000_000_000_000_000)
	require.Equal(t, len(partial.Delegations[testAddress2]), 1)

	requireQuantityEqual(t, partial.Delegations[testAddress3][testAddress1].Shares, 100_000_000_000_000_000)
	requireQuantityEqual(t, partial.Delegations[testAddress3][testAddress2].Shares, 200_000_000_000_000_000)
	require.Equal(t, len(partial.Delegations[testAddress3]), 2)

	requireQuantityEqual(t, partial.Delegations[testAddress4][testAddress1].Shares, 100_000_000_000_000_000)
	requireQuantityEqual(t, partial.Delegations[testAddress4][testAddress2].Shares, 200_000_000_000_000_000)
	require.Equal(t, len(partial.Delegations[testAddress4]), 2)
}

func TestDelegateTooMuchAccountingGenesis(t *testing.T) {
	genesis := baseAccountingGenesis()

	testAddress1 := randomStakingAddress()
	testAddress2 := randomStakingAddress()

	genesis.AddAccount(testAddress1, quantity.NewFromUint64(1_000_000_000))
	genesis.AddAccount(testAddress2, quantity.NewFromUint64(0))

	err := genesis.AddDelegation(testAddress1, testAddress2, quantity.NewFromUint64(1_000_000_001))
	require.Error(t, err, "insufficient balance")
}
