package app

import (
	gethcmn "github.com/ethereum/go-ethereum/common"
	gethcore "github.com/ethereum/go-ethereum/core"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"

	stake "github.com/zeniqsmart/zeniq-smart-chain/staking/types"
)

type Validator struct {
	Address      gethcmn.Address `json:"address"`
	Pubkey       gethcmn.Hash    `json:"pubkey"`
	RewardTo     gethcmn.Address `json:"reward_to"`
	VotingPower  int64           `json:"voting_power"`
	Introduction string          `json:"introduction"`
	StakedCoins  gethcmn.Hash    `json:"staked_coins"`
	IsRetiring   bool            `json:"is_retiring"`
	MinerAddress crypto.Address  `json:"miner_address"`
}

type GenesisData struct {
	Validators []*Validator          `json:"validators"`
	Alloc      gethcore.GenesisAlloc `json:"alloc"`
}

func (g GenesisData) StakingValidators() []*stake.Validator {
	ret := make([]*stake.Validator, len(g.Validators))
	for i, v := range g.Validators {
		ret[i] = &stake.Validator{
			Address:      v.Address,
			Pubkey:       v.Pubkey,
			RewardTo:     v.RewardTo,
			VotingPower:  v.VotingPower,
			Introduction: v.Introduction,
			StakedCoins:  v.StakedCoins,
			IsRetiring:   v.IsRetiring,
		}
	}
	return ret
}

func FromStakingValidators(vs []*stake.Validator) []*Validator {
	ret := make([]*Validator, len(vs))
	for i, v := range vs {
		ret[i] = FromStakingValidator(v)
	}
	return ret
}

func FromStakingValidator(v *stake.Validator) *Validator {
	return &Validator{
		Address:      v.Address,
		Pubkey:       v.Pubkey,
		RewardTo:     v.RewardTo,
		VotingPower:  v.VotingPower,
		Introduction: v.Introduction,
		StakedCoins:  v.StakedCoins,
		IsRetiring:   v.IsRetiring,
		MinerAddress: ed25519.PubKey(v.Pubkey[:]).Address(),
	}
}
