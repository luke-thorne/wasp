package vmcontext

import (
	"math"
	"math/big"

	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/util/panicutil"
	"github.com/iotaledger/wasp/packages/vm"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
	"github.com/iotaledger/wasp/packages/vm/core/errors/coreerrors"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/core/root"
	"github.com/iotaledger/wasp/packages/vm/gas"
	"github.com/iotaledger/wasp/packages/vm/vmcontext/vmexceptions"
)

// creditToAccount deposits transfer from request to chain account of of the called contract
// It adds new tokens to the chain ledger. It is used when new tokens arrive with a request
func (vmctx *VMContext) creditToAccount(agentID iscp.AgentID, ftokens *iscp.FungibleTokens) {
	vmctx.callCore(accounts.Contract, func(s kv.KVStore) {
		accounts.CreditToAccount(s, agentID, ftokens)
	})
}

func (vmctx *VMContext) creditNFTToAccount(agentID iscp.AgentID, nft *iscp.NFT) {
	vmctx.callCore(accounts.Contract, func(s kv.KVStore) {
		accounts.CreditNFTToAccount(s, agentID, nft)
	})
}

// debitFromAccount subtracts tokens from account if it is enough of it.
// should be called only when posting request
func (vmctx *VMContext) debitFromAccount(agentID iscp.AgentID, transfer *iscp.FungibleTokens) {
	vmctx.callCore(accounts.Contract, func(s kv.KVStore) {
		accounts.DebitFromAccount(s, agentID, transfer)
	})
}

// debitNFTFromAccount removes a NFT from account.
// should be called only when posting request
func (vmctx *VMContext) debitNFTFromAccount(agentID iscp.AgentID, nftID iotago.NFTID) {
	vmctx.callCore(accounts.Contract, func(s kv.KVStore) {
		accounts.DebitNFTFromAccount(s, agentID, nftID)
	})
}

func (vmctx *VMContext) mustMoveBetweenAccounts(fromAgentID, toAgentID iscp.AgentID, fungibleTokens *iscp.FungibleTokens, nfts []iotago.NFTID) {
	vmctx.callCore(accounts.Contract, func(s kv.KVStore) {
		accounts.MustMoveBetweenAccounts(s, fromAgentID, toAgentID, fungibleTokens, nfts)
	})
}

func (vmctx *VMContext) totalL2Assets() *iscp.FungibleTokens {
	var ret *iscp.FungibleTokens
	vmctx.callCore(accounts.Contract, func(s kv.KVStore) {
		ret = accounts.GetTotalL2Assets(s)
	})
	return ret
}

func (vmctx *VMContext) findContractByHname(contractHname iscp.Hname) (ret *root.ContractRecord) {
	vmctx.callCore(root.Contract, func(s kv.KVStore) {
		ret = root.FindContract(s, contractHname)
	})
	return ret
}

func (vmctx *VMContext) getChainInfo() *governance.ChainInfo {
	var ret *governance.ChainInfo
	vmctx.callCore(governance.Contract, func(s kv.KVStore) {
		ret = governance.MustGetChainInfo(s)
	})
	return ret
}

func (vmctx *VMContext) GetBaseTokensBalance(agentID iscp.AgentID) uint64 {
	var ret uint64
	vmctx.callCore(accounts.Contract, func(s kv.KVStore) {
		ret = accounts.GetBaseTokensBalance(s, agentID)
	})
	return ret
}

func (vmctx *VMContext) HasEnoughForAllowance(agentID iscp.AgentID, allowance *iscp.Allowance) bool {
	var ret bool
	vmctx.callCore(accounts.Contract, func(s kv.KVStore) {
		ret = accounts.HasEnoughForAllowance(s, agentID, allowance)
	})
	return ret
}

func (vmctx *VMContext) GetNativeTokenBalance(agentID iscp.AgentID, tokenID *iotago.NativeTokenID) *big.Int {
	var ret *big.Int
	vmctx.callCore(accounts.Contract, func(s kv.KVStore) {
		ret = accounts.GetNativeTokenBalance(s, agentID, tokenID)
	})
	return ret
}

func (vmctx *VMContext) GetNativeTokenBalanceTotal(tokenID *iotago.NativeTokenID) *big.Int {
	var ret *big.Int
	vmctx.callCore(accounts.Contract, func(s kv.KVStore) {
		ret = accounts.GetNativeTokenBalanceTotal(s, tokenID)
	})
	return ret
}

func (vmctx *VMContext) GetAssets(agentID iscp.AgentID) *iscp.FungibleTokens {
	var ret *iscp.FungibleTokens
	vmctx.callCore(accounts.Contract, func(s kv.KVStore) {
		ret = accounts.GetAccountAssets(s, agentID)
		if ret == nil {
			ret = &iscp.FungibleTokens{}
		}
	})
	return ret
}

func (vmctx *VMContext) GetAccountNFTs(agentID iscp.AgentID) (ret []iotago.NFTID) {
	vmctx.callCore(accounts.Contract, func(s kv.KVStore) {
		ret = accounts.GetAccountNFTs(s, agentID)
	})
	return ret
}

func (vmctx *VMContext) GetNFTData(nftID iotago.NFTID) (ret iscp.NFT) {
	vmctx.callCore(accounts.Contract, func(s kv.KVStore) {
		ret = accounts.GetNFTData(s, nftID)
	})
	return ret
}

func (vmctx *VMContext) GetSenderTokenBalanceForFees() uint64 {
	sender := vmctx.req.SenderAccount()
	if sender == nil {
		return 0
	}
	if vmctx.chainInfo.GasFeePolicy.GasFeeTokenID == nil {
		// base tokens are used as gas tokens
		return vmctx.GetBaseTokensBalance(sender)
	}
	// native tokens are used for gas fee
	tokenID := vmctx.chainInfo.GasFeePolicy.GasFeeTokenID
	// to pay for gas chain is configured to use some native token, not base tokens
	tokensAvailableBig := vmctx.GetNativeTokenBalance(sender, tokenID)
	if tokensAvailableBig.IsUint64() {
		return tokensAvailableBig.Uint64()
	}
	return math.MaxUint64
}

func (vmctx *VMContext) requestLookupKey() blocklog.RequestLookupKey {
	return blocklog.NewRequestLookupKey(vmctx.virtualState.BlockIndex(), vmctx.requestIndex)
}

func (vmctx *VMContext) eventLookupKey() blocklog.EventLookupKey {
	return blocklog.NewEventLookupKey(vmctx.virtualState.BlockIndex(), vmctx.requestIndex, vmctx.requestEventIndex)
}

func (vmctx *VMContext) writeReceiptToBlockLog(errProvided error) *blocklog.RequestReceipt {
	vmctx.Debugf("writeReceiptToBlockLog: %s err: %s", vmctx.req.ID(), errProvided)
	receipt := &blocklog.RequestReceipt{
		Request:       vmctx.req,
		GasBudget:     vmctx.gasBudgetAdjusted,
		GasBurned:     vmctx.gasBurned,
		GasFeeCharged: vmctx.gasFeeCharged,
	}

	if errProvided != nil {
		var vmError *iscp.VMError
		if _, ok := errProvided.(*iscp.VMError); ok {
			vmError = errProvided.(*iscp.VMError)
		} else {
			vmError = coreerrors.ErrUntypedError.Create(errProvided.Error())
		}
		receipt.Error = vmError.AsUnresolvedError()
	}

	receipt.GasBurnLog = vmctx.gasBurnLog

	if vmctx.task.EnableGasBurnLogging {
		vmctx.gasBurnLog = gas.NewGasBurnLog()
	}
	var err error
	vmctx.callCore(blocklog.Contract, func(s kv.KVStore) {
		err = blocklog.SaveRequestReceipt(vmctx.State(), receipt, vmctx.requestLookupKey())
	})
	if err != nil {
		panic(err)
	}
	return receipt
}

func (vmctx *VMContext) MustSaveEvent(contract iscp.Hname, msg string) {
	if vmctx.requestEventIndex > vmctx.chainInfo.MaxEventsPerReq {
		panic(vm.ErrTooManyEvents)
	}
	if len([]byte(msg)) > int(vmctx.chainInfo.MaxEventSize) {
		panic(vm.ErrTooLargeEvent)
	}
	vmctx.Debugf("MustSaveEvent/%s: msg: '%s'", contract.String(), msg)

	var err error
	vmctx.callCore(blocklog.Contract, func(s kv.KVStore) {
		err = blocklog.SaveEvent(vmctx.State(), msg, vmctx.eventLookupKey(), contract)
	})
	if err != nil {
		panic(err)
	}
	vmctx.requestEventIndex++
}

// updateOffLedgerRequestMaxAssumedNonce updates stored nonce for off ledger requests
func (vmctx *VMContext) updateOffLedgerRequestMaxAssumedNonce() {
	vmctx.GasBurnEnable(false)
	defer vmctx.GasBurnEnable(true)
	vmctx.callCore(accounts.Contract, func(s kv.KVStore) {
		accounts.SaveMaxAssumedNonce(
			s,
			vmctx.req.SenderAccount(),
			vmctx.req.(iscp.OffLedgerRequest).Nonce(),
		)
	})
}

// adjustL2BaseTokensIfNeeded adjust L2 ledger for base tokens if the L1 changed because of dust deposit changes
func (vmctx *VMContext) adjustL2BaseTokensIfNeeded(adjustment int64, account iscp.AgentID) {
	if adjustment == 0 {
		return
	}
	err := panicutil.CatchPanicReturnError(func() {
		vmctx.callCore(accounts.Contract, func(s kv.KVStore) {
			accounts.AdjustAccountBaseTokens(s, account, adjustment)
		})
	}, accounts.ErrNotEnoughFunds)
	if err != nil {
		panic(vmexceptions.ErrNotEnoughFundsForInternalDustDeposit)
	}
}
