package stakinggenesis

import (
	"fmt"

	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
)

// AccountingGenesis a partial genesis used to initialize the accounting
// (ledger/delegations) of the genesis. Totals which can be used for common pool
// calculations are tracked
type AccountingGenesis struct {
	ledger               StakingAccounts
	delegations          StakingDelegations
	totalAllocatedTokens *quantity.Quantity
	totalSupply          *quantity.Quantity
	commissionRateMax    *quantity.Quantity
	commissionRateMin    *quantity.Quantity
	commissionRate       *quantity.Quantity
	precision            uint64
}

func NewAccountingGenesis(precision, totalSupply, commissionRateMax, commissionRateMin, commissionRate uint64) *AccountingGenesis {
	return &AccountingGenesis{
		ledger:               make(StakingAccounts),
		delegations:          make(StakingDelegations),
		totalAllocatedTokens: quantity.NewFromUint64(0),
		precision:            precision,
		totalSupply:          quantity.NewFromUint64(totalSupply),
		commissionRateMax:    quantity.NewFromUint64(commissionRateMax),
		commissionRateMin:    quantity.NewFromUint64(commissionRateMin),
		commissionRate:       quantity.NewFromUint64(commissionRate),
	}
}

func (a *AccountingGenesis) preciseTokens(q *quantity.Quantity) *quantity.Quantity {
	preciseTokens := quantity.NewFromUint64(a.precision)
	preciseTokens.Mul(q)
	return preciseTokens
}

func (a *AccountingGenesis) preciseTokensFromUint64(tokens uint64) *quantity.Quantity {
	return a.preciseTokens(quantity.NewFromUint64(tokens))
}

// AddAccount initializes an account on the AccountingGenesis
func (a *AccountingGenesis) AddAccount(address staking.Address, tokenBalance *quantity.Quantity) error {
	if a.accountExists(address) {
		return fmt.Errorf(`duplicate account found for "%s"`, address)
	}

	preciseTokenBalance := a.preciseTokens(tokenBalance)

	a.ledger[address] = &staking.Account{
		General: staking.GeneralAccount{
			Balance: *preciseTokenBalance.Clone(),
			Nonce:   0,
		},
		Escrow: staking.EscrowAccount{
			Active: staking.SharePool{
				Balance:     *quantity.NewFromUint64(0),
				TotalShares: *quantity.NewFromUint64(0),
			},
			Debonding: staking.SharePool{
				Balance:     *quantity.NewFromUint64(0),
				TotalShares: *quantity.NewFromUint64(0),
			},
		},
	}

	a.totalAllocatedTokens.Add(preciseTokenBalance)

	return nil
}

func (a *AccountingGenesis) accountExists(address staking.Address) bool {
	_, ok := a.ledger[address]
	return ok
}

func (a *AccountingGenesis) AddDelegation(from staking.Address, to staking.Address, amount *quantity.Quantity) error {
	preciseAmount := a.preciseTokens(amount)

	// Ensure that the accounts exist
	if !a.accountExists(from) {
		return fmt.Errorf(`cannot delegate. account "%s" does not exist`, from)
	}
	if !a.accountExists(to) {
		return fmt.Errorf(`cannot delegate. account "%s" does not exist`, to)
	}

	if _, ok := a.delegations[to]; !ok {
		a.delegations[to] = make(map[staking.Address]*staking.Delegation)
	}

	// Check that this isn't a duplicate delegation
	if _, ok := a.delegations[to][from]; ok {
		return fmt.Errorf(`duplicate delegation from "%s" to "%s"`, from, to)
	}

	// Subtract from the "from" account to escrow into the "to" acocunt
	err := a.ledger[from].General.Balance.Sub(preciseAmount)
	if err != nil {
		return err
	}

	a.delegations[to][from] = &staking.Delegation{
		Shares: *preciseAmount.Clone(),
	}

	err = a.ledger[to].Escrow.Active.Balance.Add(preciseAmount)
	if err != nil {
		return err
	}
	err = a.ledger[to].Escrow.Active.TotalShares.Add(preciseAmount)
	if err != nil {
		return err
	}

	// Ensure the commission schedule is set since this account is getting
	// delegations. This is currently set to a bound of 0-20% and a starting rate of 5%
	a.ledger[to].Escrow.CommissionSchedule.Rates = []staking.CommissionRateStep{
		{
			Start: 0,
			Rate:  *a.commissionRate.Clone(),
		},
	}

	a.ledger[to].Escrow.CommissionSchedule.Bounds = []staking.CommissionRateBoundStep{
		{
			Start:   0,
			RateMin: *a.commissionRateMin.Clone(),
			RateMax: *a.commissionRateMax.Clone(),
		},
	}

	return nil
}

func (a *AccountingGenesis) GetPartialGenesis() staking.Genesis {
	preciseTotalSupply := a.preciseTokens(a.totalSupply)

	preciseCommonPool := preciseTotalSupply.Clone()
	preciseCommonPool.Sub(a.totalAllocatedTokens)

	return staking.Genesis{
		Ledger:      a.ledger,
		Delegations: a.delegations,
		TotalSupply: *preciseTotalSupply,
		CommonPool:  *preciseCommonPool,
	}
}
