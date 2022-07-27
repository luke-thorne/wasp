package tests

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/wasp/client/chainclient"
	"github.com/iotaledger/wasp/contracts/native/inccounter"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/utxodb"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/core/root"
	"github.com/iotaledger/wasp/tools/cluster"
	"github.com/stretchr/testify/require"
)

func TestBasicAccounts(t *testing.T) {
	e := setupWithNoChain(t)
	counter, err := e.Clu.StartMessageCounter(map[string]int{
		"state":       2,
		"request_in":  0,
		"request_out": 1,
	})
	require.NoError(t, err)
	defer counter.Close()
	chain, err := e.Clu.DeployDefaultChain()
	require.NoError(t, err)
	newChainEnv(t, e.Clu, chain).testBasicAccounts(counter)
}

func TestBasicAccountsNLow(t *testing.T) {
	runTest := func(tt *testing.T, n, t int) {
		e := setupWithNoChain(tt)
		chainNodes := make([]int, n)
		for i := range chainNodes {
			chainNodes[i] = i
		}
		counter, err := cluster.NewMessageCounter(e.Clu, chainNodes, map[string]int{
			"state": 3,
		})
		require.NoError(tt, err)
		defer counter.Close()
		chain, err := e.Clu.DeployChainWithDKG(fmt.Sprintf("low_node_chain_%v_%v", n, t), chainNodes, chainNodes, uint16(t))
		require.NoError(tt, err)
		newChainEnv(tt, e.Clu, chain).testBasicAccounts(counter)
	}
	t.Run("N=1", func(tt *testing.T) { runTest(tt, 1, 1) })
	t.Run("N=2", func(tt *testing.T) { runTest(tt, 2, 2) })
	t.Run("N=3", func(tt *testing.T) { runTest(tt, 3, 3) })
	t.Run("N=4", func(tt *testing.T) { runTest(tt, 4, 3) })
}

func (e *ChainEnv) testBasicAccounts(counter *cluster.MessageCounter) {
	hname := iscp.Hn(nativeIncCounterSCName)
	description := "testing contract deployment with inccounter"
	programHash1 := inccounter.Contract.ProgramHash

	_, err := e.Chain.DeployContract(nativeIncCounterSCName, programHash1.String(), description, map[string]interface{}{
		inccounter.VarCounter: 42,
		root.ParamName:        nativeIncCounterSCName,
	})
	require.NoError(e.t, err)

	if !counter.WaitUntilExpectationsMet() {
		e.t.Fatal()
	}

	e.t.Logf("   %s: %s", root.Contract.Name, root.Contract.Hname().String())
	e.t.Logf("   %s: %s", accounts.Contract.Name, accounts.Contract.Hname().String())

	e.checkCoreContracts()

	for i := range e.Chain.CommitteeNodes {
		blockIndex, err := e.Chain.BlockIndex(i)
		require.NoError(e.t, err)
		require.Greater(e.t, blockIndex, uint32(2))

		contractRegistry, err := e.Chain.ContractRegistry(i)
		require.NoError(e.t, err)

		cr := contractRegistry[hname]

		require.EqualValues(e.t, programHash1, cr.ProgramHash)
		require.EqualValues(e.t, description, cr.Description)
		require.EqualValues(e.t, nativeIncCounterSCName, cr.Name)

		counterValue, err := e.Chain.GetCounterValue(hname, i)
		require.NoError(e.t, err)
		require.EqualValues(e.t, 42, counterValue)
	}

	myWallet, myAddress, err := e.Clu.NewKeyPairWithFunds()
	require.NoError(e.t, err)

	transferBaseTokens := 1 * iscp.Mi
	chClient := chainclient.New(e.Clu.L1Client(), e.Clu.WaspClient(0), e.Chain.ChainID, myWallet)

	par := chainclient.NewPostRequestParams().WithBaseTokens(transferBaseTokens)
	reqTx, err := chClient.Post1Request(hname, inccounter.FuncIncCounter.Hname(), *par)
	require.NoError(e.t, err)

	receipts, err := e.Chain.CommitteeMultiClient().WaitUntilAllRequestsProcessedSuccessfully(e.Chain.ChainID, reqTx, 10*time.Second)
	require.NoError(e.t, err)

	fees := receipts[0].GasFeeCharged
	e.checkBalanceOnChain(iscp.NewAgentID(myAddress), iscp.BaseTokenID, transferBaseTokens-fees)

	for i := range e.Chain.CommitteeNodes {
		counterValue, err := e.Chain.GetCounterValue(nativeIncCounterSCHname, i)
		require.NoError(e.t, err)
		require.EqualValues(e.t, 43, counterValue)
	}

	if !e.Clu.AssertAddressBalances(myAddress, iscp.NewFungibleBaseTokens(utxodb.FundsFromFaucetAmount-transferBaseTokens)) {
		e.t.Fatal()
	}

	incCounterAgentID := iscp.NewContractAgentID(e.Chain.ChainID, hname)
	e.checkBalanceOnChain(incCounterAgentID, iscp.BaseTokenID, 0)
}

func TestBasic2Accounts(t *testing.T) {
	e := setupWithNoChain(t)

	counter, err := e.Clu.StartMessageCounter(map[string]int{
		"state":       2,
		"request_in":  0,
		"request_out": 1,
	})
	require.NoError(t, err)
	defer counter.Close()

	chain, err := e.Clu.DeployDefaultChain()
	require.NoError(t, err)

	chEnv := newChainEnv(t, e.Clu, chain)

	hname := iscp.Hn(nativeIncCounterSCName)
	description := "testing contract deployment with inccounter"
	programHash1 := inccounter.Contract.ProgramHash
	require.NoError(t, err)

	_, err = chain.DeployContract(nativeIncCounterSCName, programHash1.String(), description, map[string]interface{}{
		inccounter.VarCounter: 42,
		root.ParamName:        nativeIncCounterSCName,
	})
	require.NoError(t, err)

	if !counter.WaitUntilExpectationsMet() {
		t.Fatal()
	}

	chEnv.checkCoreContracts()

	for _, i := range chain.CommitteeNodes {
		blockIndex, err := chain.BlockIndex(i)
		require.NoError(t, err)
		require.Greater(t, blockIndex, uint32(2))

		contractRegistry, err := chain.ContractRegistry(i)
		require.NoError(t, err)

		t.Logf("%+v", contractRegistry)
		cr := contractRegistry[hname]

		require.EqualValues(t, programHash1, cr.ProgramHash)
		require.EqualValues(t, description, cr.Description)
		require.EqualValues(t, nativeIncCounterSCName, cr.Name)

		counterValue, err := chain.GetCounterValue(hname, i)
		require.NoError(t, err)
		require.EqualValues(t, 42, counterValue)
	}

	originatorSigScheme := chain.OriginatorKeyPair
	originatorAddress := chain.OriginatorAddress()

	chEnv.checkLedger()

	myWallet, myAddress, err := e.Clu.NewKeyPairWithFunds()
	require.NoError(t, err)

	transferBaseTokens := 1 * iscp.Mi
	myWalletClient := chainclient.New(e.Clu.L1Client(), e.Clu.WaspClient(0), chain.ChainID, myWallet)

	par := chainclient.NewPostRequestParams().WithBaseTokens(transferBaseTokens)
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
	if !e.Clu.AssertAddressBalances(myAddress, iscp.NewFungibleBaseTokens(utxodb.FundsFromFaucetAmount-transferBaseTokens)) {
		t.Fatal()
	}

	chEnv.printAccounts("withdraw before")

	// withdraw back 500 base tokens to originator address
	fmt.Printf("\norig address from sigsheme: %s\n", originatorAddress.Bech32(parameters.L1.Protocol.Bech32HRP))
	origL1Balance := e.Clu.AddressBalances(originatorAddress).BaseTokens
	originatorClient := chainclient.New(e.Clu.L1Client(), e.Clu.WaspClient(0), chain.ChainID, originatorSigScheme)
	allowanceBaseTokens := uint64(800_000)
	req2, err := originatorClient.PostOffLedgerRequest(accounts.Contract.Hname(), accounts.FuncWithdraw.Hname(),
		chainclient.PostRequestParams{
			Allowance: iscp.NewAllowanceBaseTokens(allowanceBaseTokens),
		},
	)
	require.NoError(t, err)

	_, err = chain.CommitteeMultiClient().WaitUntilRequestProcessedSuccessfully(chain.ChainID, req2.ID(), 30*time.Second)
	require.NoError(t, err)

	chEnv.checkLedger()

	chEnv.printAccounts("withdraw after")

	require.Equal(t, e.Clu.AddressBalances(originatorAddress).BaseTokens, origL1Balance+allowanceBaseTokens)
}
