package test

import (
	"testing"
)

func Test2Chains(t *testing.T) {
	t.SkipNow()
	//run2(t, func(t *testing.T, w bool) {
	//	chain1 := wasmsolo.StartChain(t, "chain1")
	//	chain1.CheckAccountLedger()
	//
	//	chain2 := wasmsolo.StartChain(t, "chain2", chain1.Env)
	//	chain2.CheckAccountLedger()
	//
	//	user := wasmsolo.NewSoloAgent(chain1.Env)
	//	userL1 := user.Balance()
	//
	//	ctx1 := deployTestCoreOnChain(t, w, chain1, nil)
	//	require.NoError(t, ctx1.Err)
	//	ctx2 := deployTestCoreOnChain(t, w, chain2, nil)
	//	require.NoError(t, ctx2.Err)
	//
	//	bal1 := ctx1.Balances(user)
	//	bal2 := ctx2.Balances(user)
	//
	//	deposit(t, ctx1, user, ctx2.Account(), 1234)
	//	require.EqualValues(t, userL1-1234, user.Balance())
	//
	//	bal1.Chain += ctx1.GasFee
	//	bal1.Add(user, ctx1.GasFee)
	//	bal1.VerifyBalances(t)
	//
	//	bal2.Account += 1234
	//	bal2.VerifyBalances(t)
	//
	//	//f := testcore.ScFuncs.WithdrawToChain(ctx2.Sign(user))
	//	//f.Params.ChainID().SetValue(ctx1.ChainID())
	//	//f.Func.Post()
	//	//require.NoError(t, ctx2.Err)
	//	//
	//	//require.True(t, ctx1.WaitForPendingRequests(1))
	//	//require.True(t, ctx2.WaitForPendingRequests(1))
	//	//
	//	//require.EqualValues(t, utxodb.FundsFromFaucetAmount-42-1, user.Balance())
	//	//
	//	//t.Logf("dump chain1 accounts:\n%s", ctx1.Chain.DumpAccounts())
	//	//require.EqualValues(t, 0, ctx1.Balance(user))
	//	//require.EqualValues(t, 0, ctx1.Balance(ctx1.Account()))
	//	//require.EqualValues(t, 0+42-42, ctx1.Balance(ctx2.Account()))
	//	//chainAccountBalances(ctx1, w, 2, 2+42-42)
	//	//
	//	//t.Logf("dump chain2 accounts:\n%s", ctx2.Chain.DumpAccounts())
	//	//require.EqualValues(t, 0, ctx2.Balance(user))
	//	//require.EqualValues(t, 0, ctx2.Balance(ctx1.Account()))
	//	//require.EqualValues(t, 1+42, ctx2.Balance(ctx2.Account()))
	//	//chainAccountBalances(ctx2, w, 2, 2+1+42)
	//})
}
