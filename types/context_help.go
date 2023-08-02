package types

func (c Context) IsPreCheckTx() bool { return c.enablePreCheckTx && c.preCheckTx }

func (c Context) WithIsDeliverPreCheckTx(isPreCheckTx bool) Context {
	c.preCheckTx = isPreCheckTx
	c.enablePreCheckTx = true
	return c
}

func NeedVerify(ctx Context, simulate bool) bool {
	if !ctx.enablePreCheckTx {
		return true
	}
	return ctx.IsCheckTx() || ctx.IsReCheckTx() || ctx.IsPreCheckTx() || simulate
}
