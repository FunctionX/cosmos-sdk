package keeper_test

import (
	"errors"
	"fmt"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	types2 "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"

	"github.com/cosmos/cosmos-sdk/simapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/gov/types"
)

func TestGetSetProposal(t *testing.T) {
	app := simapp.Setup(false)
	ctx := app.BaseApp.NewContext(false, tmproto.Header{})

	tp := TestProposal
	proposal, err := app.GovKeeper.SubmitProposal(ctx, tp)
	require.NoError(t, err)
	proposalID := proposal.ProposalId
	app.GovKeeper.SetProposal(ctx, proposal)

	gotProposal, ok := app.GovKeeper.GetProposal(ctx, proposalID)
	require.True(t, ok)
	require.True(t, proposal.Equal(gotProposal))
}

func TestActivateVotingPeriod(t *testing.T) {
	app := simapp.Setup(false)
	ctx := app.BaseApp.NewContext(false, tmproto.Header{})

	tp := TestProposal
	proposal, err := app.GovKeeper.SubmitProposal(ctx, tp)
	require.NoError(t, err)

	require.True(t, proposal.VotingStartTime.Equal(time.Time{}))

	app.GovKeeper.ActivateVotingPeriod(ctx, proposal)

	require.True(t, proposal.VotingStartTime.Equal(ctx.BlockHeader().Time))

	proposal, ok := app.GovKeeper.GetProposal(ctx, proposal.ProposalId)
	require.True(t, ok)

	activeIterator := app.GovKeeper.ActiveProposalQueueIterator(ctx, proposal.VotingEndTime)
	require.True(t, activeIterator.Valid())

	proposalID := types.GetProposalIDFromBytes(activeIterator.Value())
	require.Equal(t, proposalID, proposal.ProposalId)
	activeIterator.Close()
}

type invalidProposalRoute struct{ types.TextProposal }

func (invalidProposalRoute) ProposalRoute() string { return "nonexistingroute" }

func TestSubmitProposal(t *testing.T) {
	app := simapp.Setup(false)
	ctx := app.BaseApp.NewContext(false, tmproto.Header{})

	testCases := []struct {
		content     types.Content
		expectedErr error
	}{
		{&types.TextProposal{Title: "title", Description: "description"}, nil},
		// Keeper does not check the validity of title and description, no error
		{&types.TextProposal{Title: "", Description: "description"}, nil},
		{&types.TextProposal{Title: strings.Repeat("1234567890", 100), Description: "description"}, nil},
		{&types.TextProposal{Title: "title", Description: ""}, nil},
		{&types.TextProposal{Title: "title", Description: strings.Repeat("1234567890", 1000)}, nil},
		// error only when invalid route
		{&invalidProposalRoute{}, types.ErrNoProposalHandlerExists},
	}

	for i, tc := range testCases {
		_, err := app.GovKeeper.SubmitProposal(ctx, tc.content)
		require.True(t, errors.Is(tc.expectedErr, err), "tc #%d; got: %v, expected: %v", i, err, tc.expectedErr)
	}
}

func TestGetProposalsFiltered(t *testing.T) {
	proposalID := uint64(1)
	app := simapp.Setup(false)
	ctx := app.BaseApp.NewContext(false, tmproto.Header{})

	status := []types.ProposalStatus{types.StatusDepositPeriod, types.StatusVotingPeriod}

	addr1 := sdk.AccAddress("foo_________________")

	for _, s := range status {
		for i := 0; i < 50; i++ {
			p, err := types.NewProposal(TestProposal, proposalID, time.Now(), time.Now())
			require.NoError(t, err)

			p.Status = s

			if i%2 == 0 {
				d := types.NewDeposit(proposalID, addr1, nil)
				v := types.NewVote(proposalID, addr1, types.OptionYes)
				app.GovKeeper.SetDeposit(ctx, d)
				app.GovKeeper.SetVote(ctx, v)
			}

			app.GovKeeper.SetProposal(ctx, p)
			proposalID++
		}
	}

	testCases := []struct {
		params             types.QueryProposalsParams
		expectedNumResults int
	}{
		{types.NewQueryProposalsParams(1, 50, types.StatusNil, nil, nil), 50},
		{types.NewQueryProposalsParams(1, 50, types.StatusDepositPeriod, nil, nil), 50},
		{types.NewQueryProposalsParams(1, 50, types.StatusVotingPeriod, nil, nil), 50},
		{types.NewQueryProposalsParams(1, 25, types.StatusNil, nil, nil), 25},
		{types.NewQueryProposalsParams(2, 25, types.StatusNil, nil, nil), 25},
		{types.NewQueryProposalsParams(1, 50, types.StatusRejected, nil, nil), 0},
		{types.NewQueryProposalsParams(1, 50, types.StatusNil, addr1, nil), 50},
		{types.NewQueryProposalsParams(1, 50, types.StatusNil, nil, addr1), 50},
		{types.NewQueryProposalsParams(1, 50, types.StatusNil, addr1, addr1), 50},
		{types.NewQueryProposalsParams(1, 50, types.StatusDepositPeriod, addr1, addr1), 25},
		{types.NewQueryProposalsParams(1, 50, types.StatusDepositPeriod, nil, nil), 50},
		{types.NewQueryProposalsParams(1, 50, types.StatusVotingPeriod, nil, nil), 50},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Test Case %d", i), func(t *testing.T) {
			proposals := app.GovKeeper.GetProposalsFiltered(ctx, tc.params)
			require.Len(t, proposals, tc.expectedNumResults)

			for _, p := range proposals {
				if types.ValidProposalStatus(tc.params.ProposalStatus) {
					require.Equal(t, tc.params.ProposalStatus, p.Status)
				}
			}
		})
	}
}

func TestKeeper_SupportEGFProposalTotDepositProposal(t *testing.T) {
	app := simapp.Setup(false)
	ctx := app.BaseApp.NewContext(false, tmproto.Header{}).WithBlockHeight(100)

	types.SetEGFProposalSupportBlock(100)
	app.GovKeeper.SetEGFDepositParams(ctx, types.EGFDepositParams{
		InitialDeposit:           sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(types.InitialDeposit))),
		ClaimRatio:               sdk.MustNewDecFromStr(types.ClaimRatio),
		DepositProposalThreshold: sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(types.EGFDepositProposalThreshold))),
	})

	address, err := sdk.AccAddressFromBech32("cosmos1t5u0jfg3ljsjrh2m9e47d4ny2hea7eehxrzdgd")
	if err != nil {
		t.Fatal(err)
	}
	fp2 := types2.FeePool{CommunityPool: sdk.DecCoins{{Denom: "stake", Amount: sdk.NewDec(10000000000000)}}}
	app.DistrKeeper.SetFeePool(ctx, fp2)

	moduleacc := authtypes.NewModuleAddress("distribution")
	err = app.BankKeeper.AddCoins(ctx, moduleacc, sdk.NewCoins(sdk.NewCoin(app.StakingKeeper.BondDenom(ctx), sdk.NewInt(100000000000000000))))
	if err != nil {
		panic(err)
	}
	addresses := simapp.AddTestAddrsIncremental(app, ctx, 2, sdk.NewInt(100000000000000000))
	fourStake := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(101000)))

	tp := types2.NewCommunityPoolSpendProposal("test", "description", address, sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(1000000))))
	proposal, err := app.GovKeeper.SubmitProposal(ctx, tp)
	require.NoError(t, err)

	require.True(t, proposal.VotingStartTime.Equal(time.Time{}))

	// Check first deposit
	votingStarted, err := app.GovKeeper.AddDeposit(ctx, proposal.ProposalId, addresses[0], fourStake)
	require.NoError(t, err)

	require.True(t, votingStarted)

	app.GovKeeper.ActivateVotingPeriod(ctx, proposal)

	require.True(t, proposal.VotingStartTime.Equal(ctx.BlockHeader().Time))

	proposal, ok := app.GovKeeper.GetProposal(ctx, proposal.ProposalId)
	require.True(t, ok)

	activeIterator := app.GovKeeper.ActiveProposalQueueIterator(ctx, proposal.VotingEndTime)
	require.True(t, activeIterator.Valid())

	proposalID := types.GetProposalIDFromBytes(activeIterator.Value())
	require.Equal(t, proposalID, proposal.ProposalId)
	activeIterator.Close()
}

func TestKeeper_SupportNotEGFProposalTotDepositProposal(t *testing.T) {
	app := simapp.Setup(false)
	ctx := app.BaseApp.NewContext(false, tmproto.Header{}).WithBlockHeight(100)

	types.SetEGFProposalSupportBlock(100)
	app.GovKeeper.SetEGFDepositParams(ctx, types.EGFDepositParams{
		InitialDeposit:           sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(types.InitialDeposit))),
		ClaimRatio:               sdk.MustNewDecFromStr(types.ClaimRatio),
		DepositProposalThreshold: sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(types.EGFDepositProposalThreshold))),
	})

	addresses := simapp.AddTestAddrsIncremental(app, ctx, 2, sdk.NewInt(100000000000000000))
	fourStake := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(10000000)))

	tp := TestProposal
	proposal, err := app.GovKeeper.SubmitProposal(ctx, tp)
	require.NoError(t, err)

	require.True(t, proposal.VotingStartTime.Equal(time.Time{}))

	// Check first deposit
	votingStarted, err := app.GovKeeper.AddDeposit(ctx, proposal.ProposalId, addresses[0], fourStake)
	require.NoError(t, err)

	require.True(t, votingStarted)

	app.GovKeeper.ActivateVotingPeriod(ctx, proposal)

	require.True(t, proposal.VotingStartTime.Equal(ctx.BlockHeader().Time))

	proposal, ok := app.GovKeeper.GetProposal(ctx, proposal.ProposalId)
	require.True(t, ok)

	activeIterator := app.GovKeeper.ActiveProposalQueueIterator(ctx, proposal.VotingEndTime)
	require.True(t, activeIterator.Valid())

	proposalID := types.GetProposalIDFromBytes(activeIterator.Value())
	require.Equal(t, proposalID, proposal.ProposalId)
	activeIterator.Close()
}
