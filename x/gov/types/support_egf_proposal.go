package types

import sdk "github.com/cosmos/cosmos-sdk/types"

const (
	// ProposalTypeCommunityPoolSpend defines the type for a CommunityPoolSpendProposal
	ProposalTypeCommunityPoolSpend = "CommunityPoolSpend"
	InitialDeposit                 = 1000
	EGFDepositProposalThreshold    = 100000
	claimRatio                     = 10
)

var EGFProposalSupportBscBlock = int64(0)

func SetEGFProposalSupportBscBlock(blockHeight int64) {
	EGFProposalSupportBscBlock = blockHeight
}

func getEGFProposalSupportBscBlock() int64 {
	return EGFProposalSupportBscBlock
}

func SupportEGFProposal(ctx sdk.Context, proposalType string) bool {
	if EGFProposalSupportBscBlock > 0 && ctx.BlockHeight() > getEGFProposalSupportBscBlock() && ProposalTypeCommunityPoolSpend == proposalType {
		return true
	}
	return false
}

func SupportEGFProposalTotDepositProposal(initialDeposit, claimCoin sdk.Coin) sdk.Coins {
	if claimCoin.Amount.LTE(sdk.NewInt(EGFDepositProposalThreshold)) {
		return sdk.Coins{initialDeposit}
	}
	amount := claimCoin.Amount.Quo(sdk.NewInt(claimRatio))
	coin := initialDeposit.Add(sdk.Coin{Denom: initialDeposit.Denom, Amount: amount})
	return sdk.Coins{coin}
}
