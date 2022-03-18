package types

import (
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"math/big"
)

const (
	// ProposalTypeCommunityPoolSpend defines the type for a CommunityPoolSpendProposal
	ProposalTypeCommunityPoolSpend = "CommunityPoolSpend"
	CommunityPoolSpendByRouter     = "distribution"
	DefaultDepositDenom            = "FX"
	InitialDeposit                 = 1000
	EGFDepositProposalThreshold    = 100000
	ClaimRatio                     = "0.1"
)

// var FxDecimals = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
type EGFDepositParams struct {
	InitialDeposit           sdk.Coins `protobuf:"bytes,1,rep,name=initial_deposit,json=initialDeposit,proto3,castrepeated=github.com/cosmos/cosmos-sdk/types.Coins" json:"initial_deposit,omitempty" yaml:"initial_deposit"`
	ClaimRatio               sdk.Dec   `protobuf:"bytes,2,opt,name=claim_ratio,json=claimRatio,proto3,customtype=github.com/cosmos/cosmos-sdk/types.Dec" json:"claim_ratio,omitempty" yaml:"claim_ratio"`
	DepositProposalThreshold sdk.Coins `protobuf:"bytes,2,opt,name=deposit_proposal_threshold,json=depositProposalThreshold,proto3,castrepeated=github.com/cosmos/cosmos-sdk/types.Coins" json:"deposit_proposal_threshold,omitempty" yaml:"deposit_proposal_threshold"`
}

func validateEGFPDepositParams(i interface{}) error {
	v, ok := i.(EGFDepositParams)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	if !v.InitialDeposit.IsValid() {
		return fmt.Errorf("invalid initial deposit deposit: %s", v.InitialDeposit)
	}
	if !v.ClaimRatio.IsPositive() {
		return fmt.Errorf("claim ratio must be positive: %s", v.ClaimRatio)
	}
	if !v.ClaimRatio.LTE(sdk.OneDec()) {
		return fmt.Errorf("claim ratio too large: %s", v.ClaimRatio)
	}
	if !v.DepositProposalThreshold.IsValid() {
		return fmt.Errorf("invalid deposit proposal threshold: %s", v.InitialDeposit)
	}
	return nil
}

var SupportEGFProposalBlock = int64(0)

func SetEGFProposalSupportBlock(blockHeight int64) {
	SupportEGFProposalBlock = blockHeight
}

func GetEGFProposalSupportBlock() int64 {
	return SupportEGFProposalBlock
}

func DefaultEGFDepositParams() EGFDepositParams {
	return EGFDepositParams{
		InitialDeposit:           sdk.NewCoins(sdk.NewCoin(DefaultDepositDenom, sdk.NewInt(InitialDeposit).Mul(sdk.NewIntFromBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))))),
		ClaimRatio:               sdk.MustNewDecFromStr(ClaimRatio),
		DepositProposalThreshold: sdk.NewCoins(sdk.NewCoin(DefaultDepositDenom, sdk.NewInt(EGFDepositProposalThreshold).Mul(sdk.NewIntFromBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))))),
	}
}
