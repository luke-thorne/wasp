// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package evmchain

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotaledger/wasp/packages/evm/evmtest"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/iscp/colored"
	"github.com/iotaledger/wasp/packages/solo"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/stretchr/testify/require"
)

func TestDeploy(t *testing.T) {
	initEVMChain(t)
}

func TestFaucetBalance(t *testing.T) {
	evmChain := initEVMChain(t)
	bal := evmChain.getBalance(evmChain.faucetAddress())
	require.Zero(t, evmChain.faucetSupply.Cmp(bal))
}

func TestStorageContract(t *testing.T) {
	evmChain := initEVMChain(t)

	// deploy solidity `storage` contract
	storage := evmChain.deployStorageContract(evmChain.faucetKey, 42)

	// call evmchain's FuncCallView to call EVM contract's `retrieve` view, get 42
	require.EqualValues(t, 42, storage.retrieve())

	// call FuncSendTransaction with EVM tx that calls `store(43)`
	res, err := storage.store(43)
	require.NoError(t, err)
	require.Equal(t, types.ReceiptStatusSuccessful, res.receipt.Status)

	// call `retrieve` view, get 43
	require.EqualValues(t, 43, storage.retrieve())
}

func TestERC20Contract(t *testing.T) {
	evmChain := initEVMChain(t)

	// deploy solidity `erc20` contract
	erc20 := evmChain.deployERC20Contract(evmChain.faucetKey, "TestCoin", "TEST")

	// call `totalSupply` view
	{
		v := erc20.totalSupply()
		// 100 * 10^18
		expected := new(big.Int).Mul(big.NewInt(100), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
		require.Zero(t, v.Cmp(expected))
	}

	_, recipientAddress := generateEthereumKey(t)
	transferAmount := big.NewInt(1337)

	// call `transfer` => send 1337 TestCoin to recipientAddress
	res, err := erc20.transfer(recipientAddress, transferAmount)
	require.NoError(t, err)

	require.Equal(t, types.ReceiptStatusSuccessful, res.receipt.Status)
	require.Equal(t, 1, len(res.receipt.Logs))

	// call `balanceOf` view => check balance of recipient = 1337 TestCoin
	require.Zero(t, erc20.balanceOf(recipientAddress).Cmp(transferAmount))
}

func TestGetCode(t *testing.T) {
	evmChain := initEVMChain(t)
	erc20 := evmChain.deployERC20Contract(evmChain.faucetKey, "TestCoin", "TEST")

	// get contract bytecode from EVM emulator
	retrievedBytecode := evmChain.getCode(erc20.address)

	// ensure returned bytecode matches the expected runtime bytecode
	require.True(t, bytes.Equal(retrievedBytecode, evmtest.ERC20ContractRuntimeBytecode), "bytecode retrieved from the chain must match the deployed bytecode")
}

func TestGasCharged(t *testing.T) {
	evmChain := initEVMChain(t)
	storage := evmChain.deployStorageContract(evmChain.faucetKey, 42)

	iotaWallet, iotaAddress := evmChain.solo.NewKeyPairWithFunds()
	iotaAgentID := iscp.NewAgentID(iotaAddress, 0)

	initialBalance := evmChain.solo.GetAddressBalance(iotaAddress, colored.IOTA)
	iotasSent := initialBalance - 1

	// call `store(999)` with enough gas
	res, err := storage.store(999, ethCallOptions{iota: iotaCallOptions{wallet: iotaWallet, transfer: iotasSent}})
	require.NoError(t, err)
	require.Greater(t, res.iotaChargedFee, uint64(0))

	// call `retrieve` view, get 42
	require.EqualValues(t, 999, storage.retrieve())

	// user on-chain account is credited with excess iotas (iotasSent - gasUsed)
	expectedUserBalance := iotasSent - res.iotaChargedFee

	evmChain.soloChain.AssertIotas(iotaAgentID, expectedUserBalance)

	// call `store(123)` without enough gas
	_, err = storage.store(123, ethCallOptions{iota: iotaCallOptions{wallet: iotaWallet, transfer: 1}})
	require.Contains(t, err.Error(), "transferred tokens (1) not enough")

	// call `retrieve` view, get 999 - which means store(123) failed and the previous state is kept
	require.EqualValues(t, 999, storage.retrieve())

	// verify user on-chain account still has the same balance (no refund happened)
	evmChain.soloChain.AssertIotas(iotaAgentID, expectedUserBalance)
}

func TestOwner(t *testing.T) {
	evmChain := initEVMChain(t)

	// the default owner is correct
	owner := evmChain.getOwner()
	require.True(t, owner.Equals(&evmChain.soloChain.OriginatorAgentID))

	// only the owner can call the setOwner endpoint
	user1Wallet, user1Address := evmChain.solo.NewKeyPairWithFunds()
	user1AgentID := iscp.NewAgentID(user1Address, 0)
	_, err := evmChain.soloChain.PostRequestSync(
		solo.NewCallParams(Contract.Name, FuncSetNextOwner.Name, FieldNextEvmOwner, user1AgentID).
			WithIotas(100000),
		user1Wallet,
	)
	require.Error(t, err)

	// ensure owner didn't change after a failed call
	owner = evmChain.getOwner()
	require.True(t, owner.Equals(&evmChain.soloChain.OriginatorAgentID))

	// current owner is able to set a new "next owner"
	_, err = evmChain.soloChain.PostRequestSync(
		solo.NewCallParams(Contract.Name, FuncSetNextOwner.Name, FieldNextEvmOwner, user1AgentID).
			WithIotas(100000),
		evmChain.soloChain.OriginatorKeyPair,
	)
	require.NoError(t, err)

	// check that the owner didn't change yet (new owner needs to claim ownership)
	owner = evmChain.getOwner()
	require.True(t, owner.Equals(&evmChain.soloChain.OriginatorAgentID))

	// check no other user can claim ownership
	user2Wallet, _ := evmChain.solo.NewKeyPairWithFunds()

	_, err = evmChain.soloChain.PostRequestSync(
		solo.NewCallParams(Contract.Name, FuncClaimOwnership.Name).
			WithIotas(100000),
		user2Wallet,
	)
	require.Error(t, err)

	// owner still the same
	owner = evmChain.getOwner()
	require.True(t, owner.Equals(&evmChain.soloChain.OriginatorAgentID))

	// claim ownership successfully
	_, err = evmChain.soloChain.PostRequestSync(
		solo.NewCallParams(Contract.Name, FuncClaimOwnership.Name).
			WithIotas(100000),
		user1Wallet,
	)
	require.NoError(t, err)
	owner = evmChain.getOwner()
	require.True(t, owner.Equals(user1AgentID))
}

func TestGasPerIotas(t *testing.T) {
	evmChain := initEVMChain(t)
	storage := evmChain.deployStorageContract(evmChain.faucetKey, 42)

	// the default value is correct
	require.Equal(t, DefaultGasPerIota, evmChain.getGasPerIotas())

	res, err := storage.store(43)
	require.NoError(t, err)
	initialGasFee := res.iotaChargedFee

	// only the owner can call the setGasPerIotas endpoint
	newGasPerIota := DefaultGasPerIota * 1000
	newUserWallet, _ := evmChain.solo.NewKeyPairWithFunds()
	err = evmChain.setGasPerIotas(newGasPerIota, iotaCallOptions{wallet: newUserWallet})
	require.Contains(t, err.Error(), "can only be called by the contract owner")
	require.Equal(t, DefaultGasPerIota, evmChain.getGasPerIotas())

	// current owner is able to set a new gasPerIotas
	err = evmChain.setGasPerIotas(newGasPerIota, iotaCallOptions{wallet: evmChain.soloChain.OriginatorKeyPair})
	require.NoError(t, err)
	require.Equal(t, newGasPerIota, evmChain.getGasPerIotas())

	// run an equivalent request and compare the gas fees
	res, err = storage.store(44)
	require.NoError(t, err)
	require.Less(t, res.iotaChargedFee, initialGasFee)
}

func TestWithdrawalOwnerFees(t *testing.T) {
	evmChain := initEVMChain(t)
	storage := evmChain.deployStorageContract(evmChain.faucetKey, 42)

	// only the owner can call withdrawal
	user1Wallet, user1Address := evmChain.solo.NewKeyPairWithFunds()
	user1AgentID := iscp.NewAgentID(user1Address, 0)

	err := evmChain.withdrawGasFees(user1Wallet)
	require.Contains(t, err.Error(), "can only be called by the contract owner")

	// change owner to user1
	err = evmChain.setNextOwner(user1AgentID)
	require.NoError(t, err)
	err = evmChain.claimOwnership(iotaCallOptions{wallet: user1Wallet})
	require.NoError(t, err)

	// collect fees from contract deployment
	user1Balance0 := evmChain.solo.GetAddressBalance(user1Address, colored.IOTA)
	require.NoError(t, evmChain.withdrawGasFees(user1Wallet))
	user1Balance1 := evmChain.solo.GetAddressBalance(user1Address, colored.IOTA)
	require.Greater(t, user1Balance1, user1Balance0)

	// collect fees from a SC call, check that the collected fees matches the fees charged
	user1Balance2 := evmChain.solo.GetAddressBalance(user1Address, colored.IOTA)
	res, err := storage.store(43)
	require.NoError(t, err)
	require.NoError(t, evmChain.withdrawGasFees(user1Wallet))
	user1Balance3 := evmChain.solo.GetAddressBalance(user1Address, colored.IOTA)
	require.Equal(t, user1Balance3, user1Balance2+res.iotaChargedFee)

	// try to withdraw a second time, it should succeed, but owner balance shouldnt not change (there are no fees to withdraw)
	require.NoError(t, evmChain.withdrawGasFees(user1Wallet))
	user1Balance4 := evmChain.solo.GetAddressBalance(user1Address, colored.IOTA)
	require.Equal(t, user1Balance3, user1Balance4)

	// try to withdraw fees to another actor using using the FieldAgentId param
	res, err = storage.store(44)
	require.NoError(t, err)
	_, user2Address := evmChain.solo.NewKeyPairWithFunds()
	user2AgentID := iscp.NewAgentID(user2Address, 0)
	user2Balance0 := evmChain.solo.GetAddressBalance(user2Address, colored.IOTA)
	err = evmChain.withdrawGasFees(user1Wallet, user2AgentID)
	require.NoError(t, err)
	user2Balance1 := evmChain.solo.GetAddressBalance(user2Address, colored.IOTA)
	require.Equal(t, user2Balance1, user2Balance0+res.iotaChargedFee+1) // 1 extra iota from the withdrawal request
}

// tests that the gas limits are correctly enforced based on the iotas sent
func TestGasLimit(t *testing.T) {
	evmChain := initEVMChain(t)
	storage := evmChain.deployStorageContract(evmChain.faucetKey, 42)

	gasPerIotas := evmChain.getGasPerIotas()

	iotasForGas := uint64(10000)
	gaslimit := iotasForGas * gasPerIotas

	_, err := storage.store(123, ethCallOptions{gasLimit: gaslimit, iota: iotaCallOptions{transfer: iotasForGas - 1}})
	require.Contains(t, err.Error(), "transferred tokens (9999) not enough")

	_, err = storage.store(123, ethCallOptions{gasLimit: gaslimit, iota: iotaCallOptions{transfer: iotasForGas}})
	require.NoError(t, err)
}

// ensure the amount of iotas sent impacts the amount of loop iterators (gas used)
func TestLoop(t *testing.T) {
	evmChain := initEVMChain(t)
	loop := evmChain.deployLoopContract(evmChain.faucetKey)
	gasPerIotas := evmChain.getGasPerIotas()

	iotaWallet, iotaAddress := evmChain.solo.NewKeyPairWithFunds()
	iotaAgentID := iscp.NewAgentID(iotaAddress, 0)

	initialBalance := evmChain.solo.GetAddressBalance(iotaAddress, colored.IOTA)
	iotasSpent1 := uint64(100)
	res, err := loop.loop(ethCallOptions{
		gasLimit: iotasSpent1 * gasPerIotas,
		iota:     iotaCallOptions{wallet: iotaWallet, transfer: iotasSpent1},
	})
	require.NoError(t, err)
	require.Equal(t, res.iotaChargedFee, iotasSpent1)
	gasUsed := res.receipt.GasUsed

	iotasSpent2 := uint64(1000)
	res, err = loop.loop(ethCallOptions{
		gasLimit: iotasSpent2 * gasPerIotas,
		iota:     iotaCallOptions{wallet: iotaWallet, transfer: iotasSpent2},
	})
	require.NoError(t, err)
	require.Equal(t, res.iotaChargedFee, iotasSpent2)
	require.Greater(t, res.receipt.GasUsed, gasUsed)

	// ensure iotas sent are kept by the evmchain SC
	require.Equal(t, evmChain.solo.GetAddressBalance(iotaAddress, colored.IOTA), initialBalance-iotasSpent1-iotasSpent2)
	evmChain.soloChain.AssertIotas(iotaAgentID, 0)
}

func TestNonFaucetUsers(t *testing.T) {
	evmChain := initEVMChain(t)
	storage := evmChain.deployStorageContract(evmChain.faucetKey, 42)

	// call EVM contract with a new key (that doesn't own ether on the EVM evmChain.soloChain)
	gasPerIotas := evmChain.getGasPerIotas()
	iotas := uint64(10000)
	// this should be successful because gasPrice is 0
	res, err := storage.store(123, ethCallOptions{gasLimit: iotas * gasPerIotas, iota: iotaCallOptions{transfer: iotas}})
	require.NoError(t, err)
	require.Greater(t, res.iotaChargedFee, uint64(0))

	require.EqualValues(t, 123, storage.retrieve())
}

func TestPrePaidFees(t *testing.T) {
	evmChain := initEVMChain(t)
	storage := evmChain.deployStorageContract(evmChain.faucetKey, 42)

	iotaWallet, iotaAddress := evmChain.solo.NewKeyPairWithFunds()

	// test sending off-ledger request without depositing funds first
	txdata, _, _ := storage.buildEthTxData(nil, "store", uint32(999))
	offledgerRequest := evmChain.buildSoloRequest(FuncSendTransaction.Name, 100, FieldTransactionData, txdata)
	evmChain.soloChain.PostRequestOffLedger(offledgerRequest, iotaWallet)

	// check that the tx has no effect
	require.EqualValues(t, 42, storage.retrieve())

	// deposit funds
	initialBalance := evmChain.solo.GetAddressBalance(iotaAddress, colored.IOTA)
	_, err := evmChain.soloChain.PostRequestSync(
		solo.NewCallParams(accounts.Contract.Name, accounts.FuncDeposit.Name).WithIotas(initialBalance),
		iotaWallet,
	)
	require.NoError(t, err)

	// send offledger request again and check that is works
	evmChain.soloChain.PostRequestOffLedger(offledgerRequest, iotaWallet)
	require.EqualValues(t, 999, storage.retrieve())
}
