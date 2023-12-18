package types

import sdk "github.com/cosmos/cosmos-sdk/types"

// bank module event types
const (
	EventTypeTransfer = "transfer"

	AttributeKeyRecipient = "recipient"
	AttributeKeySender    = "sender"

	EventTypeCoinSpent    = "coin_spent"
	EventTypeCoinReceived = "coin_received"
	EventTypeCoinMint     = "coinbase"
	EventTypeCoinBurn     = "burn"

	AttributeKeySpender  = "spender"
	AttributeKeyReceiver = "receiver"
	AttributeKeyMinter   = "minter"
	AttributeKeyBurner   = "burner"

	AttributeValueCategory = ModuleName
)

func NewCoinSpentEvent(spender sdk.AccAddress, amount sdk.Coins) sdk.Event {
	return sdk.NewEvent(
		EventTypeCoinSpent,
		sdk.NewAttribute(AttributeKeySpender, spender.String()),
		sdk.NewAttribute(sdk.AttributeKeyAmount, amount.String()),
	)
}

func NewCoinReceivedEvent(receiver sdk.AccAddress, amount sdk.Coins) sdk.Event {
	return sdk.NewEvent(
		EventTypeCoinReceived,
		sdk.NewAttribute(AttributeKeyReceiver, receiver.String()),
		sdk.NewAttribute(sdk.AttributeKeyAmount, amount.String()),
	)
}

func NewCoinMintEvent(minter sdk.AccAddress, amount sdk.Coins) sdk.Event {
	return sdk.NewEvent(
		EventTypeCoinMint,
		sdk.NewAttribute(AttributeKeyMinter, minter.String()),
		sdk.NewAttribute(sdk.AttributeKeyAmount, amount.String()),
	)
}

func NewCoinBurnEvent(burner sdk.AccAddress, amount sdk.Coins) sdk.Event {
	return sdk.NewEvent(
		EventTypeCoinBurn,
		sdk.NewAttribute(AttributeKeyBurner, burner.String()),
		sdk.NewAttribute(sdk.AttributeKeyAmount, amount.String()),
	)
}
