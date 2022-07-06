// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package jsonrpctest

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/iotaledger/wasp/packages/evm/evmtest"
	"github.com/iotaledger/wasp/packages/evm/evmtypes"
	"github.com/iotaledger/wasp/packages/evm/evmutil"
	"github.com/iotaledger/wasp/packages/evm/jsonrpc"
	"github.com/iotaledger/wasp/packages/solo"
	"github.com/iotaledger/wasp/packages/vm/core/evm"
	"github.com/stretchr/testify/require"
)

type soloTestEnv struct {
	Env
	solo      *solo.Solo
	soloChain *solo.Chain
}

func newSoloTestEnv(t *testing.T) *soloTestEnv {
	evmtest.InitGoEthLogger(t)

	s := solo.New(t, &solo.InitOptions{AutoAdjustDustDeposit: true, Debug: true, PrintStackTrace: true})
	chainOwner, _ := s.NewKeyPairWithFunds()
	chain := s.NewChain(chainOwner, "iscpchain")

	accounts := jsonrpc.NewAccountManager(nil)
	rpcsrv := jsonrpc.NewServer(chain.EVM(), accounts)
	t.Cleanup(rpcsrv.Stop)

	rawClient := rpc.DialInProc(rpcsrv)
	client := ethclient.NewClient(rawClient)
	t.Cleanup(client.Close)

	return &soloTestEnv{
		Env: Env{
			T:              t,
			Client:         client,
			RawClient:      rawClient,
			ChainID:        evm.DefaultChainID,
			accountManager: accounts,
		},
		solo:      s,
		soloChain: chain,
	}
}

func TestRPCGetBalance(t *testing.T) {
	env := newSoloTestEnv(t)
	_, emptyAddress := solo.NewEthereumAccount()
	require.Zero(t, env.Balance(emptyAddress).Uint64())
	_, nonEmptyAddress := env.soloChain.NewEthereumAccountWithL2Funds()
	require.NotZero(t, env.Balance(nonEmptyAddress).Uint64())
}

func TestRPCGetCode(t *testing.T) {
	env := newSoloTestEnv(t)
	creator, creatorAddress := env.soloChain.NewEthereumAccountWithL2Funds()

	// account address
	require.Empty(t, env.Code(creatorAddress))

	// contract address
	{
		_, contractAddress, _ := env.deployStorageContract(creator, 42)
		require.NotEmpty(t, env.Code(contractAddress))
	}
}

func TestRPCGetStorage(t *testing.T) {
	env := newSoloTestEnv(t)
	creator, _ := env.soloChain.NewEthereumAccountWithL2Funds()

	_, contractAddress, contractABI := env.deployStorageContract(creator, 42)

	// first static variable in contract (uint32 n) has slot 0. See:
	// https://docs.soliditylang.org/en/v0.6.6/miscellaneous.html#layout-of-state-variables-in-storage
	slot := common.Hash{}
	ret := env.Storage(contractAddress, slot)

	var v uint32
	err := contractABI.UnpackIntoInterface(&v, "retrieve", ret)
	require.NoError(t, err)
	require.Equal(t, uint32(42), v)
}

func TestRPCBlockNumber(t *testing.T) {
	env := newSoloTestEnv(t)
	require.EqualValues(t, 0, env.BlockNumber())
	creator, _ := env.soloChain.NewEthereumAccountWithL2Funds()
	require.EqualValues(t, 1, env.BlockNumber()) // EVM block number is incremented along with ISC block index
	env.deployStorageContract(creator, 42)
	require.EqualValues(t, 2, env.BlockNumber())
}

func TestRPCGetTransactionCount(t *testing.T) {
	env := newSoloTestEnv(t)
	creator, creatorAddress := env.soloChain.NewEthereumAccountWithL2Funds()
	require.EqualValues(t, 0, env.NonceAt(creatorAddress))
	env.deployStorageContract(creator, 42)
	require.EqualValues(t, 1, env.NonceAt(creatorAddress))
}

func TestRPCGetBlockByNumber(t *testing.T) {
	env := newSoloTestEnv(t)
	creator, _ := env.soloChain.NewEthereumAccountWithL2Funds()
	require.EqualValues(t, 0, env.BlockByNumber(big.NewInt(0)).Number().Uint64())
	env.deployStorageContract(creator, 42)
	require.EqualValues(t, 1, env.BlockByNumber(big.NewInt(1)).Number().Uint64())
}

func TestRPCGetBlockByHash(t *testing.T) {
	env := newSoloTestEnv(t)
	require.Nil(t, env.BlockByHash(common.Hash{}))
	creator, _ := env.soloChain.NewEthereumAccountWithL2Funds()
	require.EqualValues(t, 0, env.BlockByHash(env.BlockByNumber(big.NewInt(0)).Hash()).Number().Uint64())
	env.deployStorageContract(creator, 42)
	require.EqualValues(t, 1, env.BlockByHash(env.BlockByNumber(big.NewInt(1)).Hash()).Number().Uint64())
}

func TestRPCGetTransactionByHash(t *testing.T) {
	env := newSoloTestEnv(t)
	require.Nil(t, env.TransactionByHash(common.Hash{}))
	creator, _ := env.soloChain.NewEthereumAccountWithL2Funds()
	env.deployStorageContract(creator, 42)
	block := env.BlockByNumber(new(big.Int).SetUint64(env.BlockNumber()))
	tx := env.TransactionByHash(block.Transactions()[0].Hash())
	require.Equal(t, block.Transactions()[0].Hash(), tx.Hash())
}

func TestRPCGetTransactionByBlockHashAndIndex(t *testing.T) {
	env := newSoloTestEnv(t)
	require.Nil(t, env.TransactionByBlockHashAndIndex(common.Hash{}, 0))
	creator, _ := env.soloChain.NewEthereumAccountWithL2Funds()
	env.deployStorageContract(creator, 42)
	block := env.BlockByNumber(new(big.Int).SetUint64(env.BlockNumber()))
	tx := env.TransactionByBlockHashAndIndex(block.Hash(), 0)
	require.Equal(t, block.Transactions()[0].Hash(), tx.Hash())
}

func TestRPCGetUncleByBlockHashAndIndex(t *testing.T) {
	env := newSoloTestEnv(t)
	require.Nil(t, env.UncleByBlockHashAndIndex(common.Hash{}, 0))
	creator, _ := env.soloChain.NewEthereumAccountWithL2Funds()
	env.deployStorageContract(creator, 42)
	block := env.BlockByNumber(new(big.Int).SetUint64(env.BlockNumber()))
	require.Nil(t, env.UncleByBlockHashAndIndex(block.Hash(), 0))
}

func TestRPCGetTransactionByBlockNumberAndIndex(t *testing.T) {
	env := newSoloTestEnv(t)
	require.Nil(t, env.TransactionByBlockNumberAndIndex(big.NewInt(3), 0))
	creator, _ := env.soloChain.NewEthereumAccountWithL2Funds()
	env.deployStorageContract(creator, 42)
	block := env.BlockByNumber(new(big.Int).SetUint64(env.BlockNumber()))
	tx := env.TransactionByBlockNumberAndIndex(block.Number(), 0)
	require.EqualValues(t, block.Hash(), *tx.BlockHash)
	require.EqualValues(t, 0, *tx.TransactionIndex)
}

func TestRPCGetUncleByBlockNumberAndIndex(t *testing.T) {
	env := newSoloTestEnv(t)
	require.Nil(t, env.UncleByBlockNumberAndIndex(big.NewInt(3), 0))
	creator, _ := env.soloChain.NewEthereumAccountWithL2Funds()
	env.deployStorageContract(creator, 42)
	block := env.BlockByNumber(new(big.Int).SetUint64(env.BlockNumber()))
	require.Nil(t, env.UncleByBlockNumberAndIndex(block.Number(), 0))
}

func TestRPCGetTransactionCountByHash(t *testing.T) {
	env := newSoloTestEnv(t)
	creator, _ := env.soloChain.NewEthereumAccountWithL2Funds()
	env.deployStorageContract(creator, 42)
	block := env.BlockByNumber(new(big.Int).SetUint64(env.BlockNumber()))
	require.Positive(t, len(block.Transactions()))
	require.EqualValues(t, len(block.Transactions()), env.BlockTransactionCountByHash(block.Hash()))
	require.EqualValues(t, 0, env.BlockTransactionCountByHash(common.Hash{}))
}

func TestRPCGetUncleCountByBlockHash(t *testing.T) {
	env := newSoloTestEnv(t)
	creator, _ := env.soloChain.NewEthereumAccountWithL2Funds()
	env.deployStorageContract(creator, 42)
	block := env.BlockByNumber(new(big.Int).SetUint64(env.BlockNumber()))
	require.Zero(t, len(block.Uncles()))
	require.EqualValues(t, len(block.Uncles()), env.UncleCountByBlockHash(block.Hash()))
	require.EqualValues(t, 0, env.UncleCountByBlockHash(common.Hash{}))
}

func TestRPCGetTransactionCountByNumber(t *testing.T) {
	env := newSoloTestEnv(t)
	creator, _ := env.soloChain.NewEthereumAccountWithL2Funds()
	env.deployStorageContract(creator, 42)
	block := env.BlockByNumber(new(big.Int).SetUint64(env.BlockNumber()))
	require.Positive(t, len(block.Transactions()))
	require.EqualValues(t, len(block.Transactions()), env.BlockTransactionCountByNumber())
}

func TestRPCGetUncleCountByBlockNumber(t *testing.T) {
	env := newSoloTestEnv(t)
	creator, _ := env.soloChain.NewEthereumAccountWithL2Funds()
	env.deployStorageContract(creator, 42)
	block := env.BlockByNumber(new(big.Int).SetUint64(env.BlockNumber()))
	require.Zero(t, len(block.Uncles()))
	require.EqualValues(t, len(block.Uncles()), env.UncleCountByBlockNumber(big.NewInt(1)))
}

func TestRPCAccounts(t *testing.T) {
	env := newSoloTestEnv(t)
	k1, _ := solo.NewEthereumAccount()
	k2, _ := solo.NewEthereumAccount()
	env.accountManager.Add(k1)
	env.accountManager.Add(k2)
	accounts := env.Accounts()
	require.EqualValues(t, 2, len(accounts))
}

func TestRPCSign(t *testing.T) {
	k1, a1 := solo.NewEthereumAccount()
	env := newSoloTestEnv(t)
	env.accountManager.Add(k1)
	signed := env.Sign(a1, []byte("hello"))
	require.NotEmpty(t, signed)
}

const additionalGasBurnedByVM = 10

func TestRPCSignTransaction(t *testing.T) {
	_, to := solo.NewEthereumAccount()
	env := newSoloTestEnv(t)
	ethKey, ethAddr := env.soloChain.NewEthereumAccountWithL2Funds()
	env.accountManager.Add(ethKey)

	gas := hexutil.Uint64(params.TxGas) + additionalGasBurnedByVM
	nonce := hexutil.Uint64(env.NonceAt(ethAddr))
	signed := env.SignTransaction(&jsonrpc.SendTxArgs{
		From:     ethAddr,
		To:       &to,
		Gas:      &gas,
		GasPrice: (*hexutil.Big)(evm.GasPrice),
		Value:    (*hexutil.Big)(big.NewInt(42)),
		Nonce:    &nonce,
	})
	require.NotEmpty(t, signed)

	// assert that the tx is correctly signed
	{
		decodedTx, err := evmtypes.DecodeTransaction(signed)
		require.NoError(t, err)
		sender, err := evmutil.GetSender(decodedTx)
		require.NoError(t, err)
		require.Equal(t, ethAddr, sender)
	}
}

func TestRPCSendTransaction(t *testing.T) {
	env := newSoloTestEnv(t)
	ethKey, ethAddr := env.soloChain.NewEthereumAccountWithL2Funds()
	env.accountManager.Add(ethKey)

	gas := hexutil.Uint64(100_000)
	nonce := hexutil.Uint64(env.NonceAt(ethAddr))
	data := common.Hex2Bytes("600180600b6000396000f3") // some contract bytecode
	txHash := env.MustSendTransaction(&jsonrpc.SendTxArgs{
		From:     ethAddr,
		Gas:      &gas,
		GasPrice: (*hexutil.Big)(evm.GasPrice),
		Nonce:    &nonce,
		Data:     (*hexutil.Bytes)(&data),
	})
	require.NotEqualValues(t, common.Hash{}, txHash)
}

func TestRPCGetTxReceipt(t *testing.T) {
	env := newSoloTestEnv(t)
	creator, _ := env.soloChain.NewEthereumAccountWithL2Funds()

	contractABI, err := abi.JSON(strings.NewReader(evmtest.StorageContractABI))
	require.NoError(t, err)
	tx, receipt, contractAddress := env.DeployEVMContract(creator, contractABI, evmtest.StorageContractBytecode, uint32(42))

	require.EqualValues(t, types.LegacyTxType, receipt.Type)
	require.EqualValues(t, types.ReceiptStatusSuccessful, receipt.Status)
	require.NotZero(t, receipt.CumulativeGasUsed)
	require.EqualValues(t, types.Bloom{}, receipt.Bloom)
	require.EqualValues(t, 0, len(receipt.Logs))

	require.EqualValues(t, tx.Hash(), receipt.TxHash)
	require.EqualValues(t, contractAddress, receipt.ContractAddress)
	require.NotZero(t, receipt.GasUsed)

	require.EqualValues(t, big.NewInt(2), receipt.BlockNumber)
	require.EqualValues(t, env.BlockByNumber(big.NewInt(2)).Hash(), receipt.BlockHash)
	require.EqualValues(t, 0, receipt.TransactionIndex)
}

func TestRPCGetTxReceiptMissing(t *testing.T) {
	env := newSoloTestEnv(t)

	_, err := env.TxReceipt(common.Hash{})
	require.Error(t, err)
	require.Equal(t, "not found", err.Error())
}

func TestRPCCall(t *testing.T) {
	env := newSoloTestEnv(t)
	creator, creatorAddress := env.soloChain.NewEthereumAccountWithL2Funds()
	contractABI, err := abi.JSON(strings.NewReader(evmtest.StorageContractABI))
	require.NoError(t, err)
	_, _, contractAddress := env.DeployEVMContract(creator, contractABI, evmtest.StorageContractBytecode, uint32(42))

	callArguments, err := contractABI.Pack("retrieve")
	require.NoError(t, err)

	ret, err := env.Client.CallContract(context.Background(), ethereum.CallMsg{
		From: creatorAddress,
		To:   &contractAddress,
		Data: callArguments,
	}, nil)
	require.NoError(t, err)

	var v uint32
	err = contractABI.UnpackIntoInterface(&v, "retrieve", ret)
	require.NoError(t, err)
	require.Equal(t, uint32(42), v)
}

func TestRPCGetLogs(t *testing.T) {
	env := newSoloTestEnv(t)
	env.TestRPCGetLogs(env.soloChain.NewEthereumAccountWithL2Funds)
}

func TestRPCGasLimit(t *testing.T) {
	env := newSoloTestEnv(t)
	env.TestRPCGasLimit(env.soloChain.NewEthereumAccountWithL2Funds)
}

func TestRPCEthChainID(t *testing.T) {
	env := newSoloTestEnv(t)
	var chainID hexutil.Uint
	err := env.RawClient.Call(&chainID, "eth_chainId")
	require.NoError(t, err)
	require.EqualValues(t, evm.DefaultChainID, chainID)
}
