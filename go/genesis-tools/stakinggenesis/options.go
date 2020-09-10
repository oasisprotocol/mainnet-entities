package stakinggenesis

import (
	"encoding/json"
	"io/ioutil"

	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
)

// GenesisOptions options for the staking genesis document.
type GenesisOptions struct {
	IsTestGenesis             bool
	ConfigurationPath         string
	ConsensusParametersPath   string
	ConsensusParametersLoader func() staking.ConsensusParameters
	Entities                  Entities
}

func (g GenesisOptions) LoadConsensusParameters() (*staking.ConsensusParameters, error) {
	if g.ConsensusParametersLoader != nil {
		params := g.ConsensusParametersLoader()
		return &params, nil
	}
	params, err := LoadStakingConsensusParameters(g.ConsensusParametersPath)
	return params, err
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
