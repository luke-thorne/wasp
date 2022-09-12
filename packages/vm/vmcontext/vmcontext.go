package vmcontext

import (
	"fmt"
	"time"

	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/trie.go/trie"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/coreutil"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/vm"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/core/blob"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/core/root"
	"github.com/iotaledger/wasp/packages/vm/execution"
	"github.com/iotaledger/wasp/packages/vm/gas"
	"github.com/iotaledger/wasp/packages/vm/processors"
	"github.com/iotaledger/wasp/packages/vm/vmcontext/vmtxbuilder"
)

// VMContext represents state of the chain during one run of the VM while processing
// a batch of requests. VMContext object mutates with each request in the bathc.
// The VMContext is created from immutable vm.VMTask object and UTXO state of the
// chain address contained in the statetxbuilder.Builder
type VMContext struct {
	task *vm.VMTask
	// same for the block
	chainOwnerID              isc.AgentID
	virtualState              state.VirtualStateAccess
	finalStateTimestamp       time.Time
	blockContext              map[isc.Hname]interface{}
	storageDepositAssumptions *transaction.StorageDepositAssumption
	txbuilder                 *vmtxbuilder.AnchorTransactionBuilder
	txsnapshot                *vmtxbuilder.AnchorTransactionBuilder
	gasBurnedTotal            uint64
	gasFeeChargedTotal        uint64

	// ---- request context
	chainInfo          *governance.ChainInfo
	req                isc.Request
	NumPostedOutputs   int // how many outputs has been posted in the request
	requestIndex       uint16
	requestEventIndex  uint16
	currentStateUpdate state.Update
	entropy            hashing.HashValue
	callStack          []*callContext
	// --- gas related
	// max tokens cane be charged for gas fee
	gasMaxTokensToSpendForGasFee uint64
	// final gas budget set for the run
	gasBudgetAdjusted uint64
	// is gas bur enabled
	gasBurnEnabled bool
	// gas already burned
	gasBurned uint64
	// tokens charged
	gasFeeCharged uint64
	// burn history. If disabled, it is nil
	gasBurnLog *gas.BurnLog
}

var _ execution.WaspContext = &VMContext{}

type callContext struct {
	caller             isc.AgentID    // calling agent
	contract           isc.Hname      // called contract
	params             isc.Params     // params passed
	allowanceAvailable *isc.Allowance // MUTABLE: allowance budget left after TransferAllowedFunds
}

// CreateVMContext creates a context for the whole batch run
func CreateVMContext(task *vm.VMTask) *VMContext {
	// assert consistency. It is a bit redundant double check
	if len(task.Requests) == 0 {
		// should never happen
		panic(fmt.Errorf("CreateVMContext.invalid params: must be at least 1 request"))
	}
	l1Commitment, err := state.L1CommitmentFromBytes(task.AnchorOutput.StateMetadata)
	if err != nil {
		// should never happen
		panic(fmt.Errorf("CreateVMContext: can't parse state data as L1Commitment from chain input %w", err))
	}
	// we create optimistic state access wrapper to be used inside the VM call.
	// It will panic any time the state is accessed.
	// The panic will be caught above and VM call will be abandoned peacefully
	optimisticStateAccess := state.WrapMustOptimisticVirtualStateAccess(task.VirtualStateAccess, task.SolidStateBaseline)

	// assert consistency
	commitmentFromState := state.RootCommitment(optimisticStateAccess.TrieNodeStore())
	blockIndex := optimisticStateAccess.BlockIndex()
	if !state.EqualCommitments(l1Commitment.StateCommitment, commitmentFromState) || blockIndex != task.AnchorOutput.StateIndex {
		// leaving earlier, state is not consistent and optimistic reader sync didn't catch it
		panic(coreutil.ErrorStateInvalidated)
	}
	openingStateUpdate := state.NewStateUpdateWithBlockLogValues(blockIndex+1, task.TimeAssumption, &l1Commitment)
	optimisticStateAccess.ApplyStateUpdate(openingStateUpdate)
	finalStateTimestamp := task.TimeAssumption.Add(time.Duration(len(task.Requests)+1) * time.Nanosecond)

	ret := &VMContext{
		task:                task,
		virtualState:        optimisticStateAccess,
		finalStateTimestamp: finalStateTimestamp,
		blockContext:        make(map[isc.Hname]interface{}),
		entropy:             task.Entropy,
		callStack:           make([]*callContext, 0),
	}
	if task.EnableGasBurnLogging {
		ret.gasBurnLog = gas.NewGasBurnLog()
	}
	// at the beginning of each block

	if task.AnchorOutput.StateIndex > 0 {
		ret.currentStateUpdate = state.NewStateUpdate()

		// load and validate chain's storage deposit assumptions about internal outputs. They must not get bigger!
		ret.callCore(accounts.Contract, func(s kv.KVStore) {
			ret.storageDepositAssumptions = accounts.GetStorageDepositAssumptions(s)
		})
		currentStorageDepositValues := transaction.NewStorageDepositEstimate()
		if currentStorageDepositValues.AnchorOutput > ret.storageDepositAssumptions.AnchorOutput ||
			currentStorageDepositValues.NativeTokenOutput > ret.storageDepositAssumptions.NativeTokenOutput {
			panic(vm.ErrInconsistentStorageDepositAssumptions)
		}

		// save the anchor tx ID of the current state
		ret.callCore(blocklog.Contract, func(s kv.KVStore) {
			blocklog.UpdateLatestBlockInfo(s, ret.task.AnchorOutputID.TransactionID(), &l1Commitment)
		})

		ret.virtualState.ApplyStateUpdate(ret.currentStateUpdate)
		ret.currentStateUpdate = nil
	} else {
		// assuming storage deposit assumptions for the first block. It must be consistent with parameters in the init request
		ret.storageDepositAssumptions = transaction.NewStorageDepositEstimate()
	}

	nativeTokenBalanceLoader := func(id *iotago.NativeTokenID) (*iotago.BasicOutput, *iotago.UTXOInput) {
		return ret.loadNativeTokenOutput(id)
	}
	foundryLoader := func(serNum uint32) (*iotago.FoundryOutput, *iotago.UTXOInput) {
		return ret.loadFoundry(serNum)
	}
	nftLoader := func(id iotago.NFTID) (*iotago.NFTOutput, *iotago.UTXOInput) {
		return ret.loadNFT(id)
	}
	ret.txbuilder = vmtxbuilder.NewAnchorTransactionBuilder(
		task.AnchorOutput,
		task.AnchorOutputID,
		nativeTokenBalanceLoader,
		foundryLoader,
		nftLoader,
		*ret.storageDepositAssumptions,
	)

	return ret
}

// CloseVMContext does the closing actions on the block
// return nil for normal block and rotation address for rotation block
func (vmctx *VMContext) CloseVMContext(numRequests, numSuccess, numOffLedger uint16) (uint32, *state.L1Commitment, time.Time, iotago.Address) {
	vmctx.GasBurnEnable(false)
	vmctx.currentStateUpdate = state.NewStateUpdate() // need this before to make state valid
	rotationAddr := vmctx.saveBlockInfo(numRequests, numSuccess, numOffLedger)
	if vmctx.task.AnchorOutput.StateIndex > 0 {
		vmctx.closeBlockContexts()
	}
	vmctx.saveInternalUTXOs()
	vmctx.virtualState.ApplyStateUpdate(vmctx.currentStateUpdate)
	vmctx.virtualState.Commit()

	block, err := vmctx.virtualState.ExtractBlock()
	if err != nil {
		panic(err)
	}

	stateCommitment := trie.RootCommitment(vmctx.virtualState.TrieNodeStore())
	blockHash := state.BlockHashFromData(block.EssenceBytes())
	l1Commitment := state.NewL1Commitment(stateCommitment, blockHash)

	blockIndex := vmctx.virtualState.BlockIndex()
	timestamp := vmctx.virtualState.Timestamp()

	return blockIndex, l1Commitment, timestamp, rotationAddr
}

func (vmctx *VMContext) checkRotationAddress() (ret iotago.Address) {
	vmctx.callCore(governance.Contract, func(s kv.KVStore) {
		ret = governance.GetRotationAddress(s)
	})
	return
}

// saveBlockInfo is in the blocklog partition context. Returns rotation address if this block is a rotation block
func (vmctx *VMContext) saveBlockInfo(numRequests, numSuccess, numOffLedger uint16) iotago.Address {
	if rotationAddress := vmctx.checkRotationAddress(); rotationAddress != nil {
		// block was marked fake by the governance contract because it is a committee rotation.
		// There was only on request in the block
		// We skip saving block information in order to avoid inconsistencies
		return rotationAddress
	}
	// block info will be stored into the separate state update
	prevL1Commitment, err := state.L1CommitmentFromBytes(vmctx.task.AnchorOutput.StateMetadata)
	if err != nil {
		panic(err)
	}
	// sub essence hash is known without L1 commitment. It is needed for fraud proofs
	subEssenceHash := vmctx.CalcTransactionSubEssenceHash()
	totalBaseTokensInContracts, totalStorageDepositOnChain := vmctx.txbuilder.TotalBaseTokensInOutputs()
	blockInfo := &blocklog.BlockInfo{
		BlockIndex:                  vmctx.virtualState.BlockIndex(),
		Timestamp:                   vmctx.virtualState.Timestamp(),
		TotalRequests:               numRequests,
		NumSuccessfulRequests:       numSuccess,
		NumOffLedgerRequests:        numOffLedger,
		PreviousL1Commitment:        prevL1Commitment,
		L1Commitment:                nil,                    // current L1Commitment not known at this point
		AnchorTransactionID:         iotago.TransactionID{}, // nil for now, will be updated the next round with the real tx id
		TransactionSubEssenceHash:   subEssenceHash,
		TotalBaseTokensInL2Accounts: totalBaseTokensInContracts,
		TotalStorageDeposit:         totalStorageDepositOnChain,
		GasBurned:                   vmctx.gasBurnedTotal,
		GasFeeCharged:               vmctx.gasFeeChargedTotal,
	}
	if !state.EqualCommitments(vmctx.virtualState.PreviousL1Commitment().StateCommitment, blockInfo.PreviousL1Commitment.StateCommitment) {
		panic("CloseVMContext: inconsistent previous state commitment")
	}

	vmctx.callCore(blocklog.Contract, func(s kv.KVStore) {
		blocklog.SaveNextBlockInfo(s, blockInfo)
		blocklog.SaveControlAddressesIfNecessary(
			s,
			vmctx.task.AnchorOutput.StateController(),
			vmctx.task.AnchorOutput.GovernorAddress(),
			vmctx.task.AnchorOutput.StateIndex,
		)
	})
	return nil
}

// OpenBlockContexts calls the block context open function for all subscribed core contracts
func (vmctx *VMContext) OpenBlockContexts() {
	if vmctx.gasBurnEnabled {
		panic("expected gasBurnEnabled == false")
	}

	vmctx.currentStateUpdate = state.NewStateUpdate()
	vmctx.loadChainConfig()

	var subs []root.BlockContextSubscription
	vmctx.callCore(root.Contract, func(s kv.KVStore) {
		subs = root.GetBlockContextSubscriptions(s)
	})
	for _, sub := range subs {
		vmctx.callProgram(sub.Contract, sub.OpenFunc, nil, nil)
	}

	vmctx.virtualState.ApplyStateUpdate(vmctx.currentStateUpdate)
}

// closeBlockContexts closes block contexts in deterministic FIFO sequence
func (vmctx *VMContext) closeBlockContexts() {
	if vmctx.gasBurnEnabled {
		panic("expected gasBurnEnabled == false")
	}
	var subs []root.BlockContextSubscription
	vmctx.callCore(root.Contract, func(s kv.KVStore) {
		subs = root.GetBlockContextSubscriptions(s)
	})
	for i := len(subs) - 1; i >= 0; i-- {
		vmctx.callProgram(subs[i].Contract, subs[i].CloseFunc, nil, nil)
	}
}

// saveInternalUTXOs relies on the order of the outputs in the anchor tx. If that order changes, this will be broken.
// Anchor Transaction outputs order must be:
// 1. NativeTokens
// 2. Foundries
// 3. NFTs
func (vmctx *VMContext) saveInternalUTXOs() {
	nativeTokenIDs, nativeTokensToBeRemoved := vmctx.txbuilder.NativeTokenRecordsToBeUpdated()
	nativeTokensOutputsToBeUpdated := vmctx.txbuilder.NativeTokenOutputsByTokenIDs(nativeTokenIDs)

	foundryIDs, foundriesToBeRemoved := vmctx.txbuilder.FoundriesToBeUpdated()
	foundrySNToBeUpdated := vmctx.txbuilder.FoundryOutputsBySN(foundryIDs)

	NFTOutputsToBeAdded, NFTOutputsToBeRemoved := vmctx.txbuilder.NFTOutputsToBeUpdated()

	blockIndex := vmctx.task.AnchorOutput.StateIndex + 1
	outputIndex := uint16(1)

	vmctx.callCore(accounts.Contract, func(s kv.KVStore) {
		// update native token outputs
		for _, out := range nativeTokensOutputsToBeUpdated {
			accounts.SaveNativeTokenOutput(s, out, blockIndex, outputIndex)
			outputIndex++
		}
		for _, id := range nativeTokensToBeRemoved {
			accounts.DeleteNativeTokenOutput(s, id)
		}

		// update foundry UTXOs
		for _, out := range foundrySNToBeUpdated {
			accounts.SaveFoundryOutput(s, out, blockIndex, outputIndex)
			outputIndex++
		}
		for _, sn := range foundriesToBeRemoved {
			accounts.DeleteFoundryOutput(s, sn)
		}

		// update NFT Outputs
		for _, out := range NFTOutputsToBeAdded {
			accounts.SaveNFTOutput(s, out, blockIndex, outputIndex)
			outputIndex++
		}
		for _, out := range NFTOutputsToBeRemoved {
			accounts.DeleteNFTOutput(s, out.NFTID)
		}
	})
}

func (vmctx *VMContext) assertConsistentL2WithL1TxBuilder(checkpoint string) {
	if vmctx.task.AnchorOutput.StateIndex == 0 && vmctx.isInitChainRequest() {
		return
	}
	var totalL2Assets *isc.FungibleTokens
	vmctx.callCore(accounts.Contract, func(s kv.KVStore) {
		totalL2Assets = accounts.GetTotalL2Assets(s)
	})
	vmctx.txbuilder.AssertConsistentWithL2Totals(totalL2Assets, checkpoint)
}

func (vmctx *VMContext) AssertConsistentGasTotals() {
	var sumGasBurned, sumGasFeeCharged uint64

	for _, r := range vmctx.task.Results {
		sumGasBurned += r.Receipt.GasBurned
		sumGasFeeCharged += r.Receipt.GasFeeCharged
	}
	if vmctx.gasBurnedTotal != sumGasBurned {
		panic("vmctx.gasBurnedTotal != sumGasBurned")
	}
	if vmctx.gasFeeChargedTotal != sumGasFeeCharged {
		panic("vmctx.gasFeeChargedTotal != sumGasFeeCharged")
	}
}

func (vmctx *VMContext) LocateProgram(programHash hashing.HashValue) (vmtype string, binary []byte, err error) {
	vmctx.callCore(blob.Contract, func(s kv.KVStore) {
		vmtype, binary, err = blob.LocateProgram(vmctx.State(), programHash)
	})
	return vmtype, binary, err
}

func (vmctx *VMContext) Processors() *processors.Cache {
	return vmctx.task.Processors
}
