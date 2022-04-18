package tests

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/utxodb"

	"github.com/iotaledger/wasp/client/chainclient"
	"github.com/iotaledger/wasp/contracts/native/inccounter"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/core/root"
	"github.com/iotaledger/wasp/tools/cluster"
	"github.com/stretchr/testify/require"
)

const someIotas = 1000

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

func TestBasicAccountsNLow(t *testing.T) {
	runTest := func(tt *testing.T, n, t int) {
		e := setupWithNoChain(tt)
		chainNodes := make([]int, n)
		for i := range chainNodes {
			chainNodes[i] = i
		}
		counter, err := cluster.NewMessageCounter(e.clu, chainNodes, map[string]int{
			"state": 3,
		})
		require.NoError(tt, err)
		defer counter.Close()
		chain, err := e.clu.DeployChainWithDKG(fmt.Sprintf("low_node_chain_%v_%v", n, t), chainNodes, chainNodes, uint16(t))
		require.NoError(tt, err)
		newChainEnv(tt, e.clu, chain).testBasicAccounts(counter)
	}
	t.Run("N=1", func(tt *testing.T) { runTest(tt, 1, 1) })
	t.Run("N=2", func(tt *testing.T) { runTest(tt, 2, 2) })
	t.Run("N=3", func(tt *testing.T) { runTest(tt, 3, 3) })
	t.Run("N=4", func(tt *testing.T) { runTest(tt, 4, 3) })
}

func (e *chainEnv) testBasicAccounts(counter *cluster.MessageCounter) {
	chainNodeCount := uint64(len(e.chain.AllPeers))
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
		require.EqualValues(e.t, incCounterSCName, cr.Name)

		counterValue, err := e.chain.GetCounterValue(hname, i)
		require.NoError(e.t, err)
		require.EqualValues(e.t, 42, counterValue)
	}

	if !e.clu.AssertAddressBalances(e.chain.ChainID.AsAddress(),
		iscp.NewTokensIotas(someIotas+2+chainNodeCount)) {
		e.t.Fail()
	}

	e.requestFunds(scOwnerAddr, "originator")

	transferIotas := uint64(42)
	chClient := chainclient.New(e.clu.L1Client(), e.clu.WaspClient(0), e.chain.ChainID, scOwner)

	par := chainclient.NewPostRequestParams().WithIotas(transferIotas)
	reqTx, err := chClient.Post1Request(hname, inccounter.FuncIncCounter.Hname(), *par)
	require.NoError(e.t, err)

	_, err = e.chain.CommitteeMultiClient().WaitUntilAllRequestsProcessedSuccessfully(e.chain.ChainID, reqTx, 10*time.Second)
	require.NoError(e.t, err)

	for i := range e.chain.CommitteeNodes {
		counterValue, err := e.chain.GetCounterValue(incCounterSCHname, i)
		require.NoError(e.t, err)
		require.EqualValues(e.t, 43, counterValue)
	}

	if !e.clu.AssertAddressBalances(scOwnerAddr, iscp.NewTokensIotas(utxodb.FundsFromFaucetAmount-transferIotas)) {
		e.t.Fail()
	}

	if !e.clu.AssertAddressBalances(e.chain.ChainID.AsAddress(),
		iscp.NewTokensIotas(someIotas+transferIotas+2+chainNodeCount)) {
		e.t.Fail()
	}
	agentID := iscp.NewAgentID(e.chain.ChainID.AsAddress(), hname)
	actual := e.getBalanceOnChain(agentID, iscp.IotaTokenID)
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
	chainNodeCount := uint64(len(chain.AllPeers))

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
		require.EqualValues(t, incCounterSCName, cr.Name)

		counterValue, err := chain.GetCounterValue(hname, i)
		require.NoError(t, err)
		require.EqualValues(t, 42, counterValue)
	}

	if !e.clu.AssertAddressBalances(chain.ChainID.AsAddress(),
		iscp.NewTokensIotas(someIotas+2+chainNodeCount)) {
		t.Fail()
	}

	originatorSigScheme := chain.OriginatorKeyPair
	originatorAddress := chain.OriginatorAddress()

	if !e.clu.AssertAddressBalances(originatorAddress,
		iscp.NewTokensIotas(utxodb.FundsFromFaucetAmount-someIotas-2-chainNodeCount)) {
		t.Fail()
	}
	chEnv.checkLedger()

	myWallet := cryptolib.NewKeyPairFromSeed(wallet.SubSeed(3))
	myWalletAddr := myWallet.Address()

	e.requestFunds(myWalletAddr, "myWalletAddress")

	transferIotas := uint64(42)
	myWalletClient := chainclient.New(e.clu.L1Client(), e.clu.WaspClient(0), chain.ChainID, myWallet)

	par := chainclient.NewPostRequestParams().WithIotas(transferIotas)
	reqTx, err := myWalletClient.Post1Request(hname, inccounter.FuncIncCounter.Hname(), *par)
	require.NoError(t, err)

	_, err = chain.CommitteeMultiClient().WaitUntilAllRequestsProcessedSuccessfully(chain.ChainID, reqTx, 30*time.Second)
	require.NoError(t, err)
	chEnv.checkLedger()

	for _, i := range chain.CommitteeNodes {
		counterValue, err := chain.GetCounterValue(hname, i)
		require.NoError(t, err)
		require.EqualValues(t, 43, counterValue)
	}
	if !e.clu.AssertAddressBalances(originatorAddress,
		iscp.NewTokensIotas(utxodb.FundsFromFaucetAmount-someIotas-2-chainNodeCount)) {
		t.Fail()
	}
	if !e.clu.AssertAddressBalances(myWalletAddr, iscp.NewTokensIotas(utxodb.FundsFromFaucetAmount-transferIotas)) {
		t.Fail()
	}
	if !e.clu.AssertAddressBalances(chain.ChainID.AsAddress(),
		iscp.NewTokensIotas(someIotas+2+transferIotas+chainNodeCount)) {
		t.Fail()
	}
	// verify and print chain accounts
	agentID := iscp.NewAgentID(chain.ChainID.AsAddress(), hname)
	actual := chEnv.getBalanceOnChain(agentID, iscp.IotaTokenID)
	require.EqualValues(t, 42, actual)

	chEnv.printAccounts("withdraw before")

	// withdraw back 2 iotas to originator address
	fmt.Printf("\norig address from sigsheme: %s\n", originatorAddress.Bech32(e.clu.L1Client().L1Params().Bech32Prefix))
	originatorClient := chainclient.New(e.clu.L1Client(), e.clu.WaspClient(0), chain.ChainID, originatorSigScheme)
	reqTx2, err := originatorClient.Post1Request(accounts.Contract.Hname(), accounts.FuncWithdraw.Hname())
	require.NoError(t, err)

	_, err = chain.CommitteeMultiClient().WaitUntilAllRequestsProcessedSuccessfully(chain.ChainID, reqTx2, 30*time.Second)
	require.NoError(t, err)

	chEnv.checkLedger()

	chEnv.printAccounts("withdraw after")

	// must remain 0 on chain
	agentID = iscp.NewAgentID(originatorAddress, 0)
	actual = chEnv.getBalanceOnChain(agentID, iscp.IotaTokenID)
	require.EqualValues(t, 0, actual)

	if !e.clu.AssertAddressBalances(originatorAddress,
		iscp.NewTokensIotas(utxodb.FundsFromFaucetAmount-someIotas-3-chainNodeCount)) {
		t.Fail()
	}
}
