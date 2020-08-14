package stakinggenesis_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/mainnet-entities/go/genesis-tools/stakinggenesis"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
)

func TestLoadAllocations(t *testing.T) {
	var precision uint64 = 1_000_000_000
	allocations, err := stakinggenesis.NewGenesisAllocationsFromFile("fixtures/staking_ledger_allocations.yaml", precision)

	require.NoError(t, err)

	test1 := allocations.ResolveAllocation("test1")
	require.Equal(t, quantity.NewFromUint64(10_000_000_000_000), test1)

	test1_caps := allocations.ResolveAllocation("TEST1")
	require.Equal(t, quantity.NewFromUint64(10_000_000_000_000), test1_caps)

	test2 := allocations.ResolveAllocation("test2")
	require.Equal(t, quantity.NewFromUint64(207_000_000_000), test2)

	test3 := allocations.ResolveAllocation("test3")
	require.Equal(t, quantity.NewFromUint64(1_000_000_000), test3)

}
