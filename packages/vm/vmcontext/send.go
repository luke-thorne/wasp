package vmcontext

import (
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/transaction"
)

// Send implements sandbox function of sending cross-chain request
func (vmctx *VMContext) Send(par iscp.RequestParameters) {
	if vmctx.NumPostedOutputs >= MaxPostedOutputsInOneRequest {
		panic(ErrExceededPostedOutputLimit)
	}

	vmctx.NumPostedOutputs++

	assets := par.Assets
	// create extended output with adjusted dust deposit
	out := transaction.BasicOutputFromPostData(
		vmctx.task.AnchorOutput.AliasID.ToAddress(),
		vmctx.CurrentContractHname(),
		par,
		vmctx.task.RentStructure,
	)
	if out.Amount > par.Assets.Iotas {
		// it was adjusted
		assets = assets.Clone()
		assets.Iotas = out.Amount
	}
	vmctx.assertConsistentL2WithL1TxBuilder("sandbox.Send: begin")
	// this call cannot panic due to not enough iotas for dust because
	// it does not change total balance of the transaction, and it does not create new internal outputs
	// The call can destroy internal output when all native tokens of particular ID are moved outside chain
	// The caller will receive all the dust deposit
	iotaAdjustmentL2 := vmctx.txbuilder.AddOutput(out)
	vmctx.adjustL2IotasIfNeeded(iotaAdjustmentL2, vmctx.AccountID())
	// debit the assets from the on-chain account
	// It panics with accounts.ErrNotEnoughFunds if sender's account balances are exceeded
	vmctx.debitFromAccount(vmctx.AccountID(), assets)
	vmctx.assertConsistentL2WithL1TxBuilder("sandbox.Send: end")
}
