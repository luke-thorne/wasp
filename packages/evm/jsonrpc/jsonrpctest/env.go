package jsonrpctest

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/iotaledger/wasp/packages/evm"
	"github.com/iotaledger/wasp/packages/evm/evmtest"
	"github.com/iotaledger/wasp/packages/evm/jsonrpc"
	"github.com/stretchr/testify/require"
)

type Env struct {
	T         *testing.T
	Server    *rpc.Server
	Client    *ethclient.Client
	RawClient *rpc.Client
	ChainID   int
}

func (e *Env) signer() types.Signer {
	return evm.Signer(big.NewInt(int64(e.ChainID)))
}

var RequestFundsAmount = big.NewInt(1e18) // 1 ETH

func (e *Env) RequestFunds(target common.Address) *types.Transaction {
	nonce, err := e.Client.NonceAt(context.Background(), evmtest.FaucetAddress, nil)
	require.NoError(e.T, err)
	tx, err := types.SignTx(
		types.NewTransaction(nonce, target, RequestFundsAmount, evm.TxGas, evm.GasPrice, nil),
		e.signer(),
		evmtest.FaucetKey,
	)
	require.NoError(e.T, err)
	err = e.Client.SendTransaction(context.Background(), tx)
	require.NoError(e.T, err)
	return tx
}

func (e *Env) DeployEVMContract(creator *ecdsa.PrivateKey, contractABI abi.ABI, contractBytecode []byte, args ...interface{}) (*types.Transaction, common.Address) {
	creatorAddress := crypto.PubkeyToAddress(creator.PublicKey)

	nonce := e.NonceAt(creatorAddress)

	constructorArguments, err := contractABI.Pack("", args...)
	require.NoError(e.T, err)

	data := concatenate(contractBytecode, constructorArguments)

	value := big.NewInt(0)

	gasLimit := e.estimateGas(ethereum.CallMsg{
		From:     creatorAddress,
		To:       nil, // contract creation
		GasPrice: evm.GasPrice,
		Value:    value,
		Data:     data,
	})

	tx, err := types.SignTx(
		types.NewContractCreation(nonce, value, gasLimit, evm.GasPrice, data),
		e.signer(),
		creator,
	)
	require.NoError(e.T, err)

	err = e.Client.SendTransaction(context.Background(), tx)
	require.NoError(e.T, err)

	return tx, crypto.CreateAddress(creatorAddress, nonce)
}

func concatenate(a, b []byte) []byte {
	r := make([]byte, 0, len(a)+len(b))
	r = append(r, a...)
	r = append(r, b...)
	return r
}

func (e *Env) estimateGas(msg ethereum.CallMsg) uint64 {
	gas, err := e.Client.EstimateGas(context.Background(), msg)
	require.NoError(e.T, err)
	return gas
}

func (e *Env) NonceAt(address common.Address) uint64 {
	nonce, err := e.Client.NonceAt(context.Background(), address, nil)
	require.NoError(e.T, err)
	return nonce
}

func (e *Env) BlockNumber() uint64 {
	blockNumber, err := e.Client.BlockNumber(context.Background())
	require.NoError(e.T, err)
	return blockNumber
}

func (e *Env) BlockByNumber(number *big.Int) *types.Block {
	block, err := e.Client.BlockByNumber(context.Background(), number)
	require.NoError(e.T, err)
	return block
}

func (e *Env) BlockByHash(hash common.Hash) *types.Block {
	block, err := e.Client.BlockByHash(context.Background(), hash)
	if errors.Is(err, ethereum.NotFound) {
		return nil
	}
	require.NoError(e.T, err)
	return block
}

func (e *Env) TransactionByHash(hash common.Hash) *types.Transaction {
	tx, isPending, err := e.Client.TransactionByHash(context.Background(), hash)
	if errors.Is(err, ethereum.NotFound) {
		return nil
	}
	require.NoError(e.T, err)
	require.False(e.T, isPending)
	return tx
}

func (e *Env) TransactionByBlockHashAndIndex(blockHash common.Hash, index uint) *types.Transaction {
	tx, err := e.Client.TransactionInBlock(context.Background(), blockHash, index)
	if errors.Is(err, ethereum.NotFound) {
		return nil
	}
	require.NoError(e.T, err)
	return tx
}

func (e *Env) UncleByBlockHashAndIndex(blockHash common.Hash, index uint) map[string]interface{} {
	var uncle map[string]interface{}
	err := e.RawClient.Call(&uncle, "eth_getUncleByBlockHashAndIndex", blockHash, hexutil.Uint(index))
	require.NoError(e.T, err)
	return uncle
}

func (e *Env) TransactionByBlockNumberAndIndex(blockNumber *big.Int, index uint) *jsonrpc.RPCTransaction {
	var tx *jsonrpc.RPCTransaction
	err := e.RawClient.Call(&tx, "eth_getTransactionByBlockNumberAndIndex", (*hexutil.Big)(blockNumber), hexutil.Uint(index))
	require.NoError(e.T, err)
	return tx
}

func (e *Env) UncleByBlockNumberAndIndex(blockNumber *big.Int, index uint) map[string]interface{} {
	var uncle map[string]interface{}
	err := e.RawClient.Call(&uncle, "eth_getUncleByBlockNumberAndIndex", (*hexutil.Big)(blockNumber), hexutil.Uint(index))
	require.NoError(e.T, err)
	return uncle
}

func (e *Env) BlockTransactionCountByHash(hash common.Hash) uint {
	n, err := e.Client.TransactionCount(context.Background(), hash)
	require.NoError(e.T, err)
	return n
}

func (e *Env) UncleCountByBlockHash(hash common.Hash) uint {
	var res hexutil.Uint
	err := e.RawClient.Call(&res, "eth_getUncleCountByBlockHash", hash)
	require.NoError(e.T, err)
	return uint(res)
}

func (e *Env) BlockTransactionCountByNumber() uint {
	// the client only supports calling this method with "pending"
	n, err := e.Client.PendingTransactionCount(context.Background())
	require.NoError(e.T, err)
	return n
}

func (e *Env) UncleCountByBlockNumber(blockNumber *big.Int) uint {
	var res hexutil.Uint
	err := e.RawClient.Call(&res, "eth_getUncleCountByBlockNumber", (*hexutil.Big)(blockNumber))
	require.NoError(e.T, err)
	return uint(res)
}

func (e *Env) Balance(address common.Address) *big.Int {
	bal, err := e.Client.BalanceAt(context.Background(), address, nil)
	require.NoError(e.T, err)
	return bal
}

func (e *Env) Code(address common.Address) []byte {
	code, err := e.Client.CodeAt(context.Background(), address, nil)
	require.NoError(e.T, err)
	return code
}

func (e *Env) Storage(address common.Address, key common.Hash) []byte {
	data, err := e.Client.StorageAt(context.Background(), address, key, nil)
	require.NoError(e.T, err)
	return data
}

func (e *Env) TxReceipt(hash common.Hash) *types.Receipt {
	r, err := e.Client.TransactionReceipt(context.Background(), hash)
	require.NoError(e.T, err)
	return r
}

func (e *Env) Accounts() []common.Address {
	var res []common.Address
	err := e.RawClient.Call(&res, "eth_accounts")
	require.NoError(e.T, err)
	return res
}

func (e *Env) Sign(address common.Address, data []byte) []byte {
	var res hexutil.Bytes
	err := e.RawClient.Call(&res, "eth_sign", address, hexutil.Bytes(data))
	require.NoError(e.T, err)
	return res
}

func (e *Env) SignTransaction(args *jsonrpc.SendTxArgs) []byte {
	var res hexutil.Bytes
	err := e.RawClient.Call(&res, "eth_signTransaction", args)
	require.NoError(e.T, err)
	return res
}

func (e *Env) SendTransaction(args *jsonrpc.SendTxArgs) common.Hash {
	var res common.Hash
	err := e.RawClient.Call(&res, "eth_sendTransaction", args)
	require.NoError(e.T, err)
	return res
}

func (e *Env) getLogs(q ethereum.FilterQuery) []types.Log {
	logs, err := e.Client.FilterLogs(context.Background(), q)
	require.NoError(e.T, err)
	return logs
}

func (e *Env) TestRPCGetLogs() {
	creator, creatorAddress := evmtest.Accounts[0], evmtest.AccountAddress(0)
	contractABI, err := abi.JSON(strings.NewReader(evmtest.ERC20ContractABI))
	require.NoError(e.T, err)
	contractAddress := crypto.CreateAddress(creatorAddress, e.NonceAt(creatorAddress))

	filterQuery := ethereum.FilterQuery{
		Addresses: []common.Address{contractAddress},
	}

	require.Empty(e.T, e.getLogs(filterQuery))

	e.DeployEVMContract(creator, contractABI, evmtest.ERC20ContractBytecode, "TestCoin", "TEST")

	require.Equal(e.T, 1, len(e.getLogs(filterQuery)))

	recipientAddress := evmtest.AccountAddress(1)
	nonce := hexutil.Uint64(e.NonceAt(creatorAddress))
	callArguments, err := contractABI.Pack("transfer", recipientAddress, big.NewInt(1337))
	value := big.NewInt(0)
	gas := hexutil.Uint64(e.estimateGas(ethereum.CallMsg{
		From:  creatorAddress,
		To:    &contractAddress,
		Value: value,
		Data:  callArguments,
	}))
	require.NoError(e.T, err)
	e.SendTransaction(&jsonrpc.SendTxArgs{
		From:     creatorAddress,
		To:       &contractAddress,
		Gas:      &gas,
		GasPrice: (*hexutil.Big)(evm.GasPrice),
		Value:    (*hexutil.Big)(value),
		Nonce:    &nonce,
		Data:     (*hexutil.Bytes)(&callArguments),
	})

	require.Equal(e.T, 2, len(e.getLogs(filterQuery)))
}
