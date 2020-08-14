package stakinggenesis

import (
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	"gopkg.in/yaml.v2"
)

// GenesisAllocations allows us to turn entity names into a wallet
// allocation in a human readable way
type GenesisAllocations interface {
	ResolveAllocation(name string) *quantity.Quantity
}

// BasicGenesisAllocations an implementation of a basic genesis allocations
// interface
type BasicGenesisAllocations struct {
	allocations map[string]*quantity.Quantity
}

// NewGenesisAllocationsFromFile loads a genesis allocation from a yaml file
func NewGenesisAllocationsFromFile(path string, precision uint64) (*BasicGenesisAllocations, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	rawAllocations := make(map[string]string)
	err = yaml.Unmarshal(bytes, &rawAllocations)
	if err != nil {
		return nil, err
	}

	allocations := make(map[string]*quantity.Quantity)
	// Convert each of the values into a quantity
	for entityName, allocationString := range rawAllocations {
		allocationInt, err := strconv.ParseUint(allocationString, 10, 64)
		if err != nil {
			return nil, err
		}
		allocation := quantity.NewFromUint64(allocationInt * precision)
		allocations[strings.ToLower(entityName)] = allocation
	}
	return &BasicGenesisAllocations{allocations: allocations}, nil
}

// ResolveAllocation resolves the allocation based on the name of the entity
func (a *BasicGenesisAllocations) ResolveAllocation(name string) *quantity.Quantity {
	searchName := strings.ToLower(name)
	allocation, ok := a.allocations[searchName]
	if !ok {
		return quantity.NewFromUint64(0)
	}
	return allocation
}
