package tests

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/wasp/client/chainclient"
	"github.com/iotaledger/wasp/contracts/native/inccounter"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/iscp/colored"
	"github.com/iotaledger/wasp/packages/solo"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/core/root"
	"github.com/iotaledger/wasp/tools/cluster"
	"github.com/stretchr/testify/require"
)

func TestBasicAccounts(t *testing.T) {
	e := setupWithNoChain(t)
	counter, err := e.clu.StartMessageCounter(map[string]int{
		"state":       2,
		"request_in":  0,
		"request_out": 1,
	})
	require.NoError(t, err)
	defer counter.Close()
	chain, err := e.clu.DeployDefaultChain()
	require.NoError(t, err)
	newChainEnv(t, e.clu, chain).testBasicAccounts(counter)
}

func TestBasicAccountsN1(t *testing.T) {
	e := setupWithNoChain(t)
	chainNodes := []int{0}
	counter, err := cluster.NewMessageCounter(e.clu, chainNodes, map[string]int{
		"state": 3,
	})
	require.NoError(t, err)
	defer counter.Close()
	chain, err := e.clu.DeployChainWithDKG("single_node_chain", chainNodes, chainNodes, 1)
	require.NoError(t, err)
	newChainEnv(t, e.clu, chain).testBasicAccounts(counter)
}

func (e *chainEnv) testBasicAccounts(counter *cluster.MessageCounter) {
	hname := iscp.Hn(incCounterSCName)
	description := "testing contract deployment with inccounter"
	programHash1 := inccounter.Contract.ProgramHash

	_, err := e.chain.DeployContract(incCounterSCName, programHash1.String(), description, map[string]interface{}{
		inccounter.VarCounter: 42,
		root.ParamName:        incCounterSCName,
	})
	require.NoError(e.t, err)

	if !counter.WaitUntilExpectationsMet() {
		e.t.Fail()
	}

	e.t.Logf("   %s: %s", root.Contract.Name, root.Contract.Hname().String())
	e.t.Logf("   %s: %s", accounts.Contract.Name, accounts.Contract.Hname().String())

	e.checkCoreContracts()

	for i := range e.chain.CommitteeNodes {
		blockIndex, err := e.chain.BlockIndex(i)
		require.NoError(e.t, err)
		require.EqualValues(e.t, 2, blockIndex)

		contractRegistry, err := e.chain.ContractRegistry(i)
		require.NoError(e.t, err)

		cr := contractRegistry[hname]

		require.EqualValues(e.t, programHash1, cr.ProgramHash)
		require.EqualValues(e.t, description, cr.Description)
		require.EqualValues(e.t, 0, cr.OwnerFee)
		require.EqualValues(e.t, incCounterSCName, cr.Name)

		counterValue, err := e.chain.GetCounterValue(hname, i)
		require.NoError(e.t, err)
		require.EqualValues(e.t, 42, counterValue)
	}

	if !e.clu.VerifyAddressBalances(e.chain.ChainID.AsAddress(), ledgerstate.DustThresholdAliasOutputIOTA+2,
		colored.NewBalancesForIotas(ledgerstate.DustThresholdAliasOutputIOTA+2), "chain after deployment") {
		e.t.Fail()
	}

	e.requestFunds(scOwnerAddr, "originator")

	transferIotas := uint64(42)
	chClient := chainclient.New(e.clu.GoshimmerClient(), e.clu.WaspClient(0), e.chain.ChainID, scOwner)

	par := chainclient.NewPostRequestParams().WithIotas(transferIotas)
	reqTx, err := chClient.Post1Request(hname, inccounter.FuncIncCounter.Hname(), *par)
	require.NoError(e.t, err)

	err = e.chain.CommitteeMultiClient().WaitUntilAllRequestsProcessed(e.chain.ChainID, reqTx, 10*time.Second)
	require.NoError(e.t, err)

	for i := range e.chain.CommitteeNodes {
		counterValue, err := e.chain.GetCounterValue(incCounterSCHname, i)
		require.NoError(e.t, err)
		require.EqualValues(e.t, 43, counterValue)
	}

	if !e.clu.VerifyAddressBalances(scOwnerAddr, solo.Saldo-transferIotas, colored.NewBalancesForIotas(solo.Saldo-transferIotas)) {
		e.t.Fail()
	}

	if !e.clu.VerifyAddressBalances(e.chain.ChainID.AsAddress(), ledgerstate.DustThresholdAliasOutputIOTA+transferIotas+2,
		colored.NewBalancesForIotas(ledgerstate.DustThresholdAliasOutputIOTA+transferIotas+2), "chain after") {
		e.t.Fail()
	}
	agentID := iscp.NewAgentID(e.chain.ChainID.AsAddress(), hname)
	actual := e.getBalanceOnChain(agentID, colored.IOTA)
	require.EqualValues(e.t, 42, actual)
}

func TestBasic2Accounts(t *testing.T) {
	e := setupWithNoChain(t)

	counter, err := e.clu.StartMessageCounter(map[string]int{
		"state":       2,
		"request_in":  0,
		"request_out": 1,
	})
	require.NoError(t, err)
	defer counter.Close()

	chain, err := e.clu.DeployDefaultChain()
	require.NoError(t, err)

	chEnv := newChainEnv(t, e.clu, chain)

	hname := iscp.Hn(incCounterSCName)
	description := "testing contract deployment with inccounter"
	programHash1 := inccounter.Contract.ProgramHash
	require.NoError(t, err)

	_, err = chain.DeployContract(incCounterSCName, programHash1.String(), description, map[string]interface{}{
		inccounter.VarCounter: 42,
		root.ParamName:        incCounterSCName,
	})
	require.NoError(t, err)

	if !counter.WaitUntilExpectationsMet() {
		t.Fail()
	}

	chEnv.checkCoreContracts()

	for _, i := range chain.CommitteeNodes {
		blockIndex, err := chain.BlockIndex(i)
		require.NoError(t, err)
		require.EqualValues(t, 2, blockIndex)

		contractRegistry, err := chain.ContractRegistry(i)
		require.NoError(t, err)

		t.Logf("%+v", contractRegistry)
		cr := contractRegistry[hname]

		require.EqualValues(t, programHash1, cr.ProgramHash)
		require.EqualValues(t, description, cr.Description)
		require.EqualValues(t, 0, cr.OwnerFee)
		require.EqualValues(t, incCounterSCName, cr.Name)

		counterValue, err := chain.GetCounterValue(hname, i)
		require.NoError(t, err)
		require.EqualValues(t, 42, counterValue)
	}

	if !e.clu.VerifyAddressBalances(chain.ChainID.AsAddress(), ledgerstate.DustThresholdAliasOutputIOTA+2,
		colored.NewBalancesForIotas(ledgerstate.DustThresholdAliasOutputIOTA+2), "chain after deployment") {
		t.Fail()
	}

	originatorSigScheme := chain.OriginatorKeyPair()
	originatorAddress := chain.OriginatorAddress()

	if !e.clu.VerifyAddressBalances(originatorAddress, solo.Saldo-ledgerstate.DustThresholdAliasOutputIOTA-2,
		colored.NewBalancesForIotas(solo.Saldo-ledgerstate.DustThresholdAliasOutputIOTA-2),
		"originator after deployment") {
		t.Fail()
	}
	chEnv.checkLedger()

	myWallet := wallet.KeyPair(3)
	myWalletAddr := ledgerstate.NewED25519Address(myWallet.PublicKey)

	e.requestFunds(myWalletAddr, "myWalletAddress")

	transferIotas := uint64(42)
	myWalletClient := chainclient.New(e.clu.GoshimmerClient(), e.clu.WaspClient(0), chain.ChainID, myWallet)

	par := chainclient.NewPostRequestParams().WithIotas(transferIotas)
	reqTx, err := myWalletClient.Post1Request(hname, inccounter.FuncIncCounter.Hname(), *par)
	require.NoError(t, err)

	err = chain.CommitteeMultiClient().WaitUntilAllRequestsProcessed(chain.ChainID, reqTx, 30*time.Second)
	require.NoError(t, err)
	chEnv.checkLedger()

	for _, i := range chain.CommitteeNodes {
		counterValue, err := chain.GetCounterValue(hname, i)
		require.NoError(t, err)
		require.EqualValues(t, 43, counterValue)
	}
	if !e.clu.VerifyAddressBalances(originatorAddress, solo.Saldo-ledgerstate.DustThresholdAliasOutputIOTA-2,
		colored.NewBalancesForIotas(solo.Saldo-ledgerstate.DustThresholdAliasOutputIOTA-2),
		"originator after") {
		t.Fail()
	}
	if !e.clu.VerifyAddressBalances(myWalletAddr, solo.Saldo-transferIotas, colored.NewBalancesForIotas(solo.Saldo-transferIotas), "myWalletAddr after") {
		t.Fail()
	}
	if !e.clu.VerifyAddressBalances(chain.ChainID.AsAddress(), ledgerstate.DustThresholdAliasOutputIOTA+2+transferIotas,
		colored.NewBalancesForIotas(ledgerstate.DustThresholdAliasOutputIOTA+2+transferIotas),
		"chain after") {
		t.Fail()
	}
	// verify and print chain accounts
	agentID := iscp.NewAgentID(chain.ChainID.AsAddress(), hname)
	actual := chEnv.getBalanceOnChain(agentID, colored.IOTA)
	require.EqualValues(t, 42, actual)

	chEnv.printAccounts("withdraw before")

	// withdraw back 2 iotas to originator address
	fmt.Printf("\norig address from sigsheme: %s\n", originatorAddress.Base58())
	originatorClient := chainclient.New(e.clu.GoshimmerClient(), e.clu.WaspClient(0), chain.ChainID, originatorSigScheme)
	reqTx2, err := originatorClient.Post1Request(accounts.Contract.Hname(), accounts.FuncWithdraw.Hname())
	require.NoError(t, err)

	err = chain.CommitteeMultiClient().WaitUntilAllRequestsProcessed(chain.ChainID, reqTx2, 30*time.Second)
	require.NoError(t, err)

	chEnv.checkLedger()

	chEnv.printAccounts("withdraw after")

	// must remain 0 on chain
	agentID = iscp.NewAgentID(originatorAddress, 0)
	actual = chEnv.getBalanceOnChain(agentID, colored.IOTA)
	require.EqualValues(t, 0, actual)

	if !e.clu.VerifyAddressBalances(originatorAddress, solo.Saldo-ledgerstate.DustThresholdAliasOutputIOTA-3,
		colored.NewBalancesForIotas(solo.Saldo-ledgerstate.DustThresholdAliasOutputIOTA-3),
		"originator after withdraw: "+originatorAddress.String()) {
		t.Fail()
	}
}
