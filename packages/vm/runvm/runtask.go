package runvm

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/ledgerstate/utxoutil"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/iscp/colored"
	"github.com/iotaledger/wasp/packages/iscp/request"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/kv/optimism"
	"github.com/iotaledger/wasp/packages/vm"
	"github.com/iotaledger/wasp/packages/vm/vmcontext"
	"golang.org/x/xerrors"
)

type VMRunner struct{}

func (r VMRunner) Run(task *vm.VMTask) {
	// panic catcher for the whole VM task
	// ir returns gracefully if the panic was about invalidated state
	// otherwise it panics again
	// The smart contract panics are processed inside and do not reach this point
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		if _, ok := r.(*optimism.ErrorStateInvalidated); ok {
			task.Log.Warnf("VM task has been abandoned due to invalidated state. ACS session id: %d", task.ACSSessionID)
			return
		}
		panic(r)
	}()
	runTask(task)
}

func NewVMRunner() VMRunner {
	return VMRunner{}
}

// runTask runs batch of requests on VM
func runTask(task *vm.VMTask) {
	task.Log.Debugw("runTask IN",
		"chainID", task.ChainInput.Address().Base58(),
		"timestamp", task.Timestamp,
		"block index", task.VirtualState.BlockIndex(),
		"num req", len(task.Requests),
	)
	if len(task.Requests) == 0 {
		task.Log.Panicf("MustRunVMTaskAsync: must be at least 1 request")
	}
	// TODO access and consensus pledge
	outputs := outputsFromRequests(task.Requests...)
	txb := utxoutil.NewBuilder(append(outputs, task.ChainInput)...)

	vmctx, err := vmcontext.CreateVMContext(task, txb)
	if err != nil {
		task.Log.Panicf("runTask: CreateVMContext: %v", err)
	}

	var lastResult dict.Dict
	var lastErr error
	var lastTotalAssets colored.Balances

	// loop over the batch of requests and run each request on the VM.
	// the result accumulates in the VMContext and in the list of stateUpdates
	var numOffLedger, numSuccess uint16
	for i, req := range task.Requests {
		vmctx.RunTheRequest(req, uint16(i))
		lastResult, lastTotalAssets, lastErr = vmctx.GetResult()

		if req.IsOffLedger() {
			numOffLedger++
		}
		if lastErr == nil {
			numSuccess++
		} else {
			task.Log.Debugf("runTask, ERROR running request: %s, error: %v", req.ID().Base58(), lastErr)
		}
	}
	task.Log.Debugf("runTask, ran %d requests. success: %d, offledger: %d", len(task.Requests), numSuccess, numOffLedger)

	// save the block info into the 'blocklog' contract
	// if rotationAddr != nil ir means the block is a rotation block
	rotationAddr := vmctx.CloseVMContext(uint16(len(task.Requests)), numSuccess, numOffLedger)

	if rotationAddr == nil {
		task.ResultTransactionEssence, err = vmctx.BuildTransactionEssence(task.VirtualState.Hash(), task.VirtualState.Timestamp())
		if err != nil {
			task.OnFinish(nil, nil, xerrors.Errorf("RunVM.BuildTransactionEssence: %w", err))
			return
		}
		if err = checkTotalAssets(task.ResultTransactionEssence, lastTotalAssets); err != nil {
			task.Log.Panic(xerrors.Errorf("RunVM.checkTotalAssets: %w", err))
		}
		task.Log.Debug("runTask OUT. ",
			"block index: ", task.VirtualState.BlockIndex(),
			" variable state hash: ", task.VirtualState.Hash().String(),
			" tx essence hash: ", hashing.HashData(task.ResultTransactionEssence.Bytes()).String(),
			" tx finalTimestamp: ", task.ResultTransactionEssence.Timestamp(),
		)
	} else {
		task.RotationAddress = rotationAddr
		task.ResultTransactionEssence = nil
		task.Log.Debugf("runTask OUT: rotate to address %s", rotationAddr.Base58())
	}
	task.OnFinish(lastResult, lastErr, nil)
}

// outputsFromRequests collect all outputs from requests which are on-ledger
func outputsFromRequests(requests ...iscp.Request) []ledgerstate.Output {
	ret := make([]ledgerstate.Output, 0)
	for _, req := range requests {
		if req.IsOffLedger() {
			continue
		}
		ret = append(ret, req.(*request.OnLedger).Output())
	}
	return ret
}

func checkTotalAssets(essence *ledgerstate.TransactionEssence, lastTotalOnChainAssets colored.Balances) error {
	var chainOutput *ledgerstate.AliasOutput
	for _, o := range essence.Outputs() {
		if out, ok := o.(*ledgerstate.AliasOutput); ok {
			chainOutput = out
		}
	}
	if chainOutput == nil {
		return xerrors.New("inconsistency: chain output not found")
	}
	balancesOnOutput := colored.BalancesFromL1Balances(chainOutput.Balances())
	diffAssets := balancesOnOutput.Diff(lastTotalOnChainAssets)
	// we expect assets in the chain output and total assets on-chain differs only in the amount of
	// ant-dust tokens locked in the output. Otherwise it is inconsistency
	if len(diffAssets) != 1 || diffAssets[colored.IOTA] != int64(ledgerstate.DustThresholdAliasOutputIOTA) {
		return xerrors.Errorf("inconsistency between L1 and L2 ledgers. Diff: %+v", diffAssets)
	}
	return nil
}
