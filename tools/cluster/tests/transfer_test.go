package tests

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/wasp/client/chainclient"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/solo"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/stretchr/testify/require"
)

func TestDepositWithdraw(t *testing.T) {
	e := setupWithNoChain(t)

	chain, err := e.clu.DeployDefaultChain()
	require.NoError(t, err)

	chEnv := newChainEnv(t, e.clu, chain)

	testOwner := wallet.KeyPair(1)
	myAddress := ledgerstate.NewED25519Address(testOwner.PublicKey)

	e.requestFunds(myAddress, "myAddress")
	if !e.clu.VerifyAddressBalances(myAddress, solo.Saldo, map[ledgerstate.Color]uint64{
		ledgerstate.ColorIOTA: solo.Saldo,
	}, "myAddress begin") {
		t.Fail()
	}
	if !e.clu.VerifyAddressBalances(chain.OriginatorAddress(), solo.Saldo-ledgerstate.DustThresholdAliasOutputIOTA-1, map[ledgerstate.Color]uint64{
		ledgerstate.ColorIOTA: solo.Saldo - ledgerstate.DustThresholdAliasOutputIOTA - 1,
	}, "originatorAddress begin") {
		t.Fail()
	}
	if !e.clu.VerifyAddressBalances(chain.ChainAddress(), ledgerstate.DustThresholdAliasOutputIOTA+1, map[ledgerstate.Color]uint64{
		ledgerstate.ColorIOTA: ledgerstate.DustThresholdAliasOutputIOTA + 1,
	}, "chainAddress begin") {
		t.Fail()
	}
	chEnv.checkLedger()

	myAgentID := iscp.NewAgentID(myAddress, 0)
	origAgentID := iscp.NewAgentID(chain.OriginatorAddress(), 0)

	chEnv.checkBalanceOnChain(origAgentID, ledgerstate.ColorIOTA, 0)
	chEnv.checkBalanceOnChain(myAgentID, ledgerstate.ColorIOTA, 0)
	chEnv.checkLedger()

	// deposit some iotas to the chain
	depositIotas := uint64(42)
	chClient := chainclient.New(e.clu.GoshimmerClient(), e.clu.WaspClient(0), chain.ChainID, testOwner)

	par := chainclient.NewPostRequestParams().WithIotas(depositIotas)
	reqTx, err := chClient.Post1Request(accounts.Contract.Hname(), accounts.FuncDeposit.Hname(), *par)
	require.NoError(t, err)

	err = chain.CommitteeMultiClient().WaitUntilAllRequestsProcessed(chain.ChainID, reqTx, 30*time.Second)
	require.NoError(t, err)
	chEnv.checkLedger()
	chEnv.checkBalanceOnChain(myAgentID, ledgerstate.ColorIOTA, depositIotas)
	chEnv.checkBalanceOnChain(origAgentID, ledgerstate.ColorIOTA, 0)

	if !e.clu.VerifyAddressBalances(myAddress, solo.Saldo-depositIotas, map[ledgerstate.Color]uint64{
		ledgerstate.ColorIOTA: solo.Saldo - depositIotas,
	}, "myAddress after deposit") {
		t.Fail()
	}

	// withdraw iotas back
	reqTx3, err := chClient.Post1Request(accounts.Contract.Hname(), accounts.FuncWithdraw.Hname())
	require.NoError(t, err)
	err = chain.CommitteeMultiClient().WaitUntilAllRequestsProcessed(chain.ChainID, reqTx3, 30*time.Second)
	require.NoError(t, err)

	require.NoError(t, err)
	chEnv.checkLedger()
	chEnv.checkBalanceOnChain(myAgentID, ledgerstate.ColorIOTA, 0)

	if !e.clu.VerifyAddressBalances(myAddress, solo.Saldo-1, map[ledgerstate.Color]uint64{
		ledgerstate.ColorIOTA: solo.Saldo - 1,
	}, "myAddress after withdraw") {
		t.Fail()
	}
}
