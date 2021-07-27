// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package evmchain

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotaledger/wasp/packages/evm"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/iscp/assert"
	"github.com/iotaledger/wasp/packages/iscp/colored"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/kv/kvdecoder"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
)

var Processor = Contract.Processor(initialize,
	// Ethereum blockchain
	FuncSendTransaction.WithHandler(applyTransaction),
	FuncGetBalance.WithHandler(getBalance),
	FuncCallContract.WithHandler(callContract),
	FuncEstimateGas.WithHandler(estimateGas),
	FuncGetNonce.WithHandler(getNonce),
	FuncGetReceipt.WithHandler(getReceipt),
	FuncGetCode.WithHandler(getCode),
	FuncGetBlockNumber.WithHandler(getBlockNumber),
	FuncGetBlockByNumber.WithHandler(getBlockByNumber),
	FuncGetBlockByHash.WithHandler(getBlockByHash),
	FuncGetTransactionByHash.WithHandler(getTransactionByHash),
	FuncGetTransactionByBlockHashAndIndex.WithHandler(getTransactionByBlockHashAndIndex),
	FuncGetTransactionByBlockNumberAndIndex.WithHandler(getTransactionByBlockNumberAndIndex),
	FuncGetBlockTransactionCountByHash.WithHandler(getBlockTransactionCountByHash),
	FuncGetBlockTransactionCountByNumber.WithHandler(getBlockTransactionCountByNumber),
	FuncGetStorage.WithHandler(getStorage),
	FuncGetLogs.WithHandler(getLogs),

	// EVMchain SC management
	FuncSetNextOwner.WithHandler(setNextOwner),
	FuncClaimOwnership.WithHandler(claimOwnership),
	FuncSetGasPerIota.WithHandler(setGasPerIota),
	FuncWithdrawGasFees.WithHandler(withdrawGasFees),
	FuncGetOwner.WithHandler(getOwner),
	FuncGetGasPerIota.WithHandler(getGasPerIota),
)

func initialize(ctx iscp.Sandbox) (dict.Dict, error) {
	a := assert.NewAssert(ctx.Log())
	genesisAlloc, err := DecodeGenesisAlloc(ctx.Params().MustGet(FieldGenesisAlloc))
	a.RequireNoError(err)
	chainID, ok, err := codec.DecodeUint16(ctx.Params().MustGet(FieldChainID))
	a.RequireNoError(err)
	if !ok {
		chainID = evm.DefaultChainID
	}
	evm.InitGenesis(int(chainID), rawdb.NewDatabase(evm.NewKVAdapter(ctx.State())), genesisAlloc, evm.GasLimitDefault)
	ctx.State().Set(FieldGasPerIota, codec.EncodeUint64(DefaultGasPerIota))
	ctx.State().Set(FieldEvmOwner, codec.EncodeAgentID(ctx.ContractCreator()))
	return nil, nil
}

func applyTransaction(ctx iscp.Sandbox) (dict.Dict, error) {
	a := assert.NewAssert(ctx.Log())

	tx := &types.Transaction{}
	err := tx.UnmarshalBinary(ctx.Params().MustGet(FieldTransactionData))
	a.RequireNoError(err)

	transferredIotas := ctx.IncomingTransfer().Get(getFeeColor(ctx))
	gasPerIota, _, err := codec.DecodeUint64(ctx.State().MustGet(FieldGasPerIota))
	a.RequireNoError(err)

	a.Require(
		transferredIotas >= tx.Gas()/gasPerIota,
		"transferred tokens (%d) not enough to cover the gas limit set in the transaction (%d at %d gas per iota token)", transferredIotas, tx.Gas(), gasPerIota,
	)

	emu := getOrCreateEmulator(ctx)
	usedGas, err := emu.SendTransaction(tx)
	a.RequireNoError(err)

	iotasGasFee := usedGas / gasPerIota
	if transferredIotas > iotasGasFee {
		// refund unspent gas fee to the sender's on-chain account
		iotasGasRefund := transferredIotas - iotasGasFee
		_, err = ctx.Call(
			accounts.Contract.Hname(),
			accounts.FuncDeposit.Hname(),
			dict.Dict{accounts.ParamAgentID: codec.EncodeAgentID(ctx.Caller())},
			colored.NewBalancesForIotas(iotasGasRefund),
		)
		a.RequireNoError(err)
	}

	return dict.Dict{
		FieldGasFee:  codec.EncodeUint64(iotasGasFee),
		FieldGasUsed: codec.EncodeUint64(usedGas),
	}, nil
}

func getBalance(ctx iscp.SandboxView) (dict.Dict, error) {
	a := assert.NewAssert(ctx.Log())
	addr := common.BytesToAddress(ctx.Params().MustGet(FieldAddress))
	blockNumber := paramBlockNumber(ctx)

	return withEmulatorR(ctx, func(emu *evm.EVMEmulator) dict.Dict {
		bal, err := emu.BalanceAt(addr, blockNumber)
		a.RequireNoError(err)
		return result(bal.Bytes())
	})
}

func getBlockNumber(ctx iscp.SandboxView) (dict.Dict, error) {
	return withEmulatorR(ctx, func(emu *evm.EVMEmulator) dict.Dict {
		return result(emu.Blockchain().CurrentBlock().Number().Bytes())
	})
}

func getBlockByNumber(ctx iscp.SandboxView) (dict.Dict, error) {
	return withBlockByNumber(ctx, func(emu *evm.EVMEmulator, block *types.Block) dict.Dict {
		if block == nil {
			return nil
		}
		return result(EncodeBlock(block))
	})
}

func getBlockByHash(ctx iscp.SandboxView) (dict.Dict, error) {
	return withBlockByHash(ctx, func(emu *evm.EVMEmulator, block *types.Block) dict.Dict {
		if block == nil {
			return nil
		}
		return result(EncodeBlock(block))
	})
}

func getTransactionByHash(ctx iscp.SandboxView) (dict.Dict, error) {
	return withTransactionByHash(ctx, func(emu *evm.EVMEmulator, tx *types.Transaction) dict.Dict {
		return txResult(ctx, emu, tx)
	})
}

func getTransactionByBlockHashAndIndex(ctx iscp.SandboxView) (dict.Dict, error) {
	return withTransactionByBlockHashAndIndex(ctx, func(emu *evm.EVMEmulator, tx *types.Transaction) dict.Dict {
		return txResult(ctx, emu, tx)
	})
}

func getTransactionByBlockNumberAndIndex(ctx iscp.SandboxView) (dict.Dict, error) {
	return withTransactionByBlockNumberAndIndex(ctx, func(emu *evm.EVMEmulator, tx *types.Transaction) dict.Dict {
		return txResult(ctx, emu, tx)
	})
}

func getBlockTransactionCountByHash(ctx iscp.SandboxView) (dict.Dict, error) {
	return withBlockByHash(ctx, func(emu *evm.EVMEmulator, block *types.Block) dict.Dict {
		if block == nil {
			return nil
		}
		return result(codec.EncodeUint64(uint64(len(block.Transactions()))))
	})
}

func getBlockTransactionCountByNumber(ctx iscp.SandboxView) (dict.Dict, error) {
	return withBlockByNumber(ctx, func(emu *evm.EVMEmulator, block *types.Block) dict.Dict {
		if block == nil {
			return nil
		}
		return result(codec.EncodeUint64(uint64(len(block.Transactions()))))
	})
}

func getReceipt(ctx iscp.SandboxView) (dict.Dict, error) {
	a := assert.NewAssert(ctx.Log())
	return withTransactionByHash(ctx, func(emu *evm.EVMEmulator, tx *types.Transaction) dict.Dict {
		if tx == nil {
			return nil
		}
		receipt, err := emu.TransactionReceipt(tx.Hash())
		a.RequireNoError(err)

		return result(NewReceipt(receipt, tx).Bytes())
	})
}

func getNonce(ctx iscp.SandboxView) (dict.Dict, error) {
	a := assert.NewAssert(ctx.Log())
	addr := common.BytesToAddress(ctx.Params().MustGet(FieldAddress))
	blockNumber := paramBlockNumber(ctx)

	return withEmulatorR(ctx, func(emu *evm.EVMEmulator) dict.Dict {
		nonce, err := emu.NonceAt(addr, blockNumber)
		a.RequireNoError(err)
		return result(codec.EncodeUint64(nonce))
	})
}

func getCode(ctx iscp.SandboxView) (dict.Dict, error) {
	a := assert.NewAssert(ctx.Log())
	addr := common.BytesToAddress(ctx.Params().MustGet(FieldAddress))
	blockNumber := paramBlockNumber(ctx)

	return withEmulatorR(ctx, func(emu *evm.EVMEmulator) dict.Dict {
		code, err := emu.CodeAt(addr, blockNumber)
		a.RequireNoError(err)
		return result(code)
	})
}

func getStorage(ctx iscp.SandboxView) (dict.Dict, error) {
	a := assert.NewAssert(ctx.Log())
	addr := common.BytesToAddress(ctx.Params().MustGet(FieldAddress))
	key := common.BytesToHash(ctx.Params().MustGet(FieldKey))
	blockNumber := paramBlockNumber(ctx)

	return withEmulatorR(ctx, func(emu *evm.EVMEmulator) dict.Dict {
		data, err := emu.StorageAt(addr, key, blockNumber)
		a.RequireNoError(err)
		return result(data)
	})
}

func getLogs(ctx iscp.SandboxView) (dict.Dict, error) {
	a := assert.NewAssert(ctx.Log())
	q, err := DecodeFilterQuery(ctx.Params().MustGet(FieldFilterQuery))
	a.RequireNoError(err)

	return withEmulatorR(ctx, func(emu *evm.EVMEmulator) dict.Dict {
		logs, err := emu.FilterLogs(q)
		a.RequireNoError(err)
		return result(EncodeLogs(logs))
	})
}

func callContract(ctx iscp.SandboxView) (dict.Dict, error) {
	a := assert.NewAssert(ctx.Log())
	callMsg, err := DecodeCallMsg(ctx.Params().MustGet(FieldCallMsg))
	a.RequireNoError(err)
	blockNumber := paramBlockNumber(ctx)

	return withEmulatorR(ctx, func(emu *evm.EVMEmulator) dict.Dict {
		res, err := emu.CallContract(callMsg, blockNumber)
		a.RequireNoError(err)
		return result(res)
	})
}

func estimateGas(ctx iscp.SandboxView) (dict.Dict, error) {
	a := assert.NewAssert(ctx.Log())
	callMsg, err := DecodeCallMsg(ctx.Params().MustGet(FieldCallMsg))
	a.RequireNoError(err)

	return withEmulatorR(ctx, func(emu *evm.EVMEmulator) dict.Dict {
		gas, err := emu.EstimateGas(callMsg)
		a.RequireNoError(err)
		return result(codec.EncodeUint64(gas))
	})
}

// EVM chain management functions ///////////////////////////////////////////////////////////////////////////////////////

func requireOwner(ctx iscp.Sandbox) {
	contractOwner, _, err := codec.DecodeAgentID(ctx.State().MustGet(FieldEvmOwner))
	a := assert.NewAssert(ctx.Log())
	a.RequireNoError(err)
	a.Require(contractOwner.Equals(ctx.Caller()), "can only be called by the contract owner")
}

func setNextOwner(ctx iscp.Sandbox) (dict.Dict, error) {
	requireOwner(ctx)
	par := kvdecoder.New(ctx.Params(), ctx.Log())
	ctx.State().Set(FieldNextEvmOwner, codec.EncodeAgentID(par.MustGetAgentID(FieldNextEvmOwner)))
	return nil, nil
}

func claimOwnership(ctx iscp.Sandbox) (dict.Dict, error) {
	a := assert.NewAssert(ctx.Log())

	nextOwner, _, err := codec.DecodeAgentID(ctx.State().MustGet(FieldNextEvmOwner))
	a.RequireNoError(err)
	a.Require(nextOwner.Equals(ctx.Caller()), "Can only be called by the contract owner")

	ctx.State().Set(FieldEvmOwner, codec.EncodeAgentID(&nextOwner))
	return nil, nil
}

func getOwner(ctx iscp.SandboxView) (dict.Dict, error) {
	return result(ctx.State().MustGet(FieldEvmOwner)), nil
}

func setGasPerIota(ctx iscp.Sandbox) (dict.Dict, error) {
	requireOwner(ctx)
	par := kvdecoder.New(ctx.Params())
	gasPerIotaBin := codec.EncodeUint64(par.MustGetUint64(FieldGasPerIota))
	ctx.State().Set(FieldGasPerIota, gasPerIotaBin)
	return nil, nil
}

func getGasPerIota(ctx iscp.SandboxView) (dict.Dict, error) {
	return result(ctx.State().MustGet(FieldGasPerIota)), nil
}

func withdrawGasFees(ctx iscp.Sandbox) (dict.Dict, error) {
	a := assert.NewAssert(ctx.Log())
	requireOwner(ctx)

	paramsDecoder := kvdecoder.New(ctx.Params(), ctx.Log())
	targetAgentID := paramsDecoder.MustGetAgentID(FieldAgentID, *ctx.Caller())

	isOnChain := targetAgentID.Address().Equals(ctx.ChainID().AsAddress())

	if isOnChain {
		params := codec.MakeDict(map[string]interface{}{
			accounts.ParamAgentID: targetAgentID,
		})
		_, err := ctx.Call(accounts.Contract.Hname(), accounts.FuncDeposit.Hname(), params, ctx.Balances())
		a.RequireNoError(err)
		return nil, nil
	}

	a.Require(ctx.Send(targetAgentID.Address(), ctx.Balances(), &iscp.SendMetadata{
		TargetContract: targetAgentID.Hname(),
	}), "withdraw.inconsistency: failed sending tokens ")

	return nil, nil
}
