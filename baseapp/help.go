package baseapp

import (
	storetypes "cosmossdk.io/store/types"
	"fmt"
	"github.com/cockroachdb/errors"
	"github.com/cosmos/cosmos-sdk/types/mempool"
	"sync"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	coreheader "cosmossdk.io/core/header"
	errorsmod "cosmossdk.io/errors"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

func (app *BaseApp) CheckHalt(height int64, time time.Time) error {
	return app.checkHalt(height, time)
}

func (app *BaseApp) ValidateFinalizeBlockHeight(req *abci.RequestFinalizeBlock) error {
	return app.validateFinalizeBlockHeight(req)
}

func (app *BaseApp) GetIndexEvents() map[string]struct{} {
	return app.indexEvents
}

func (app *BaseApp) GetTxDecoder() sdk.TxDecoder {
	return app.txDecoder
}

func (app *BaseApp) DeliverTx(tx []byte) *abci.ExecTxResult {
	return app.deliverTx(tx)
}

func (app *BaseApp) WorkingHash() []byte {
	return app.workingHash()
}

func (app *BaseApp) BuildFinalizeBlockState(req *abci.RequestFinalizeBlock) *state {
	header := cmtproto.Header{
		ChainID:            app.chainID,
		Height:             req.Height,
		Time:               req.Time,
		ProposerAddress:    req.ProposerAddress,
		NextValidatorsHash: req.NextValidatorsHash,
		AppHash:            app.LastCommitID().Hash,
	}

	// finalizeBlockState should be set on InitChain or ProcessProposal. If it is
	// nil, it means we are replaying this block and we need to set the state here
	// given that during block replay ProcessProposal is not executed by CometBFT.
	if app.finalizeBlockState == nil {
		app.setState(execModeFinalize, header)
	}

	// Context is now updated with Header information.
	app.finalizeBlockState.ctx = app.finalizeBlockState.ctx.
		WithBlockHeader(header).
		WithHeaderHash(req.Hash).
		WithHeaderInfo(coreheader.Info{
			ChainID: app.chainID,
			Height:  req.Height,
			Time:    req.Time,
			Hash:    req.Hash,
			AppHash: app.LastCommitID().Hash,
		}).
		WithConsensusParams(app.GetConsensusParams(app.finalizeBlockState.ctx)).
		WithVoteInfos(req.DecidedLastCommit.Votes).
		WithExecMode(sdk.ExecModeFinalize).
		WithCometInfo(cometInfo{
			Misbehavior:     req.Misbehavior,
			ValidatorsHash:  req.NextValidatorsHash,
			ProposerAddress: req.ProposerAddress,
			LastCommit:      req.DecidedLastCommit,
		})

	// GasMeter must be set after we get a context with updated consensus params.
	gasMeter := app.getBlockGasMeter(app.finalizeBlockState.ctx)
	app.finalizeBlockState.ctx = app.finalizeBlockState.ctx.WithBlockGasMeter(gasMeter)

	if app.checkState != nil {
		app.checkState.ctx = app.checkState.ctx.
			WithBlockGasMeter(gasMeter).
			WithHeaderHash(req.Hash)
	}

	return app.finalizeBlockState
}

func (app *BaseApp) IsInitHeight(height int64) bool {
	return app.initialHeight == height
}

func ExecModeProcessProposal() uint8 {
	return uint8(execModeProcessProposal)
}

func ExecModeFinalize() uint8 {
	return uint8(execModeFinalize)
}

func (app *BaseApp) DeliverTxWithMode(ctx sdk.Context, mode uint8, tx []byte) *abci.ExecTxResult {
	gInfo := sdk.GasInfo{}
	resultStr := "successful"

	defer func() {
		telemetry.IncrCounter(1, "tx", "count")
		telemetry.IncrCounter(1, "tx", resultStr)
		telemetry.SetGauge(float32(gInfo.GasUsed), "tx", "gas", "used")
		telemetry.SetGauge(float32(gInfo.GasWanted), "tx", "gas", "wanted")
	}()

	gInfo, result, anteEvents, err := app.runTxWithContext(ctx, execMode(mode), tx)
	if err != nil {
		resultStr = "failed"
	}
	return app.buildExecTxResult(gInfo, result, anteEvents, err)
}

func (app *BaseApp) buildExecTxResult(gInfo sdk.GasInfo, result *sdk.Result, anteEvents []abci.Event, err error) *abci.ExecTxResult {
	if err != nil {
		return sdkerrors.ResponseExecTxResultWithEvents(
			err,
			gInfo.GasWanted,
			gInfo.GasUsed,
			sdk.MarkEventsToIndex(anteEvents, app.indexEvents),
			app.trace,
		)
	}

	return &abci.ExecTxResult{
		GasWanted: int64(gInfo.GasWanted),
		GasUsed:   int64(gInfo.GasUsed),
		Log:       result.Log,
		Data:      result.Data,
		Events:    sdk.MarkEventsToIndex(result.Events, app.indexEvents),
	}
}

type ChannelResult struct {
	TxIndex int
	Result  *abci.ExecTxResult
}

func NewChannelResult(txIndex int, result *abci.ExecTxResult) ChannelResult {
	return ChannelResult{TxIndex: txIndex, Result: result}
}

func (app *BaseApp) PreCheckTx(ctx sdk.Context, modeValue uint8, txBytes []byte, txIndex int, wg *sync.WaitGroup, resultChan chan<- ChannelResult) {
	defer wg.Done()
	info, result, events, err := app.preCheckTx(ctx, execMode(modeValue), txBytes)
	execTxResult := &abci.ExecTxResult{
		GasWanted: int64(info.GasWanted),
		GasUsed:   int64(info.GasUsed),
	}
	if err != nil {
		execTxResult = app.buildExecTxResult(info, result, events, err)
	}
	resultChan <- ChannelResult{txIndex, execTxResult}
}

func (app *BaseApp) preCheckTx(ctx sdk.Context, mode execMode, txBytes []byte) (gInfo sdk.GasInfo, result *sdk.Result, anteEvents []abci.Event, err error) {
	// NOTE: GasWanted should be returned by the AnteHandler. GasUsed is
	// determined by the GasMeter. We need access to the context to get the gas
	// meter, so we initialize upfront.
	var gasWanted uint64

	//ctx := app.getContextForTx(mode, txBytes)
	ms := ctx.MultiStore()

	// only run the tx if there is block gas remaining
	if isFinalized(mode) && ctx.BlockGasMeter().IsOutOfGas() {
		return gInfo, nil, nil, errorsmod.Wrap(sdkerrors.ErrOutOfGas, "no block gas left to run tx")
	}

	defer func() {
		if r := recover(); r != nil {
			recoveryMW := newOutOfGasRecoveryMiddleware(gasWanted, ctx, app.runTxRecoveryMiddleware)
			err, result = processRecovery(r, recoveryMW), nil
		}

		gInfo = sdk.GasInfo{GasWanted: gasWanted, GasUsed: ctx.GasMeter().GasConsumed()}
	}()

	blockGasConsumed := false

	// consumeBlockGas makes sure block gas is consumed at most once. It must
	// happen after tx processing, and must be executed even if tx processing
	// fails. Hence, it's execution is deferred.
	consumeBlockGas := func() {
		if !blockGasConsumed {
			blockGasConsumed = true
			ctx.BlockGasMeter().ConsumeGas(
				ctx.GasMeter().GasConsumedToLimit(), "block gas meter",
			)
		}
	}

	// If BlockGasMeter() panics it will be caught by the above recover and will
	// return an error - in any case BlockGasMeter will consume gas past the limit.
	//
	// NOTE: consumeBlockGas must exist in a separate defer function from the
	// general deferred recovery function to recover from consumeBlockGas as it'll
	// be executed first (deferred statements are executed as stack).
	if isFinalized(mode) {
		defer consumeBlockGas()
	}

	tx, err := app.txDecoder(txBytes)
	if err != nil {
		return sdk.GasInfo{}, nil, nil, err
	}

	msgs := tx.GetMsgs()
	if err = validateBasicTxMsgs(msgs); err != nil {
		return sdk.GasInfo{}, nil, nil, err
	}

	for _, msg := range msgs {
		handler := app.msgServiceRouter.Handler(msg)
		if handler == nil {
			return sdk.GasInfo{}, nil, nil, errorsmod.Wrapf(sdkerrors.ErrUnknownRequest, "no message handler found for %T", msg)
		}
	}

	if app.anteHandler != nil {
		var anteCtx sdk.Context

		// Branch context before AnteHandler call in case it aborts.
		// This is required for both CheckTx and DeliverTx.
		// Ref: https://github.com/cosmos/cosmos-sdk/issues/2772
		//
		// NOTE: Alternatively, we could require that AnteHandler ensures that
		// writes do not happen if aborted/failed.  This may have some
		// performance benefits, but it'll be more difficult to get right.
		anteCtx, _ = app.cacheTxContext(ctx, txBytes)
		anteCtx = anteCtx.WithEventManager(sdk.NewEventManager())
		anteCtx = anteCtx.WithIsDeliverPreCheckTx(true)
		newCtx, err := app.anteHandler(anteCtx, tx, mode == execModeSimulate)

		if !newCtx.IsZero() {
			// At this point, newCtx.MultiStore() is a store branch, or something else
			// replaced by the AnteHandler. We want the original multistore.
			//
			// Also, in the case of the tx aborting, we need to track gas consumed via
			// the instantiated gas meter in the AnteHandler, so we update the context
			// prior to returning.
			ctx = newCtx.WithMultiStore(ms)
		}

		events := ctx.EventManager().Events()

		// GasMeter expected to be set in AnteHandler
		gasWanted = ctx.GasMeter().Limit()

		if err != nil {
			return gInfo, nil, nil, err
		}

		anteEvents = events.ToABCIEvents()
	}

	return gInfo, result, anteEvents, err
}

func (app *BaseApp) GetParamStore() ParamStore {
	return app.paramStore
}

// runTx processes a transaction within a given execution mode, encoded transaction
// bytes, and the decoded transaction itself. All state transitions occur through
// a cached Context depending on the mode provided. State only gets persisted
// if all messages get executed successfully and the execution mode is DeliverTx.
// Note, gas execution info is always returned. A reference to a Result is
// returned if the tx does not run out of gas and if all the messages are valid
// and execute successfully. An error is returned otherwise.
func (app *BaseApp) runTxWithContext(ctx sdk.Context, mode execMode, txBytes []byte) (gInfo sdk.GasInfo, result *sdk.Result, anteEvents []abci.Event, err error) {
	// NOTE: GasWanted should be returned by the AnteHandler. GasUsed is
	// determined by the GasMeter. We need access to the context to get the gas
	// meter, so we initialize upfront.
	var gasWanted uint64

	//ctx := app.getContextForTx(mode, txBytes)
	ms := ctx.MultiStore()

	// only run the tx if there is block gas remaining
	if isFinalized(mode) && ctx.BlockGasMeter().IsOutOfGas() {
		return gInfo, nil, nil, errorsmod.Wrap(sdkerrors.ErrOutOfGas, "no block gas left to run tx")
	}

	defer func() {
		if r := recover(); r != nil {
			recoveryMW := newOutOfGasRecoveryMiddleware(gasWanted, ctx, app.runTxRecoveryMiddleware)
			err, result = processRecovery(r, recoveryMW), nil
		}

		gInfo = sdk.GasInfo{GasWanted: gasWanted, GasUsed: ctx.GasMeter().GasConsumed()}
	}()

	blockGasConsumed := false

	// consumeBlockGas makes sure block gas is consumed at most once. It must
	// happen after tx processing, and must be executed even if tx processing
	// fails. Hence, it's execution is deferred.
	consumeBlockGas := func() {
		if !blockGasConsumed {
			blockGasConsumed = true
			ctx.BlockGasMeter().ConsumeGas(
				ctx.GasMeter().GasConsumedToLimit(), "block gas meter",
			)
		}
	}

	// If BlockGasMeter() panics it will be caught by the above recover and will
	// return an error - in any case BlockGasMeter will consume gas past the limit.
	//
	// NOTE: consumeBlockGas must exist in a separate defer function from the
	// general deferred recovery function to recover from consumeBlockGas as it'll
	// be executed first (deferred statements are executed as stack).
	if isFinalized(mode) {
		defer consumeBlockGas()
	}

	tx, err := app.txDecoder(txBytes)
	if err != nil {
		return sdk.GasInfo{}, nil, nil, err
	}

	msgs := tx.GetMsgs()
	if err := validateBasicTxMsgs(msgs); err != nil {
		return sdk.GasInfo{}, nil, nil, err
	}

	for _, msg := range msgs {
		handler := app.msgServiceRouter.Handler(msg)
		if handler == nil {
			return sdk.GasInfo{}, nil, nil, errorsmod.Wrapf(sdkerrors.ErrUnknownRequest, "no message handler found for %T", msg)
		}
	}

	if app.anteHandler != nil {
		var (
			anteCtx sdk.Context
			msCache storetypes.CacheMultiStore
		)

		// Branch context before AnteHandler call in case it aborts.
		// This is required for both CheckTx and DeliverTx.
		// Ref: https://github.com/cosmos/cosmos-sdk/issues/2772
		//
		// NOTE: Alternatively, we could require that AnteHandler ensures that
		// writes do not happen if aborted/failed.  This may have some
		// performance benefits, but it'll be more difficult to get right.
		anteCtx, msCache = app.cacheTxContext(ctx, txBytes)
		anteCtx = anteCtx.WithEventManager(sdk.NewEventManager())
		if isFinalized(mode) {
			anteCtx = anteCtx.WithIsDeliverPreCheckTx(false)
		}
		newCtx, err := app.anteHandler(anteCtx, tx, mode == execModeSimulate)

		if !newCtx.IsZero() {
			// At this point, newCtx.MultiStore() is a store branch, or something else
			// replaced by the AnteHandler. We want the original multistore.
			//
			// Also, in the case of the tx aborting, we need to track gas consumed via
			// the instantiated gas meter in the AnteHandler, so we update the context
			// prior to returning.
			ctx = newCtx.WithMultiStore(ms)
		}

		events := ctx.EventManager().Events()

		// GasMeter expected to be set in AnteHandler
		gasWanted = ctx.GasMeter().Limit()

		if err != nil {
			return gInfo, nil, nil, err
		}

		msCache.Write()
		anteEvents = events.ToABCIEvents()
	}

	if mode == execModeCheck {
		err = app.mempool.Insert(ctx, tx)
		if err != nil {
			return gInfo, nil, anteEvents, err
		}
	} else if isFinalized(mode) {
		err = app.mempool.Remove(tx)
		if err != nil && !errors.Is(err, mempool.ErrTxNotFound) {
			return gInfo, nil, anteEvents,
				fmt.Errorf("failed to remove tx from mempool: %w", err)
		}
	}

	// Create a new Context based off of the existing Context with a MultiStore branch
	// in case message processing fails. At this point, the MultiStore
	// is a branch of a branch.
	runMsgCtx, msCache := app.cacheTxContext(ctx, txBytes)

	// Attempt to execute all messages and only update state if all messages pass
	// and we're in DeliverTx. Note, runMsgs will never return a reference to a
	// Result if any single message fails or does not have a registered Handler.
	msgsV2, err := tx.GetMsgsV2()
	if err == nil {
		result, err = app.runMsgs(runMsgCtx, msgs, msgsV2, mode)
	}
	if err == nil {
		// Run optional postHandlers.
		//
		// Note: If the postHandler fails, we also revert the runMsgs state.
		if app.postHandler != nil {
			// The runMsgCtx context currently contains events emitted by the ante handler.
			// We clear this to correctly order events without duplicates.
			// Note that the state is still preserved.
			postCtx := runMsgCtx.WithEventManager(sdk.NewEventManager())

			newCtx, err := app.postHandler(postCtx, tx, mode == execModeSimulate, err == nil)
			if err != nil {
				return gInfo, nil, anteEvents, err
			}

			result.Events = append(result.Events, newCtx.EventManager().ABCIEvents()...)
		}

		if isFinalized(mode) {
			// When block gas exceeds, it'll panic and won't commit the cached store.
			consumeBlockGas()

			msCache.Write()
		}

		if len(anteEvents) > 0 && (isFinalized(mode) || mode == execModeSimulate) {
			// append the events in the order of occurrence
			result.Events = append(anteEvents, result.Events...)
		}
	}

	return gInfo, result, anteEvents, err
}

func (app *BaseApp) WithFinalizeBlockState(ms storetypes.CacheMultiStore) {
	app.finalizeBlockState.ms = ms
}
