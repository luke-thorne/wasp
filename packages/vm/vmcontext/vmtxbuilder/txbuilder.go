package vmtxbuilder

import (
	"encoding/hex"
	"fmt"
	"math/big"

	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/vm"
	"github.com/iotaledger/wasp/packages/vm/vmcontext/vmexceptions"
	"golang.org/x/xerrors"
)

// tokenOutputLoader externally supplied function which loads stored output from the state
// Should return nil if does not exist
type tokenOutputLoader func(*iotago.NativeTokenID) (*iotago.BasicOutput, *iotago.UTXOInput)

// foundryLoader externally supplied function which returns foundry output and id by its serial number
// Should return nil if foundry does not exist
type foundryLoader func(uint32) (*iotago.FoundryOutput, *iotago.UTXOInput)

// NFTOutputLoader externally supplied function which returns the stored NFT output from the state
// Should return nil if NFT is not accounted for
type NFTOutputLoader func(id iotago.NFTID) (*iotago.NFTOutput, *iotago.UTXOInput)

// AnchorTransactionBuilder represents structure which handles all the data needed to eventually
// build an essence of the anchor transaction
type AnchorTransactionBuilder struct {
	// anchorOutput output of the chain
	anchorOutput *iotago.AliasOutput
	// anchorOutputID is the ID of the anchor output
	anchorOutputID iotago.OutputID
	// already consumed outputs, specified by entire Request. It is needed for checking validity
	consumed []isc.OnLedgerRequest
	// base tokens which are on-chain. It does not include storage deposits on anchor and on internal outputs
	totalBaseTokensInL2Accounts uint64
	// minimum storage deposit assumption for internal outputs. It is used as constants. Assumed real storage deposit cost never grows
	storageDepositAssumption transaction.StorageDepositAssumption
	// balance loader for native tokens
	loadTokenOutput tokenOutputLoader
	// foundry loader
	loadFoundry foundryLoader
	// NFToutput loader
	loadNFTOutput NFTOutputLoader
	// balances of native tokens loaded during the batch run
	balanceNativeTokens map[iotago.NativeTokenID]*nativeTokenBalance
	// all nfts loaded during the batch run
	nftsIncluded map[iotago.NFTID]*nftIncluded
	// invoked foundries. Foundry serial number is used as a key
	invokedFoundries map[uint32]*foundryInvoked
	// requests posted by smart contracts
	postedOutputs []iotago.Output
}

// NewAnchorTransactionBuilder creates new AnchorTransactionBuilder object
func NewAnchorTransactionBuilder(
	anchorOutput *iotago.AliasOutput,
	anchorOutputID iotago.OutputID,
	tokenBalanceLoader tokenOutputLoader,
	foundryLoader foundryLoader,
	nftLoader NFTOutputLoader,
	storageDepositAssumptions transaction.StorageDepositAssumption,
) *AnchorTransactionBuilder {
	if anchorOutput.Amount < storageDepositAssumptions.AnchorOutput {
		panic("internal inconsistency")
	}
	return &AnchorTransactionBuilder{
		anchorOutput:                anchorOutput,
		anchorOutputID:              anchorOutputID,
		totalBaseTokensInL2Accounts: anchorOutput.Amount - storageDepositAssumptions.AnchorOutput,
		storageDepositAssumption:    storageDepositAssumptions,
		loadTokenOutput:             tokenBalanceLoader,
		loadFoundry:                 foundryLoader,
		loadNFTOutput:               nftLoader,
		consumed:                    make([]isc.OnLedgerRequest, 0, iotago.MaxInputsCount-1),
		balanceNativeTokens:         make(map[iotago.NativeTokenID]*nativeTokenBalance),
		postedOutputs:               make([]iotago.Output, 0, iotago.MaxOutputsCount-1),
		invokedFoundries:            make(map[uint32]*foundryInvoked),
		nftsIncluded:                make(map[iotago.NFTID]*nftIncluded),
	}
}

// Clone clones the AnchorTransactionBuilder object. Used to snapshot/recover
func (txb *AnchorTransactionBuilder) Clone() *AnchorTransactionBuilder {
	ret := &AnchorTransactionBuilder{
		anchorOutput:                txb.anchorOutput,
		anchorOutputID:              txb.anchorOutputID,
		totalBaseTokensInL2Accounts: txb.totalBaseTokensInL2Accounts,
		storageDepositAssumption:    txb.storageDepositAssumption,
		loadTokenOutput:             txb.loadTokenOutput,
		loadFoundry:                 txb.loadFoundry,
		consumed:                    make([]isc.OnLedgerRequest, 0, cap(txb.consumed)),
		balanceNativeTokens:         make(map[iotago.NativeTokenID]*nativeTokenBalance),
		postedOutputs:               make([]iotago.Output, 0, cap(txb.postedOutputs)),
		invokedFoundries:            make(map[uint32]*foundryInvoked),
		nftsIncluded:                make(map[iotago.NFTID]*nftIncluded),
	}

	ret.consumed = append(ret.consumed, txb.consumed...)
	for k, v := range txb.balanceNativeTokens {
		ret.balanceNativeTokens[k] = v.clone()
	}
	for k, v := range txb.invokedFoundries {
		ret.invokedFoundries[k] = v.clone()
	}
	for k, v := range txb.nftsIncluded {
		ret.nftsIncluded[k] = v.clone()
	}
	ret.postedOutputs = append(ret.postedOutputs, txb.postedOutputs...)
	return ret
}

// TotalBaseTokensInL2Accounts returns number of on-chain base tokens.
// It does not include minimum storage deposit needed for anchor output and other internal UTXOs
func (txb *AnchorTransactionBuilder) TotalBaseTokensInL2Accounts() uint64 {
	return txb.totalBaseTokensInL2Accounts
}

// Consume adds an input to the transaction.
// It panics if transaction cannot hold that many inputs
// All explicitly consumed inputs will hold fixed index in the transaction
// It updates total assets held by the chain. So it may panic due to exceed output counts
// Returns delta of base tokens needed to adjust the common account due to storage deposit requirement for internal UTXOs
// NOTE: if call panics with ErrNotEnoughFundsForInternalStorageDeposit, the state of the builder becomes inconsistent
// It means, in the caller context it should be rolled back altogether
func (txb *AnchorTransactionBuilder) Consume(req isc.OnLedgerRequest) int64 {
	if DebugTxBuilder {
		txb.MustBalanced("txbuilder.Consume IN")
	}
	if txb.InputsAreFull() {
		panic(vmexceptions.ErrInputLimitExceeded)
	}

	defer txb.mustCheckTotalNativeTokensExceeded()

	txb.consumed = append(txb.consumed, req)

	// first we add all base tokens arrived with the output to anchor balance
	txb.addDeltaBaseTokensToTotal(req.Output().Deposit())
	// then we add all arriving native tokens to corresponding internal outputs
	deltaBaseTokensStorageDepositAdjustment := int64(0)
	for _, nt := range req.FungibleTokens().Tokens {
		deltaBaseTokensStorageDepositAdjustment += txb.addNativeTokenBalanceDelta(&nt.ID, nt.Amount)
	}
	if req.NFT() != nil {
		deltaBaseTokensStorageDepositAdjustment += txb.consumeNFT(req.Output().(*iotago.NFTOutput), req.UTXOInput())
	}
	if DebugTxBuilder {
		txb.MustBalanced("txbuilder.Consume OUT")
	}
	return deltaBaseTokensStorageDepositAdjustment
}

// AddOutput adds an information about posted request. It will produce output
// Return adjustment needed for the L2 ledger (adjustment on base tokens related to storage deposit)
func (txb *AnchorTransactionBuilder) AddOutput(o iotago.Output) int64 {
	if txb.outputsAreFull() {
		panic(vmexceptions.ErrOutputLimitExceeded)
	}

	defer txb.mustCheckTotalNativeTokensExceeded()

	storageDeposit := parameters.L1().Protocol.RentStructure.MinRent(o)
	if o.Deposit() < storageDeposit {
		panic(xerrors.Errorf("%v: available %d < required %d base tokens",
			transaction.ErrNotEnoughBaseTokensForStorageDeposit, o.Deposit(), storageDeposit))
	}
	assets := transaction.AssetsFromOutput(o)
	txb.subDeltaBaseTokensFromTotal(assets.BaseTokens)
	bi := new(big.Int)
	baseTokensAdjustmentL2 := int64(0)
	for _, nt := range assets.Tokens {
		bi.Neg(nt.Amount)
		baseTokensAdjustmentL2 += txb.addNativeTokenBalanceDelta(&nt.ID, bi)
	}
	if nftout, ok := o.(*iotago.NFTOutput); ok {
		baseTokensAdjustmentL2 += txb.sendNFT(nftout)
	}
	txb.postedOutputs = append(txb.postedOutputs, o)
	return baseTokensAdjustmentL2
}

// InputsAreFull returns if transaction cannot hold more inputs
func (txb *AnchorTransactionBuilder) InputsAreFull() bool {
	return txb.numInputs() >= iotago.MaxInputsCount
}

// BuildTransactionEssence builds transaction essence from tx builder data
func (txb *AnchorTransactionBuilder) BuildTransactionEssence(l1Commitment *state.L1Commitment) (*iotago.TransactionEssence, []byte) {
	txb.MustBalanced("BuildTransactionEssence IN")
	inputs, inputIDs := txb.inputs()
	essence := &iotago.TransactionEssence{
		NetworkID: parameters.L1().Protocol.NetworkID(),
		Inputs:    inputIDs.UTXOInputs(),
		Outputs:   txb.outputs(l1Commitment),
		Payload:   nil,
	}

	inputsCommitment := inputIDs.OrderedSet(inputs).MustCommitment()
	copy(essence.InputsCommitment[:], inputsCommitment)

	return essence, inputsCommitment
}

// inputIDs generates a deterministic list of inputs for the transaction essence
// - index 0 is always alias output
// - then goes consumed external BasicOutput UTXOs, the requests, in the order of processing
// - then goes inputs of native token UTXOs, sorted by token id
// - then goes inputs of foundries sorted by serial number
func (txb *AnchorTransactionBuilder) inputs() (iotago.OutputSet, iotago.OutputIDs) {
	ids := make(iotago.OutputIDs, 0, len(txb.consumed)+len(txb.balanceNativeTokens)+len(txb.invokedFoundries))
	inputs := make(iotago.OutputSet)

	// alias output
	ids = append(ids, txb.anchorOutputID)
	inputs[txb.anchorOutputID] = txb.anchorOutput

	// consumed on-ledger requests
	for i := range txb.consumed {
		id := txb.consumed[i].ID().OutputID()
		ids = append(ids, id)
		inputs[id] = txb.consumed[i].Output()
	}

	// internal native token outputs
	for _, nt := range txb.nativeTokenOutputsSorted() {
		if nt.requiresInput() {
			id := nt.input.ID()
			ids = append(ids, id)
			inputs[id] = nt.in
		}
	}

	// foundries
	for _, f := range txb.foundriesSorted() {
		if f.requiresInput() {
			id := f.input.ID()
			ids = append(ids, id)
			inputs[id] = f.in
		}
	}

	// nfts
	for _, nft := range txb.nftsSorted() {
		if nft.input != nil {
			id := nft.input.ID()
			ids = append(ids, id)
			inputs[id] = nft.in
		}
	}

	if len(ids) != txb.numInputs() {
		panic(fmt.Sprintf("AnchorTransactionBuilder.inputs: internal inconsistency. expected: %d actual:%d", len(ids), txb.numInputs()))
	}
	return inputs, ids
}

// outputs generates outputs for the transaction essence
func (txb *AnchorTransactionBuilder) outputs(l1Commitment *state.L1Commitment) iotago.Outputs {
	ret := make(iotago.Outputs, 0, 1+len(txb.balanceNativeTokens)+len(txb.postedOutputs))
	// creating the anchor output
	aliasID := txb.anchorOutput.AliasID
	if aliasID.Empty() {
		aliasID = iotago.AliasIDFromOutputID(txb.anchorOutputID)
	}
	anchorOutput := &iotago.AliasOutput{
		Amount:         txb.totalBaseTokensInL2Accounts + txb.storageDepositAssumption.AnchorOutput,
		NativeTokens:   nil, // anchor output does not contain native tokens
		AliasID:        aliasID,
		StateIndex:     txb.anchorOutput.StateIndex + 1,
		StateMetadata:  l1Commitment.Bytes(),
		FoundryCounter: txb.nextFoundryCounter(),
		Conditions: iotago.UnlockConditions{
			&iotago.StateControllerAddressUnlockCondition{Address: txb.anchorOutput.StateController()},
			&iotago.GovernorAddressUnlockCondition{Address: txb.anchorOutput.GovernorAddress()},
		},
		Features: iotago.Features{
			&iotago.SenderFeature{
				Address: aliasID.ToAddress(),
			},
		},
	}
	ret = append(ret, anchorOutput)

	// creating outputs for updated internal accounts
	nativeTokensToBeUpdated, _ := txb.NativeTokenRecordsToBeUpdated()
	for _, id := range nativeTokensToBeUpdated {
		// create one output for each token ID of internal account
		ret = append(ret, txb.balanceNativeTokens[id].out)
	}
	// creating outputs for updated foundries
	foundriesToBeUpdated, _ := txb.FoundriesToBeUpdated()
	for _, sn := range foundriesToBeUpdated {
		ret = append(ret, txb.invokedFoundries[sn].out)
	}
	// creating outputs for new NFTs
	nftOuts := txb.NFTOutputs()
	for _, nftOut := range nftOuts {
		ret = append(ret, nftOut)
	}
	// creating outputs for posted on-ledger requests
	ret = append(ret, txb.postedOutputs...)
	return ret
}

// numInputs number of inputs in the future transaction
func (txb *AnchorTransactionBuilder) numInputs() int {
	ret := len(txb.consumed) + 1 // + 1 for anchor UTXO
	for _, v := range txb.balanceNativeTokens {
		if v.requiresInput() {
			ret++
		}
	}
	for _, f := range txb.invokedFoundries {
		if f.requiresInput() {
			ret++
		}
	}
	for _, nft := range txb.nftsIncluded {
		if nft.input != nil {
			ret++
		}
	}
	return ret
}

// numOutputs in the transaction
func (txb *AnchorTransactionBuilder) numOutputs() int {
	ret := 1 // for chain output
	for _, v := range txb.balanceNativeTokens {
		if v.producesOutput() {
			ret++
		}
	}
	ret += len(txb.postedOutputs)
	for _, f := range txb.invokedFoundries {
		if f.producesOutput() {
			ret++
		}
	}
	return ret
}

// outputsAreFull return if transaction cannot bear more outputs
func (txb *AnchorTransactionBuilder) outputsAreFull() bool {
	return txb.numOutputs() >= iotago.MaxOutputsCount
}

func (txb *AnchorTransactionBuilder) mustCheckTotalNativeTokensExceeded() {
	num := 0
	for _, nt := range txb.balanceNativeTokens {
		if nt.requiresInput() || nt.producesOutput() {
			num++
		}
		if num > iotago.MaxNativeTokensCount {
			panic(vmexceptions.ErrTotalNativeTokensLimitExceeded)
		}
	}
}

// addDeltaBaseTokensToTotal increases number of on-chain main account base tokens by delta
func (txb *AnchorTransactionBuilder) addDeltaBaseTokensToTotal(delta uint64) {
	if delta == 0 {
		return
	}
	// safe arithmetics
	n := txb.totalBaseTokensInL2Accounts + delta
	if n+txb.storageDepositAssumption.AnchorOutput < txb.totalBaseTokensInL2Accounts {
		panic(xerrors.Errorf("addDeltaBaseTokensToTotal: %w", vm.ErrOverflow))
	}
	txb.totalBaseTokensInL2Accounts = n
}

// subDeltaBaseTokensFromTotal decreases number of on-chain main account base tokens
func (txb *AnchorTransactionBuilder) subDeltaBaseTokensFromTotal(delta uint64) {
	if delta == 0 {
		return
	}
	// safe arithmetics
	if delta > txb.totalBaseTokensInL2Accounts {
		panic(vm.ErrNotEnoughBaseTokensBalance)
	}
	txb.totalBaseTokensInL2Accounts -= delta
}

func stringUTXOInput(inp *iotago.UTXOInput) string {
	return fmt.Sprintf("[%d]%s", inp.TransactionOutputIndex, hex.EncodeToString(inp.TransactionID[:]))
}

func stringNativeTokenID(id *iotago.NativeTokenID) string {
	return hex.EncodeToString(id[:])
}

func (txb *AnchorTransactionBuilder) String() string {
	ret := ""
	ret += fmt.Sprintf("%s\n", stringUTXOInput(txb.anchorOutputID.UTXOInput()))
	ret += fmt.Sprintf("in base tokens balance: %d\n", txb.anchorOutput.Amount)
	ret += fmt.Sprintf("current base tokens balance: %d\n", txb.totalBaseTokensInL2Accounts)
	ret += fmt.Sprintf("Native tokens (%d):\n", len(txb.balanceNativeTokens))
	for id, ntb := range txb.balanceNativeTokens {
		initial := "0"
		if ntb.in != nil {
			initial = ntb.getOutValue().String()
		}
		current := ntb.getOutValue().String()
		ret += fmt.Sprintf("      %s: %s --> %s, storage deposit charged: %v\n",
			stringNativeTokenID(&id), initial, current, ntb.storageDepositCharged)
	}
	ret += fmt.Sprintf("consumed inputs (%d):\n", len(txb.consumed))
	//for _, inp := range txb.consumed {
	//	ret += fmt.Sprintf("      %s\n", inp.ID().String())
	//}
	ret += fmt.Sprintf("added outputs (%d):\n", len(txb.postedOutputs))
	ret += ">>>>>> TODO. Not finished....."
	return ret
}
