package testcore

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotaledger/hive.go/logger"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/tpkg"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/solo"
	"github.com/iotaledger/wasp/packages/testutil/testmisc"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/utxodb"
	"github.com/iotaledger/wasp/packages/vm"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/stretchr/testify/require"
)

const IotasDepositFee = 100

func TestDeposit(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustDustDeposit: true})
	sender, _ := env.NewKeyPairWithFunds(env.NewSeedFromIndex(11))
	ch := env.NewChain(nil, "chain1")

	err := ch.DepositIotasToL2(100_000, sender)
	require.NoError(t, err)

	rec := ch.LastReceipt()
	t.Logf("========= receipt: %s", rec)
	t.Logf("========= burn log:\n%s", rec.GasBurnLog)
}

func TestHarvest(t *testing.T) {
	env := solo.New(t)
	ch, _, _ := env.NewChainExt(nil, 10_000, "chain1")
	_ = ch.Log().Sync()

	t.Logf("common iotas BEFORE: %d", ch.L2CommonAccountIotas())
	err := ch.DepositIotasToL2(100_000, nil)
	require.NoError(t, err)
	userAgentID := ch.OriginatorAgentID
	t.Logf("userAgentID iotas: %d", ch.L2Iotas(userAgentID))

	_, err = ch.PostRequestSync(
		solo.NewCallParams(
			accounts.Contract.Name,
			accounts.FuncHarvest.Name).
			AddIotas(10_000).
			WithGasBudget(100_000),
		nil)
	require.NoError(t, err)
	t.Logf("common iotas AFTER: %d", ch.L2CommonAccountIotas())
	require.True(t, ch.L2CommonAccountIotas() > accounts.MinimumIotasOnCommonAccount)
}

// allowance shouldn't allow you to bypass gas fees.
func TestDepositCheatAllowance(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustDustDeposit: false})
	sender, senderAddr := env.NewKeyPairWithFunds(env.NewSeedFromIndex(11))
	senderAgentID := iscp.NewAgentID(senderAddr)
	ch := env.NewChain(nil, "chain1")

	const iotasSent = 1 * iscp.Mi

	// send a request where allowance == assets - so that no iotas are available outside allowance
	_, err := ch.PostRequestSync(
		solo.NewCallParams(accounts.Contract.Name, accounts.FuncDeposit.Name).
			AddIotas(iotasSent).
			WithGasBudget(100_000).
			AddAllowanceIotas(iotasSent),
		sender,
	)
	require.Error(t, err)

	rec := ch.LastReceipt()
	finalBalance := ch.L2Iotas(senderAgentID)
	require.Less(t, finalBalance, iotasSent)
	require.EqualValues(t, iotasSent, finalBalance+rec.GasFeeCharged)
}

func TestWithdrawEverything(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustDustDeposit: true})
	sender, senderAddr := env.NewKeyPairWithFunds(env.NewSeedFromIndex(11))
	senderAgentID := iscp.NewAgentID(senderAddr)
	ch := env.NewChain(nil, "chain1")

	// deposit some iotas to L2
	initialL1balance := ch.Env.L1Iotas(senderAddr)
	iotasToDepositToL2 := uint64(100_000)
	err := ch.DepositIotasToL2(iotasToDepositToL2, sender)
	require.NoError(t, err)

	depositGasFee := ch.LastReceipt().GasFeeCharged
	l2balance := ch.L2Iotas(senderAgentID)

	// construct request with low allowance (just sufficient for dust balance), so its possible to estimate the gas fees
	req := solo.NewCallParams(accounts.Contract.Name, accounts.FuncWithdraw.Name).
		WithFungibleTokens(iscp.NewTokensIotas(l2balance)).AddAllowance(iscp.NewAllowanceIotas(5200))

	gasEstimate, fee, err := ch.EstimateGasOffLedger(req, sender, true)
	require.NoError(t, err)

	// set the allowance to the maximum possible value
	req = req.WithAllowance(iscp.NewAllowanceIotas(l2balance - fee)).
		WithGasBudget(gasEstimate)

	_, err = ch.PostRequestOffLedger(req, sender)
	require.NoError(t, err)

	withdrawalGasFee := ch.LastReceipt().GasFeeCharged
	finalL1Balance := ch.Env.L1Iotas(senderAddr)
	finalL2Balance := ch.L2Iotas(senderAgentID)

	// ensure everything was withdrawn
	require.Equal(t, initialL1balance, finalL1Balance+depositGasFee+withdrawalGasFee)
	require.Zero(t, finalL2Balance)
}

func TestFoundries(t *testing.T) {
	var env *solo.Solo
	var ch *solo.Chain
	var senderKeyPair *cryptolib.KeyPair
	var senderAddr iotago.Address
	var senderAgentID iscp.AgentID

	initTest := func() {
		env = solo.New(t, &solo.InitOptions{AutoAdjustDustDeposit: true})
		ch, _, _ = env.NewChainExt(nil, 10*iscp.Mi, "chain1")
		defer func(log *logger.Logger) {
			err := log.Sync()
			if err != nil {
			}
		}(ch.Log())

		senderKeyPair, senderAddr = env.NewKeyPairWithFunds(env.NewSeedFromIndex(10))
		senderAgentID = iscp.NewAgentID(senderAddr)

		ch.MustDepositIotasToL2(10*iscp.Mi, senderKeyPair)
	}
	t.Run("newFoundry fails when no allowance is provided", func(t *testing.T) {
		env = solo.New(t, &solo.InitOptions{AutoAdjustDustDeposit: true})
		ch, _, _ = env.NewChainExt(nil, 100_000, "chain1")

		req := solo.NewCallParams(accounts.Contract.Name, accounts.FuncFoundryCreateNew.Name,
			accounts.ParamTokenScheme, codec.EncodeTokenScheme(
				&iotago.SimpleTokenScheme{MaximumSupply: big.NewInt(1), MintedTokens: util.Big0, MeltedTokens: util.Big0},
			),
		).AddIotas(2 * iscp.Mi).WithGasBudget(math.MaxUint64)
		_, err := ch.PostRequestSync(req, nil)
		require.Error(t, err)
		// it succeeds when allowance is added
		_, err = ch.PostRequestSync(req.AddAllowanceIotas(1*iscp.Mi), nil)
		require.NoError(t, err)
	})
	t.Run("newFoundry overrides bad melted/minted token counters in tokenscheme", func(t *testing.T) {
		env = solo.New(t, &solo.InitOptions{AutoAdjustDustDeposit: true})
		ch, _, _ = env.NewChainExt(nil, 100_000, "chain1")

		req := solo.NewCallParams(accounts.Contract.Name, accounts.FuncFoundryCreateNew.Name,
			accounts.ParamTokenScheme, codec.EncodeTokenScheme(
				&iotago.SimpleTokenScheme{MaximumSupply: big.NewInt(1), MintedTokens: big.NewInt(10), MeltedTokens: big.NewInt(10)},
			),
		).AddIotas(2 * iscp.Mi).WithGasBudget(math.MaxUint64)
		_, err := ch.PostRequestSync(req.AddAllowanceIotas(1*iscp.Mi), nil)
		require.NoError(t, err)
	})
	t.Run("supply 10", func(t *testing.T) {
		initTest()
		sn, _, err := ch.NewFoundryParams(10).
			WithUser(senderKeyPair).
			CreateFoundry()
		require.NoError(t, err)
		require.EqualValues(t, 1, int(sn))
	})
	t.Run("supply 1", func(t *testing.T) {
		initTest()
		sn, _, err := ch.NewFoundryParams(1).
			WithUser(senderKeyPair).
			CreateFoundry()
		require.NoError(t, err)
		require.EqualValues(t, 1, sn)
	})
	t.Run("supply 0", func(t *testing.T) {
		initTest()
		_, _, err := ch.NewFoundryParams(0).
			WithUser(senderKeyPair).
			CreateFoundry()
		testmisc.RequireErrorToBe(t, err, vm.ErrCreateFoundryMaxSupplyMustBePositive)
	})
	t.Run("supply negative", func(t *testing.T) {
		initTest()
		require.Panics(t, func() {
			_, _, _ = ch.NewFoundryParams(-1).
				WithUser(senderKeyPair).
				CreateFoundry()
		})
	})
	t.Run("supply max possible", func(t *testing.T) {
		initTest()
		sn, _, err := ch.NewFoundryParams(abi.MaxUint256).
			WithUser(senderKeyPair).
			CreateFoundry()
		require.NoError(t, err)
		require.EqualValues(t, 1, sn)
	})
	t.Run("supply exceed max possible", func(t *testing.T) {
		initTest()
		maxSupply := new(big.Int).Set(util.MaxUint256)
		maxSupply.Add(maxSupply, big.NewInt(1))
		require.Panics(t, func() {
			_, _, _ = ch.NewFoundryParams(maxSupply).CreateFoundry()
		})
	})
	// TODO cover all parameter options

	t.Run("max supply 10, mintTokens 5", func(t *testing.T) {
		initTest()
		sn, tokenID, err := ch.NewFoundryParams(10).
			WithUser(senderKeyPair).
			CreateFoundry()
		require.NoError(t, err)
		require.EqualValues(t, 1, sn)
		ch.AssertL2NativeTokens(senderAgentID, &tokenID, util.Big0)
		ch.AssertL2TotalNativeTokens(&tokenID, util.Big0)

		err = ch.SendFromL1ToL2AccountIotas(IotasDepositFee, 1000, ch.CommonAccount(), senderKeyPair)
		require.NoError(t, err)
		t.Logf("common account iotas = %d before mint", ch.L2CommonAccountIotas())

		err = ch.MintTokens(sn, big.NewInt(5), senderKeyPair)
		require.NoError(t, err)

		ch.AssertL2NativeTokens(senderAgentID, &tokenID, big.NewInt(5))
		ch.AssertL2TotalNativeTokens(&tokenID, big.NewInt(5))
	})
	t.Run("max supply 1, mintTokens 1", func(t *testing.T) {
		initTest()
		sn, tokenID, err := ch.NewFoundryParams(1).
			WithUser(senderKeyPair).
			CreateFoundry()
		require.NoError(t, err)
		require.EqualValues(t, 1, sn)
		ch.AssertL2NativeTokens(senderAgentID, &tokenID, util.Big0)
		ch.AssertL2TotalNativeTokens(&tokenID, util.Big0)

		err = ch.SendFromL1ToL2AccountIotas(IotasDepositFee, 1000, ch.CommonAccount(), senderKeyPair)
		require.NoError(t, err)
		err = ch.MintTokens(sn, 1, senderKeyPair)
		require.NoError(t, err)

		ch.AssertL2NativeTokens(senderAgentID, &tokenID, big.NewInt(1))
		ch.AssertL2TotalNativeTokens(&tokenID, big.NewInt(1))
	})

	t.Run("max supply 1, mintTokens 2", func(t *testing.T) {
		initTest()
		sn, tokenID, err := ch.NewFoundryParams(1).
			WithUser(senderKeyPair).
			CreateFoundry()
		require.NoError(t, err)
		require.EqualValues(t, 1, sn)

		err = ch.MintTokens(sn, 2, senderKeyPair)
		testmisc.RequireErrorToBe(t, err, vm.ErrNativeTokenSupplyOutOffBounds)

		ch.AssertL2NativeTokens(senderAgentID, &tokenID, util.Big0)
		ch.AssertL2TotalNativeTokens(&tokenID, util.Big0)
	})
	t.Run("max supply 1000, mintTokens 500_500_1", func(t *testing.T) {
		initTest()
		sn, tokenID, err := ch.NewFoundryParams(1000).
			WithUser(senderKeyPair).
			CreateFoundry()
		require.NoError(t, err)
		require.EqualValues(t, 1, sn)

		err = ch.SendFromL1ToL2AccountIotas(IotasDepositFee, 1000, ch.CommonAccount(), senderKeyPair)
		require.NoError(t, err)
		err = ch.MintTokens(sn, 500, senderKeyPair)
		require.NoError(t, err)
		ch.AssertL2NativeTokens(senderAgentID, &tokenID, big.NewInt(500))
		ch.AssertL2TotalNativeTokens(&tokenID, big.NewInt(500))

		err = ch.MintTokens(sn, 500, senderKeyPair)
		require.NoError(t, err)
		ch.AssertL2NativeTokens(senderAgentID, &tokenID, 1000)
		ch.AssertL2TotalNativeTokens(&tokenID, 1000)

		err = ch.MintTokens(sn, 1, senderKeyPair)
		testmisc.RequireErrorToBe(t, err, vm.ErrNativeTokenSupplyOutOffBounds)

		ch.AssertL2NativeTokens(senderAgentID, &tokenID, 1000)
		ch.AssertL2TotalNativeTokens(&tokenID, 1000)
	})
	t.Run("max supply MaxUint256, mintTokens MaxUint256_1", func(t *testing.T) {
		t.SkipNow() // TODO not working

		initTest()
		sn, tokenID, err := ch.NewFoundryParams(abi.MaxUint256).
			WithUser(senderKeyPair).
			CreateFoundry()
		require.NoError(t, err)
		require.EqualValues(t, 1, sn)

		err = ch.SendFromL1ToL2AccountIotas(IotasDepositFee, 1000, ch.CommonAccount(), senderKeyPair)
		require.NoError(t, err)
		err = ch.MintTokens(sn, abi.MaxUint256, senderKeyPair)
		require.NoError(t, err)
		ch.AssertL2NativeTokens(senderAgentID, &tokenID, abi.MaxUint256)

		err = ch.MintTokens(sn, 1, senderKeyPair)
		testmisc.RequireErrorToBe(t, err, vm.ErrOverflow)

		ch.AssertL2NativeTokens(senderAgentID, &tokenID, abi.MaxUint256)
		ch.AssertL2TotalNativeTokens(&tokenID, abi.MaxUint256)
	})
	t.Run("max supply 100, destroy fail", func(t *testing.T) {
		initTest()
		sn, tokenID, err := ch.NewFoundryParams(abi.MaxUint256).
			WithUser(senderKeyPair).
			CreateFoundry()
		require.NoError(t, err)
		require.EqualValues(t, 1, sn)

		err = ch.DestroyTokensOnL2(sn, big.NewInt(1), senderKeyPair)
		testmisc.RequireErrorToBe(t, err, accounts.ErrNotEnoughFunds)
		ch.AssertL2NativeTokens(senderAgentID, &tokenID, util.Big0)
		ch.AssertL2TotalNativeTokens(&tokenID, util.Big0)
	})
	t.Run("max supply 100, mint_20, destroy_10", func(t *testing.T) {
		initTest()
		sn, tokenID, err := ch.NewFoundryParams(100).
			WithUser(senderKeyPair).
			CreateFoundry()
		require.NoError(t, err)
		require.EqualValues(t, 1, sn)

		out, err := ch.GetFoundryOutput(1)
		require.NoError(t, err)
		require.EqualValues(t, out.MustNativeTokenID(), tokenID)
		ch.AssertL2NativeTokens(senderAgentID, &tokenID, util.Big0)
		ch.AssertL2TotalNativeTokens(&tokenID, util.Big0)

		err = ch.SendFromL1ToL2AccountIotas(IotasDepositFee, 1000, ch.CommonAccount(), senderKeyPair)
		require.NoError(t, err)
		err = ch.MintTokens(sn, 20, senderKeyPair)
		require.NoError(t, err)
		ch.AssertL2NativeTokens(senderAgentID, &tokenID, 20)
		ch.AssertL2TotalNativeTokens(&tokenID, 20)

		err = ch.DestroyTokensOnL2(sn, 10, senderKeyPair)
		require.NoError(t, err)
		ch.AssertL2TotalNativeTokens(&tokenID, 10)
		ch.AssertL2NativeTokens(senderAgentID, &tokenID, 10)
	})
	t.Run("max supply 1000000, mint_1000000, destroy_1000000", func(t *testing.T) {
		initTest()
		sn, tokenID, err := ch.NewFoundryParams(1_000_000).
			WithUser(senderKeyPair).
			CreateFoundry()
		require.NoError(t, err)
		require.EqualValues(t, 1, sn)

		out, err := ch.GetFoundryOutput(1)
		require.NoError(t, err)
		require.EqualValues(t, out.MustNativeTokenID(), tokenID)
		ch.AssertL2NativeTokens(senderAgentID, &tokenID, 0)
		ch.AssertL2TotalNativeTokens(&tokenID, 0)

		err = ch.SendFromL1ToL2AccountIotas(IotasDepositFee, 1000, ch.CommonAccount(), senderKeyPair)
		require.NoError(t, err)
		err = ch.MintTokens(sn, 1_000_000, senderKeyPair)
		require.NoError(t, err)
		ch.AssertL2NativeTokens(senderAgentID, &tokenID, big.NewInt(1_000_000))
		ch.AssertL2TotalNativeTokens(&tokenID, big.NewInt(1_000_000))
		out, err = ch.GetFoundryOutput(1)
		require.NoError(t, err)
		ts := util.MustTokenScheme(out.TokenScheme)
		require.True(t, big.NewInt(1_000_000).Cmp(ts.MintedTokens) == 0)

		// FIXME bug iotago can't destroy foundry
		// err = destroyTokens(sn, big.NewInt(1000000))
		// require.NoError(t, err)
		// ch.AssertL2TotalNativeTokens(&tokenID, util.Big0)
		// ch.AssertL2NativeTokens(userAgentID, &tokenID, util.Big0)
		// out, err = ch.GetFoundryOutput(1)
		// require.NoError(t, err)
		// require.True(t, util.Big0.Cmp(out.MintedTokens) == 0)
	})
	t.Run("10 foundries", func(t *testing.T) {
		initTest()
		ch.MustDepositIotasToL2(50_000_000, senderKeyPair)
		for sn := uint32(1); sn <= 10; sn++ {
			snBack, tokenID, err := ch.NewFoundryParams(uint64(sn + 1)).
				WithUser(senderKeyPair).
				CreateFoundry()
			require.NoError(t, err)
			require.EqualValues(t, int(sn), int(snBack))
			ch.AssertL2NativeTokens(senderAgentID, &tokenID, util.Big0)
			ch.AssertL2TotalNativeTokens(&tokenID, util.Big0)
		}
		// mint max supply from each
		ch.MustDepositIotasToL2(50_000_000, senderKeyPair)
		for sn := uint32(1); sn <= 10; sn++ {
			err := ch.MintTokens(sn, sn+1, senderKeyPair)
			require.NoError(t, err)

			out, err := ch.GetFoundryOutput(sn)
			require.NoError(t, err)

			require.EqualValues(t, sn, out.SerialNumber)
			ts := util.MustTokenScheme(out.TokenScheme)
			require.True(t, ts.MaximumSupply.Cmp(big.NewInt(int64(sn+1))) == 0)
			require.True(t, ts.MintedTokens.Cmp(big.NewInt(int64(sn+1))) == 0)
			tokenID := out.MustNativeTokenID()

			ch.AssertL2NativeTokens(senderAgentID, &tokenID, big.NewInt(int64(sn+1)))
			ch.AssertL2TotalNativeTokens(&tokenID, big.NewInt(int64(sn+1)))
		}
		// destroy 1 token of each tokenID
		for sn := uint32(1); sn <= 10; sn++ {
			err := ch.DestroyTokensOnL2(sn, big.NewInt(1), senderKeyPair)
			require.NoError(t, err)
		}
		// check balances
		for sn := uint32(1); sn <= 10; sn++ {
			out, err := ch.GetFoundryOutput(sn)
			require.NoError(t, err)

			require.EqualValues(t, sn, out.SerialNumber)
			ts := util.MustTokenScheme(out.TokenScheme)
			require.True(t, ts.MaximumSupply.Cmp(big.NewInt(int64(sn+1))) == 0)
			require.True(t, big.NewInt(0).Sub(ts.MintedTokens, ts.MeltedTokens).Cmp(big.NewInt(int64(sn))) == 0)
			tokenID := out.MustNativeTokenID()

			ch.AssertL2NativeTokens(senderAgentID, &tokenID, big.NewInt(int64(sn)))
			ch.AssertL2TotalNativeTokens(&tokenID, big.NewInt(int64(sn)))
		}
	})
	t.Run("constant dust deposit to hold a token UTXO", func(t *testing.T) {
		initTest()
		// create a foundry for the maximum amount of tokens possible
		sn, tokenID, err := ch.NewFoundryParams(util.MaxUint256).
			WithUser(senderKeyPair).
			CreateFoundry()
		require.NoError(t, err)

		err = ch.SendFromL1ToL2AccountIotas(IotasDepositFee, 1, ch.CommonAccount(), senderKeyPair)
		require.NoError(t, err)
		x := ch.L2CommonAccountIotas()
		t.Logf("common account iotas = %d before mint", x)

		big1 := big.NewInt(1)
		err = ch.MintTokens(sn, big1, senderKeyPair)
		require.NoError(t, err)

		ch.AssertL2NativeTokens(senderAgentID, &tokenID, big1)
		ch.AssertL2TotalNativeTokens(&tokenID, big1)

		commonAccountBalanceBeforeLastMint := ch.L2CommonAccountIotas()

		// after minting 1 token, try to mint the remaining tokens
		allOtherTokens := new(big.Int).Set(util.MaxUint256)
		allOtherTokens = allOtherTokens.Sub(allOtherTokens, big1)

		err = ch.MintTokens(sn, allOtherTokens, senderKeyPair)
		require.NoError(t, err)

		// assert that no extra iotas were used for the dust deposit
		receipt := ch.LastReceipt()
		commonAccountBalanceAfterLastMint := ch.L2CommonAccountIotas()
		require.Equal(t, commonAccountBalanceAfterLastMint, commonAccountBalanceBeforeLastMint+receipt.GasFeeCharged)
	})
}

func TestAccountBalances(t *testing.T) {
	env := solo.New(t)

	chainOwner, chainOwnerAddr := env.NewKeyPairWithFunds(env.NewSeedFromIndex(10))
	chainOwnerAgentID := iscp.NewAgentID(chainOwnerAddr)

	sender, senderAddr := env.NewKeyPairWithFunds(env.NewSeedFromIndex(11))
	senderAgentID := iscp.NewAgentID(senderAddr)

	l1Iotas := func(addr iotago.Address) uint64 { return env.L1Assets(addr).Iotas }
	totalIotas := l1Iotas(chainOwnerAddr) + l1Iotas(senderAddr)

	ch := env.NewChain(chainOwner, "chain1")

	l2Iotas := func(agentID iscp.AgentID) uint64 { return ch.L2Iotas(agentID) }
	totalGasFeeCharged := uint64(0)

	checkBalance := func(numReqs int) {
		require.EqualValues(t,
			totalIotas,
			l1Iotas(chainOwnerAddr)+l1Iotas(senderAddr)+l1Iotas(ch.ChainID.AsAddress()),
		)

		anchor := ch.GetAnchorOutput().GetAliasOutput()
		require.EqualValues(t, l1Iotas(ch.ChainID.AsAddress()), anchor.Deposit())

		require.LessOrEqual(t, len(ch.L2Accounts()), 3)

		bi := ch.GetLatestBlockInfo()

		require.EqualValues(t,
			anchor.Deposit(),
			bi.TotalIotasInL2Accounts+bi.TotalDustDeposit,
		)

		require.EqualValues(t,
			bi.TotalIotasInL2Accounts,
			l2Iotas(chainOwnerAgentID)+l2Iotas(senderAgentID)+l2Iotas(ch.CommonAccount()),
		)

		// not true because of deposit preload
		// require.Equal(t, numReqs == 0, bi.GasFeeCharged == 0)

		totalGasFeeCharged += bi.GasFeeCharged
		require.EqualValues(t,
			int(l2Iotas(ch.CommonAccount())),
			int(totalGasFeeCharged),
		)

		require.EqualValues(t,
			utxodb.FundsFromFaucetAmount+totalGasFeeCharged-bi.TotalDustDeposit,
			l1Iotas(chainOwnerAddr)+l2Iotas(chainOwnerAgentID)+l2Iotas(ch.CommonAccount()),
		)
		require.EqualValues(t,
			utxodb.FundsFromFaucetAmount-totalGasFeeCharged,
			l1Iotas(senderAddr)+l2Iotas(senderAgentID),
		)
	}

	// preload sender account with iotas in order to be able to pay for gas fees
	err := ch.DepositIotasToL2(100_000, sender)
	require.NoError(t, err)

	checkBalance(0)

	for i := 0; i < 5; i++ {
		blobData := fmt.Sprintf("dummy blob data #%d", i+1)
		_, err := ch.UploadBlob(sender, "field", blobData)
		require.NoError(t, err)

		checkBalance(i + 1)
	}
}

type testParams struct {
	env               *solo.Solo
	chainOwner        *cryptolib.KeyPair
	chainOwnerAddr    iotago.Address
	chainOwnerAgentID iscp.AgentID
	user              *cryptolib.KeyPair
	userAddr          iotago.Address
	userAgentID       iscp.AgentID
	ch                *solo.Chain
	req               *solo.CallParams
	sn                uint32
	tokenID           *iotago.NativeTokenID
}

func initDepositTest(t *testing.T, initLoad ...uint64) *testParams {
	ret := &testParams{}
	ret.env = solo.New(t, &solo.InitOptions{AutoAdjustDustDeposit: true})

	ret.chainOwner, ret.chainOwnerAddr = ret.env.NewKeyPairWithFunds(ret.env.NewSeedFromIndex(10))
	ret.chainOwnerAgentID = iscp.NewAgentID(ret.chainOwnerAddr)
	ret.user, ret.userAddr = ret.env.NewKeyPairWithFunds(ret.env.NewSeedFromIndex(11))
	ret.userAgentID = iscp.NewAgentID(ret.userAddr)

	if len(initLoad) == 0 {
		ret.ch = ret.env.NewChain(ret.chainOwner, "chain1")
	} else {
		ret.ch, _, _ = ret.env.NewChainExt(ret.chainOwner, initLoad[0], "chain1")
	}

	ret.req = solo.NewCallParams(accounts.Contract.Name, accounts.FuncDeposit.Name)
	return ret
}

func (v *testParams) createFoundryAndMint(maxSupply, amount interface{}) (uint32, *iotago.NativeTokenID) {
	sn, tokenID, err := v.ch.NewFoundryParams(maxSupply).
		WithUser(v.user).
		CreateFoundry()
	require.NoError(v.env.T, err)
	// mint some tokens for the user
	err = v.ch.MintTokens(sn, amount, v.user)
	require.NoError(v.env.T, err)
	// check the balance of the user
	v.ch.AssertL2NativeTokens(v.userAgentID, &tokenID, amount)
	require.True(v.env.T, v.ch.L2Iotas(v.userAgentID) > 100) // must be some coming from dust deposits
	return sn, &tokenID
}

func TestDepositIotas(t *testing.T) {
	// the test check how request transaction construction functions adjust iotas to the minimum needed for the
	// dust deposit. If byte cost is 185, anything below that fill be topped up to 185, above that no adjustment is needed
	for _, addIotas := range []uint64{0, 50, 150, 200, 1000} {
		t.Run("add iotas "+strconv.Itoa(int(addIotas)), func(t *testing.T) {
			v := initDepositTest(t)
			v.req.WithGasBudget(100_000)
			gas, _, err := v.ch.EstimateGasOnLedger(v.req, v.user)
			require.NoError(t, err)

			v.req.WithGasBudget(gas)

			v.req = v.req.AddIotas(addIotas)
			tx, _, err := v.ch.PostRequestSyncTx(v.req, v.user)
			require.NoError(t, err)
			rec := v.ch.LastReceipt()

			storageDeposit := parameters.L1.Protocol.RentStructure.MinRent(tx.Essence.Outputs[0])
			t.Logf("byteCost = %d", storageDeposit)

			adjusted := addIotas
			if adjusted < storageDeposit {
				adjusted = storageDeposit
			}
			require.True(t, rec.GasFeeCharged <= adjusted)
			v.ch.AssertL2Iotas(v.userAgentID, adjusted-rec.GasFeeCharged)
		})
	}
}

// initWithdrawTest creates foundry with 1_000_000 of max supply and mint 100 tokens to user's account
func initWithdrawTest(t *testing.T, initLoad ...uint64) *testParams {
	v := initDepositTest(t, initLoad...)
	v.ch.MustDepositIotasToL2(2*iscp.Mi, v.user)
	// create foundry and mint 100 tokens
	v.sn, v.tokenID = v.createFoundryAndMint(1_000_000, 100)
	// prepare request parameters to withdraw everything what is in the account
	// do not run the request yet
	v.req = solo.NewCallParams("accounts", "withdraw").
		AddIotas(12000).
		WithGasBudget(100_000)
	v.printBalances("BEGIN")
	return v
}

func (v *testParams) printBalances(prefix string) {
	v.env.T.Logf("%s: user L1 iotas: %d", prefix, v.env.L1Iotas(v.userAddr))
	v.env.T.Logf("%s: user L1 tokens: %s : %d", prefix, v.tokenID, v.env.L1NativeTokens(v.userAddr, v.tokenID))
	v.env.T.Logf("%s: user L2: %s", prefix, v.ch.L2Assets(v.userAgentID))
	v.env.T.Logf("%s: common account L2: %s", prefix, v.ch.L2CommonAccountAssets())
}

func TestWithdrawDepositNativeTokens(t *testing.T) {
	t.Run("withdraw with empty", func(t *testing.T) {
		v := initWithdrawTest(t, 2*iscp.Mi)
		_, err := v.ch.PostRequestSync(v.req, v.user)
		testmisc.RequireErrorToBe(t, err, "can't be empty")
	})
	t.Run("withdraw not enough for dust", func(t *testing.T) {
		v := initWithdrawTest(t, 2*iscp.Mi)
		v.req.AddAllowanceNativeTokensVect(&iotago.NativeToken{
			ID:     *v.tokenID,
			Amount: new(big.Int).SetUint64(1 * iscp.Mi),
		})
		_, err := v.ch.PostRequestSync(v.req, v.user)
		testmisc.RequireErrorToBe(t, err, accounts.ErrNotEnoughIotasForDustDeposit)
	})
	t.Run("withdraw almost all", func(t *testing.T) {
		v := initWithdrawTest(t, 2*iscp.Mi)
		// we want to withdraw as many iotas as possible, so we add 300 because some more will come
		// with assets attached to the 'withdraw' request. However, withdraw all is not possible due to gas
		toWithdraw := v.ch.L2Assets(v.userAgentID).AddIotas(200)
		t.Logf("assets to withdraw: %s", toWithdraw.String())
		// withdraw all tokens to L1, but we do not add iotas to allowance, so not enough for dust
		v.req.AddAllowance(iscp.NewAllowanceFungibleTokens(toWithdraw))
		v.req.AddIotas(IotasDepositFee)
		_, err := v.ch.PostRequestSync(v.req, v.user)
		require.NoError(t, err)
		v.printBalances("END")
	})
	t.Run("mint withdraw destroy fail", func(t *testing.T) {
		v := initWithdrawTest(t, 2*iscp.Mi)
		allSenderAssets := v.ch.L2Assets(v.userAgentID)
		v.req.AddAllowance(iscp.NewAllowanceFungibleTokens(allSenderAssets))
		v.req.AddIotas(IotasDepositFee)
		_, err := v.ch.PostRequestSync(v.req, v.user)
		require.NoError(t, err)

		v.printBalances("AFTER MINT")
		v.env.AssertL1NativeTokens(v.userAddr, v.tokenID, 100)

		// should fail because those tokens are not on the user's on chain account
		err = v.ch.DestroyTokensOnL2(v.sn, big.NewInt(50), v.user)
		testmisc.RequireErrorToBe(t, err, accounts.ErrNotEnoughFunds)
		v.env.AssertL1NativeTokens(v.userAddr, v.tokenID, big.NewInt(100))
		v.printBalances("AFTER DESTROY")
	})
	t.Run("mint withdraw destroy success 1", func(t *testing.T) {
		v := initWithdrawTest(t, 2*iscp.Mi)

		allSenderAssets := v.ch.L2Assets(v.userAgentID)
		v.req.AddAllowance(iscp.NewAllowanceFungibleTokens(allSenderAssets))
		v.req.AddIotas(IotasDepositFee)
		_, err := v.ch.PostRequestSync(v.req, v.user)
		require.NoError(t, err)
		v.printBalances("AFTER MINT")
		v.env.AssertL1NativeTokens(v.userAddr, v.tokenID, 100)
		v.ch.AssertL2NativeTokens(v.userAgentID, v.tokenID, 0)

		err = v.ch.DepositAssetsToL2(iscp.NewEmptyAssets().AddNativeTokens(*v.tokenID, 50), v.user)
		require.NoError(t, err)
		v.env.AssertL1NativeTokens(v.userAddr, v.tokenID, 50)
		v.ch.AssertL2NativeTokens(v.userAgentID, v.tokenID, 50)
		v.ch.AssertL2TotalNativeTokens(v.tokenID, 50)
		v.printBalances("AFTER DEPOSIT")

		err = v.ch.DestroyTokensOnL2(v.sn, 49, v.user)
		require.NoError(t, err)
		v.ch.AssertL2NativeTokens(v.userAgentID, v.tokenID, 1)
		v.env.AssertL1NativeTokens(v.userAddr, v.tokenID, 50)
		v.printBalances("AFTER DESTROY")
	})
	t.Run("unwrap use case", func(t *testing.T) {
		v := initWithdrawTest(t, 2*iscp.Mi)
		allSenderAssets := v.ch.L2Assets(v.userAgentID)
		v.req.AddAllowance(iscp.NewAllowanceFungibleTokens(allSenderAssets))
		v.req.AddIotas(IotasDepositFee)
		_, err := v.ch.PostRequestSync(v.req, v.user)
		require.NoError(t, err)
		v.printBalances("AFTER MINT")
		v.env.AssertL1NativeTokens(v.userAddr, v.tokenID, 100)
		v.ch.AssertL2NativeTokens(v.userAgentID, v.tokenID, 0)

		err = v.ch.DepositAssetsToL2(iscp.NewEmptyAssets().AddNativeTokens(*v.tokenID, 1), v.user)
		require.NoError(t, err)
		v.printBalances("AFTER DEPOSIT 1")

		// without deposit
		err = v.ch.DestroyTokensOnL1(v.tokenID, 49, v.user)
		require.NoError(t, err)
		v.printBalances("AFTER DESTROY")
		v.ch.AssertL2NativeTokens(v.userAgentID, v.tokenID, 1)
		v.env.AssertL1NativeTokens(v.userAddr, v.tokenID, 50)
	})
	t.Run("unwrap use case", func(t *testing.T) {
		v := initWithdrawTest(t, 2*iscp.Mi)
		allSenderAssets := v.ch.L2Assets(v.userAgentID)
		v.req.AddAllowance(iscp.NewAllowanceFungibleTokens(allSenderAssets))
		v.req.AddIotas(IotasDepositFee)
		_, err := v.ch.PostRequestSync(v.req, v.user)
		require.NoError(t, err)
		v.printBalances("AFTER MINT")
		v.env.AssertL1NativeTokens(v.userAddr, v.tokenID, 100)
		v.ch.AssertL2NativeTokens(v.userAgentID, v.tokenID, 0)

		// without deposit
		err = v.ch.DestroyTokensOnL1(v.tokenID, 49, v.user)
		require.NoError(t, err)
		v.printBalances("AFTER DESTROY")
		v.ch.AssertL2NativeTokens(v.userAgentID, v.tokenID, 0)
		v.env.AssertL1NativeTokens(v.userAddr, v.tokenID, 51)
	})
	t.Run("mint withdraw destroy fail", func(t *testing.T) {
		v := initWithdrawTest(t, 2*iscp.Mi)
		allSenderAssets := v.ch.L2Assets(v.userAgentID)
		v.req.AddAllowance(iscp.NewAllowanceFungibleTokens(allSenderAssets))
		v.req.AddIotas(IotasDepositFee)
		_, err := v.ch.PostRequestSync(v.req, v.user)
		require.NoError(t, err)

		v.printBalances("AFTER MINT")
		v.env.AssertL1NativeTokens(v.userAddr, v.tokenID, 100)
		v.ch.AssertL2NativeTokens(v.userAgentID, v.tokenID, 0)

		err = v.ch.DepositAssetsToL2(iscp.NewEmptyAssets().AddNativeTokens(*v.tokenID, 50), v.user)
		require.NoError(t, err)
		v.env.AssertL1NativeTokens(v.userAddr, v.tokenID, 50)
		v.ch.AssertL2NativeTokens(v.userAgentID, v.tokenID, 50)
		v.ch.AssertL2TotalNativeTokens(v.tokenID, 50)
		v.printBalances("AFTER DEPOSIT")

		err = v.ch.DestroyTokensOnL2(v.sn, 50, v.user)
		require.NoError(t, err)
		v.ch.AssertL2NativeTokens(v.userAgentID, v.tokenID, 0)
		v.env.AssertL1NativeTokens(v.userAddr, v.tokenID, 50)
	})
}

func TestTransferAndHarvest(t *testing.T) {
	// initializes it all and prepares withdraw request, does not post it
	v := initWithdrawTest(t, 10_000)
	dustCosts := transaction.NewStorageDepositEstimate()
	commonAssets := v.ch.L2CommonAccountAssets()
	require.True(t, commonAssets.Iotas+dustCosts.AnchorOutput > 10_000)
	require.EqualValues(t, 0, len(commonAssets.Tokens))

	v.ch.AssertL2NativeTokens(v.userAgentID, v.tokenID, 100)

	// move minted tokens from user to the common account on-chain
	err := v.ch.SendFromL2ToL2AccountNativeTokens(*v.tokenID, v.ch.CommonAccount(), 50, v.user)
	require.NoError(t, err)
	// now we have 50 tokens on common account
	v.ch.AssertL2NativeTokens(v.ch.CommonAccount(), v.tokenID, 50)
	// no native tokens for chainOwner on L1
	v.env.AssertL1NativeTokens(v.chainOwnerAddr, v.tokenID, 0)

	err = v.ch.DepositIotasToL2(10_000, v.chainOwner)
	require.NoError(t, err)

	v.req = solo.NewCallParams("accounts", "harvest").
		WithGasBudget(100_000)
	_, err = v.ch.PostRequestSync(v.req, v.chainOwner)
	require.NoError(t, err)

	rec := v.ch.LastReceipt()
	t.Logf("receipt from the 'harvest' tx: %s", rec)

	// now we have 0 tokens on common account
	v.ch.AssertL2NativeTokens(v.ch.CommonAccount(), v.tokenID, 0)
	// 50 native tokens for chain on L2
	v.ch.AssertL2NativeTokens(v.chainOwnerAgentID, v.tokenID, 50)

	commonAssets = v.ch.L2CommonAccountAssets()
	// in the common account should have left minimum plus gas fee from the last request
	require.EqualValues(t, accounts.MinimumIotasOnCommonAccount+rec.GasFeeCharged, commonAssets.Iotas)
	require.EqualValues(t, 0, len(commonAssets.Tokens))
}

func TestFoundryDestroy(t *testing.T) {
	t.Run("destroy existing", func(t *testing.T) {
		v := initDepositTest(t)
		v.ch.MustDepositIotasToL2(2*iscp.Mi, v.user)
		sn, _, err := v.ch.NewFoundryParams(1_000_000).
			WithUser(v.user).
			CreateFoundry()
		require.NoError(t, err)

		err = v.ch.DestroyFoundry(sn, v.user)
		require.NoError(t, err)
		_, err = v.ch.GetFoundryOutput(sn)
		testmisc.RequireErrorToBe(t, err, "does not exist")
	})
	t.Run("destroy fail", func(t *testing.T) {
		v := initDepositTest(t)
		err := v.ch.DestroyFoundry(2, v.user)
		testmisc.RequireErrorToBe(t, err, "not controlled by the caller")
	})
}

func TestTransferPartialAssets(t *testing.T) {
	v := initDepositTest(t)
	v.ch.MustDepositIotasToL2(10*iscp.Mi, v.user)
	// setup a chain with some iotas and native tokens for user1
	sn, tokenID, err := v.ch.NewFoundryParams(10).
		WithUser(v.user).
		CreateFoundry()
	require.NoError(t, err)
	require.EqualValues(t, 1, int(sn))

	// deposit iotas for the chain owner (needed for L1 dust byte cost to mint tokens)
	err = v.ch.SendFromL1ToL2AccountIotas(IotasDepositFee, 1*iscp.Mi, v.ch.CommonAccount(), v.chainOwner)
	require.NoError(t, err)
	err = v.ch.SendFromL1ToL2AccountIotas(IotasDepositFee, 1*iscp.Mi, v.userAgentID, v.user)
	require.NoError(t, err)

	err = v.ch.MintTokens(sn, big.NewInt(10), v.user)
	require.NoError(t, err)

	v.ch.AssertL2NativeTokens(v.userAgentID, &tokenID, big.NewInt(10))
	v.ch.AssertL2TotalNativeTokens(&tokenID, big.NewInt(10))

	// send funds to user2
	user2, user2Addr := v.env.NewKeyPairWithFunds(v.env.NewSeedFromIndex(100))
	user2AgentID := iscp.NewAgentID(user2Addr)

	// deposit 1 iota to "create account" for user2 // TODO maybe remove if account creation is not needed
	v.ch.AssertL2Iotas(user2AgentID, 0)
	const iotasToSend = 3 * iscp.Mi
	err = v.ch.SendFromL1ToL2AccountIotas(IotasDepositFee, iotasToSend, user2AgentID, user2)
	rec := v.ch.LastReceipt()
	require.NoError(t, err)
	v.env.T.Logf("gas fee charged: %d", rec.GasFeeCharged)
	expectedUser2 := IotasDepositFee + iotasToSend - rec.GasFeeCharged
	v.ch.AssertL2Iotas(user2AgentID, expectedUser2)
	// -----------------------------
	err = v.ch.SendFromL2ToL2Account(
		iscp.NewAllowance(
			iotasToSend,
			iotago.NativeTokens{
				&iotago.NativeToken{
					ID:     tokenID,
					Amount: big.NewInt(9),
				},
			},
			nil,
		),
		user2AgentID,
		v.user,
	)
	require.NoError(t, err)

	// assert that balances are correct
	v.ch.AssertL2NativeTokens(v.userAgentID, &tokenID, big.NewInt(1))
	v.ch.AssertL2NativeTokens(user2AgentID, &tokenID, big.NewInt(9))
	v.ch.AssertL2Iotas(user2AgentID, expectedUser2+iotasToSend)
	v.ch.AssertL2TotalNativeTokens(&tokenID, big.NewInt(10))
}

// TestMintedTokensBurn belongs to iota.go
func TestMintedTokensBurn(t *testing.T) {
	const OneMi = 1_000_000

	_, ident1, ident1AddrKeys := tpkg.RandEd25519Identity()
	aliasIdent1 := tpkg.RandAliasAddress()

	inputIDs := tpkg.RandOutputIDs(3)
	inputs := iotago.OutputSet{
		inputIDs[0]: &iotago.BasicOutput{
			Amount: OneMi,
			Conditions: iotago.UnlockConditions{
				&iotago.AddressUnlockCondition{Address: ident1},
			},
		},
		inputIDs[1]: &iotago.AliasOutput{
			Amount:         OneMi,
			NativeTokens:   nil,
			AliasID:        aliasIdent1.AliasID(),
			StateIndex:     1,
			StateMetadata:  nil,
			FoundryCounter: 1,
			Conditions: iotago.UnlockConditions{
				&iotago.StateControllerAddressUnlockCondition{Address: ident1},
				&iotago.GovernorAddressUnlockCondition{Address: ident1},
			},
			Features: nil,
		},
		inputIDs[2]: &iotago.FoundryOutput{
			Amount:       OneMi,
			NativeTokens: nil,
			SerialNumber: 1,
			TokenScheme: &iotago.SimpleTokenScheme{
				MintedTokens:  big.NewInt(50),
				MeltedTokens:  util.Big0,
				MaximumSupply: big.NewInt(50),
			},
			Conditions: iotago.UnlockConditions{
				&iotago.ImmutableAliasUnlockCondition{Address: aliasIdent1},
			},
			Features: nil,
		},
	}

	// set input BasicOutput NativeToken to 50 which get burned
	foundryNativeTokenID := inputs[inputIDs[2]].(*iotago.FoundryOutput).MustNativeTokenID()
	inputs[inputIDs[0]].(*iotago.BasicOutput).NativeTokens = iotago.NativeTokens{
		{
			ID:     foundryNativeTokenID,
			Amount: new(big.Int).SetInt64(50),
		},
	}

	essence := &iotago.TransactionEssence{
		NetworkID: tpkg.TestNetworkID,
		Inputs:    inputIDs.UTXOInputs(),
		Outputs: iotago.Outputs{
			&iotago.AliasOutput{
				Amount:         OneMi,
				NativeTokens:   nil,
				AliasID:        aliasIdent1.AliasID(),
				StateIndex:     1,
				StateMetadata:  nil,
				FoundryCounter: 1,
				Conditions: iotago.UnlockConditions{
					&iotago.StateControllerAddressUnlockCondition{Address: ident1},
					&iotago.GovernorAddressUnlockCondition{Address: ident1},
				},
				Features: nil,
			},
			&iotago.FoundryOutput{
				Amount:       2 * OneMi,
				NativeTokens: nil,
				SerialNumber: 1,
				TokenScheme: &iotago.SimpleTokenScheme{
					// burn supply by -50
					MintedTokens:  big.NewInt(50),
					MeltedTokens:  big.NewInt(50),
					MaximumSupply: big.NewInt(50),
				},
				Conditions: iotago.UnlockConditions{
					&iotago.ImmutableAliasUnlockCondition{Address: aliasIdent1},
				},
				Features: nil,
			},
		},
	}

	sigs, err := essence.Sign(inputIDs.OrderedSet(inputs).MustCommitment(), ident1AddrKeys)
	require.NoError(t, err)

	tx := &iotago.Transaction{
		Essence: essence,
		Unlocks: iotago.Unlocks{
			&iotago.SignatureUnlock{Signature: sigs[0]},
			&iotago.ReferenceUnlock{Reference: 0},
			&iotago.AliasUnlock{Reference: 1},
		},
	}

	require.NoError(t, tx.SemanticallyValidate(&iotago.SemanticValidationContext{
		ExtParas:   nil,
		WorkingSet: nil,
	}, inputs))
}

func TestNFTAccount(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustDustDeposit: true})
	ch := env.NewChain(nil, "chain1")

	issuerWallet, _ := ch.Env.NewKeyPairWithFunds()
	ownerWallet, ownerAddress := ch.Env.NewKeyPairWithFunds()
	ownerBalance := ch.Env.L1Iotas(ownerAddress)

	nftInfo, err := ch.Env.MintNFTL1(issuerWallet, ownerAddress, []byte("foobar"))
	require.NoError(t, err)
	nftAddress := nftInfo.NFTID.ToAddress()

	// deposit funds on behalf of the NFT
	const iotasToSend = 10 * iscp.Mi
	req := solo.NewCallParams(accounts.Contract.Name, accounts.FuncDeposit.Name).
		AddFungibleTokens(iscp.NewTokensIotas(iotasToSend)).
		WithMaxAffordableGasBudget().
		WithSender(nftAddress)

	_, err = ch.PostRequestSync(req, ownerWallet)
	require.NoError(t, err)
	rec := ch.LastReceipt()

	nftAgentID := iscp.NewAgentID(nftAddress)
	ch.AssertL2Iotas(nftAgentID, iotasToSend-rec.GasFeeCharged)
	ch.Env.AssertL1Iotas(nftAddress, 0)
	ch.Env.AssertL1Iotas(
		ownerAddress,
		ownerBalance+nftInfo.Output.Deposit()-iotasToSend,
	)
	require.True(t, ch.Env.HasL1NFT(ownerAddress, &nftInfo.NFTID))

	// withdraw to the NFT on L1
	const iotasToWithdrawal = 1 * iscp.Mi
	wdReq := solo.NewCallParams(accounts.Contract.Name, accounts.FuncWithdraw.Name).
		AddAllowanceIotas(iotasToWithdrawal).
		WithMaxAffordableGasBudget()

	// NFT owner on L1 can't move L2 funds owned by the NFT unless the request is sent in behalf of the NFT (NFTID is specified as "Sender")
	_, err = ch.PostRequestSync(wdReq, ownerWallet)
	require.Error(t, err)

	// NFT owner can withdraw funds owned by the NFT on the chain
	_, err = ch.PostRequestSync(wdReq.WithSender(nftAddress), ownerWallet)
	require.NoError(t, err)
	ch.Env.AssertL1Iotas(nftAddress, iotasToWithdrawal)

	ch.Env.AssertL1Iotas(nftAddress, iotasToWithdrawal)
}
