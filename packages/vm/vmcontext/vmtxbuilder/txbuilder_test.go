package vmtxbuilder

import (
	"math/big"
	"math/rand"
	"testing"

	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/tpkg"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/testutil/testiotago"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/util/panicutil"
	"github.com/iotaledger/wasp/packages/vm/vmcontext/vmexceptions"
	"github.com/stretchr/testify/require"
)

func rndAliasID() (ret iotago.AliasID) {
	a := tpkg.RandAliasAddress()
	copy(ret[:], a[:])
	return
}

// return deposit in BaseToken
func consumeUTXO(t *testing.T, txb *AnchorTransactionBuilder, id iotago.NativeTokenID, amountNative uint64, addBaseTokensToDustMinimum ...uint64) uint64 {
	var assets *iscp.FungibleTokens
	if amountNative > 0 {
		assets = &iscp.FungibleTokens{
			BaseTokens: 0,
			Tokens:     iotago.NativeTokens{{ID: id, Amount: big.NewInt(int64(amountNative))}},
		}
	}
	out := transaction.MakeBasicOutput(
		txb.anchorOutput.AliasID.ToAddress(),
		nil,
		assets,
		nil,
		iscp.SendOptions{},
	)
	if len(addBaseTokensToDustMinimum) > 0 {
		out.Amount += addBaseTokensToDustMinimum[0]
	}
	reqData, err := iscp.OnLedgerFromUTXO(out, &iotago.UTXOInput{})
	require.NoError(t, err)
	txb.Consume(reqData)
	_, _, err = txb.Totals()
	require.NoError(t, err)
	return out.Deposit()
}

func addOutput(txb *AnchorTransactionBuilder, amount uint64, tokenID iotago.NativeTokenID) uint64 {
	assets := &iscp.FungibleTokens{
		BaseTokens: 0,
		Tokens: iotago.NativeTokens{
			&iotago.NativeToken{
				ID:     tokenID,
				Amount: new(big.Int).SetUint64(amount),
			},
		},
	}
	exout := transaction.BasicOutputFromPostData(
		txb.anchorOutput.AliasID.ToAddress(),
		iscp.Hn("test"),
		iscp.RequestParameters{
			TargetAddress:              tpkg.RandEd25519Address(),
			FungibleTokens:             assets,
			Metadata:                   &iscp.SendMetadata{},
			Options:                    iscp.SendOptions{},
			AdjustToMinimumDustDeposit: true,
		},
	)
	txb.AddOutput(exout)
	_, _, err := txb.Totals()
	if err != nil {
		panic(err)
	}
	return exout.Deposit()
}

func TestTxBuilderBasic(t *testing.T) {
	const initialTotalBaseTokens = 10 * iscp.Mi
	addr := tpkg.RandEd25519Address()
	stateMetadata := hashing.HashStrings("test")
	aliasID := rndAliasID()
	anchor := &iotago.AliasOutput{
		Amount:       initialTotalBaseTokens,
		NativeTokens: nil,
		AliasID:      aliasID,
		Conditions: iotago.UnlockConditions{
			&iotago.StateControllerAddressUnlockCondition{Address: addr},
			&iotago.GovernorAddressUnlockCondition{Address: addr},
		},
		StateIndex:     0,
		StateMetadata:  stateMetadata[:],
		FoundryCounter: 0,
		Features: iotago.Features{
			&iotago.SenderFeature{
				Address: aliasID.ToAddress(),
			},
		},
	}
	anchorID := tpkg.RandOutputIDs(1)[0]
	tokenID := testiotago.RandNativeTokenID()
	balanceLoader := func(_ *iotago.NativeTokenID) (*iotago.BasicOutput, *iotago.UTXOInput) {
		return nil, &iotago.UTXOInput{}
	}
	t.Run("1", func(t *testing.T) {
		txb := NewAnchorTransactionBuilder(anchor, anchorID, func(id *iotago.NativeTokenID) (*iotago.BasicOutput, *iotago.UTXOInput) {
			return nil, nil
		},
			nil,
			nil,
			*transaction.NewStorageDepositEstimate(),
		)
		totals, _, err := txb.Totals()
		require.NoError(t, err)
		require.EqualValues(t, initialTotalBaseTokens-txb.dustDepositAssumption.AnchorOutput, totals.TotalBaseTokensInL2Accounts)
		require.EqualValues(t, 0, len(totals.NativeTokenBalances))

		require.EqualValues(t, 1, txb.numInputs())
		require.EqualValues(t, 1, txb.numOutputs())
		require.False(t, txb.InputsAreFull())
		require.False(t, txb.outputsAreFull())

		essence, _ := txb.BuildTransactionEssence(state.RandL1Commitment())
		require.EqualValues(t, 1, len(essence.Inputs))
		require.EqualValues(t, 1, len(essence.Outputs))

		essenceBytes, err := essence.Serialize(serializer.DeSeriModeNoValidation, nil)
		require.NoError(t, err)
		t.Logf("essence bytes len = %d", len(essenceBytes))
	})
	t.Run("2", func(t *testing.T) {
		txb := NewAnchorTransactionBuilder(anchor, anchorID, func(id *iotago.NativeTokenID) (*iotago.BasicOutput, *iotago.UTXOInput) {
			return nil, nil
		},
			nil,
			nil,
			*transaction.NewStorageDepositEstimate(),
		)
		txb.addDeltaBaseTokensToTotal(42)
		require.EqualValues(t, int(initialTotalBaseTokens-txb.dustDepositAssumption.AnchorOutput+42), int(txb.totalBaseTokensInL2Accounts))
		_, _, err := txb.Totals()
		require.Error(t, err)
	})
	t.Run("3", func(t *testing.T) {
		txb := NewAnchorTransactionBuilder(
			anchor, anchorID, balanceLoader, nil, nil,
			*transaction.NewStorageDepositEstimate(),
		)
		_, _, err := txb.Totals()
		require.NoError(t, err)
		deposit := consumeUTXO(t, txb, tokenID, 0)

		t.Logf("vByteCost anchor: %d, internal output: %d, 'empty' output deposit: %d",
			txb.dustDepositAssumption.AnchorOutput, txb.dustDepositAssumption.NativeTokenOutput, deposit)

		totalsIn, totalsOut, err := txb.Totals()
		require.NoError(t, err)
		require.EqualValues(t, txb.dustDepositAssumption.AnchorOutput, totalsIn.TotalBaseTokensInDustDeposit)
		require.EqualValues(t, txb.dustDepositAssumption.AnchorOutput, totalsOut.TotalBaseTokensInDustDeposit)

		expectedBaseTokens := initialTotalBaseTokens - txb.dustDepositAssumption.AnchorOutput + deposit
		require.EqualValues(t, expectedBaseTokens, int(totalsOut.TotalBaseTokensInL2Accounts))
		require.EqualValues(t, 0, len(totalsOut.NativeTokenBalances))

		essence, _ := txb.BuildTransactionEssence(state.RandL1Commitment())

		essenceBytes, err := essence.Serialize(serializer.DeSeriModeNoValidation, nil)
		require.NoError(t, err)
		t.Logf("essence bytes len = %d", len(essenceBytes))
	})
	t.Run("4", func(t *testing.T) {
		txb := NewAnchorTransactionBuilder(anchor, anchorID, balanceLoader, nil, nil,
			*transaction.NewStorageDepositEstimate(),
		)
		_, _, err := txb.Totals()
		require.NoError(t, err)
		deposit := consumeUTXO(t, txb, tokenID, 10)

		t.Logf("vByteCost anchor: %d, internal output: %d",
			txb.dustDepositAssumption.AnchorOutput, txb.dustDepositAssumption.NativeTokenOutput)

		totalsIn, totalsOut, err := txb.Totals()
		require.NoError(t, err)
		require.EqualValues(t, int(txb.dustDepositAssumption.AnchorOutput), int(totalsIn.TotalBaseTokensInDustDeposit))
		require.EqualValues(t, int(txb.dustDepositAssumption.AnchorOutput+txb.dustDepositAssumption.NativeTokenOutput), int(totalsOut.TotalBaseTokensInDustDeposit))

		expectedBaseTokens := initialTotalBaseTokens + deposit - txb.dustDepositAssumption.AnchorOutput - txb.dustDepositAssumption.NativeTokenOutput
		require.EqualValues(t, int(expectedBaseTokens), int(totalsOut.TotalBaseTokensInL2Accounts))
		require.EqualValues(t, 1, len(totalsOut.NativeTokenBalances))
		require.True(t, totalsOut.NativeTokenBalances[tokenID].Cmp(new(big.Int).SetUint64(10)) == 0)

		essence, _ := txb.BuildTransactionEssence(state.RandL1Commitment())

		essenceBytes, err := essence.Serialize(serializer.DeSeriModeNoValidation, nil)
		require.NoError(t, err)
		t.Logf("essence bytes len = %d", len(essenceBytes))
	})
}

func TestTxBuilderConsistency(t *testing.T) {
	const initialTotalBaseTokens = 10 * iscp.Mi
	addr := tpkg.RandEd25519Address()
	stateMetadata := hashing.HashStrings("test")
	aliasID := rndAliasID()
	anchor := &iotago.AliasOutput{
		Amount:       initialTotalBaseTokens,
		NativeTokens: nil,
		AliasID:      aliasID,
		Conditions: iotago.UnlockConditions{
			&iotago.StateControllerAddressUnlockCondition{Address: addr},
			&iotago.GovernorAddressUnlockCondition{Address: addr},
		},
		StateIndex:     0,
		StateMetadata:  stateMetadata[:],
		FoundryCounter: 0,
		Features: iotago.Features{
			&iotago.SenderFeature{
				Address: aliasID.ToAddress(),
			},
		},
	}
	anchorID := tpkg.RandOutputIDs(1)[0]

	var nativeTokenIDs []iotago.NativeTokenID
	var utxoInputsNativeTokens []iotago.UTXOInput
	// all token accounts initially are empty
	balanceLoader := func(_ *iotago.NativeTokenID) (*iotago.BasicOutput, *iotago.UTXOInput) {
		return nil, &iotago.UTXOInput{}
	}

	var txb *AnchorTransactionBuilder
	var amounts map[int]uint64

	initialBalance := new(big.Int)
	balanceLoaderWithInitialBalance := func(id *iotago.NativeTokenID) (*iotago.BasicOutput, *iotago.UTXOInput) {
		for _, id1 := range nativeTokenIDs {
			if *id == id1 {
				ret := txb.newInternalTokenOutput(aliasID, *id)
				ret.NativeTokens[0].Amount = new(big.Int).Set(initialBalance)
				return ret, &iotago.UTXOInput{}
			}
		}
		return nil, &iotago.UTXOInput{}
	}

	var numTokenIDs int

	initTest := func() {
		txb = NewAnchorTransactionBuilder(anchor, anchorID, balanceLoader, nil, nil,
			*transaction.NewStorageDepositEstimate(),
		)
		amounts = make(map[int]uint64)

		nativeTokenIDs = make([]iotago.NativeTokenID, 0)
		utxoInputsNativeTokens = make([]iotago.UTXOInput, 0)

		for i := 0; i < numTokenIDs; i++ {
			nativeTokenIDs = append(nativeTokenIDs, testiotago.RandNativeTokenID())
			utxoInputsNativeTokens = append(utxoInputsNativeTokens, testiotago.RandUTXOInput())
		}
	}
	runConsume := func(numRun int, amountNative uint64, addBaseTokensToDustMinimum ...uint64) {
		deposit := uint64(0)
		for i := 0; i < numRun; i++ {
			idx := i % numTokenIDs
			s := amounts[idx]
			amounts[idx] = s + amountNative

			deposit += consumeUTXO(t, txb, nativeTokenIDs[idx], amountNative, addBaseTokensToDustMinimum...)

			_, _, err := txb.Totals()
			require.NoError(t, err)
		}
		sumIN, sumOUT, err := txb.Totals()
		require.NoError(t, err)
		expectedDust := txb.dustDepositAssumption.AnchorOutput
		if numRun < numTokenIDs {
			expectedDust += uint64(numRun) * txb.dustDepositAssumption.NativeTokenOutput
		} else {
			expectedDust += uint64(numTokenIDs) * txb.dustDepositAssumption.NativeTokenOutput
		}
		require.EqualValues(t, int(txb.dustDepositAssumption.AnchorOutput), sumIN.TotalBaseTokensInDustDeposit)
		require.EqualValues(t, int(expectedDust), sumOUT.TotalBaseTokensInDustDeposit)
	}
	runCreateBuilderAndConsumeRandomly := func(numRun int, amount uint64) {
		txb = NewAnchorTransactionBuilder(anchor, anchorID, balanceLoader, nil, nil,
			*transaction.NewStorageDepositEstimate(),
		)
		amounts = make(map[int]uint64)

		deposit := uint64(0)
		for i := 0; i < numRun; i++ {
			idx := rand.Intn(numTokenIDs)
			amounts[idx] += amount
			deposit += consumeUTXO(t, txb, nativeTokenIDs[idx], amount)

			_, _, err := txb.Totals()
			require.NoError(t, err)
		}
		sumIN, sumOUT, err := txb.Totals()
		require.NoError(t, err)

		expectedBaseTokens := initialTotalBaseTokens - txb.dustDepositAssumption.AnchorOutput + deposit
		require.EqualValues(t, expectedBaseTokens, int(sumIN.TotalBaseTokensInL2Accounts))
		expectedBaseTokens -= uint64(len(amounts) * int(txb.dustDepositAssumption.NativeTokenOutput))
		require.EqualValues(t, expectedBaseTokens, int(sumOUT.TotalBaseTokensInL2Accounts))
	}

	runPostRequest := func(n int, amount uint64) uint64 {
		ret := uint64(0)
		for i := 0; i < n; i++ {
			idx := i % numTokenIDs
			ret += addOutput(txb, amount, nativeTokenIDs[idx])
			_, _, err := txb.Totals()
			require.NoError(t, err)
		}
		return ret
	}

	runPostRequestRandomly := func(n int, amount uint64) uint64 {
		ret := uint64(0)
		for i := 0; i < n; i++ {
			idx := rand.Intn(numTokenIDs)
			ret += addOutput(txb, amount, nativeTokenIDs[idx])
			_, _, err := txb.Totals()
			require.NoError(t, err)
		}
		return ret
	}

	t.Run("consistency check 0", func(t *testing.T) {
		const runTimes = 3
		const testAmount = 10
		numTokenIDs = 4

		initTest()
		runConsume(runTimes, testAmount)

		essence, _ := txb.BuildTransactionEssence(state.RandL1Commitment())

		essenceBytes, err := essence.Serialize(serializer.DeSeriModeNoValidation, nil)
		require.NoError(t, err)
		t.Logf("essence bytes len = %d", len(essenceBytes))
	})
	t.Run("consistency check 1", func(t *testing.T) {
		const runTimes = 7
		const testAmount = 10
		numTokenIDs = 4

		initTest()
		runConsume(runTimes, testAmount)

		essence, _ := txb.BuildTransactionEssence(state.RandL1Commitment())

		essenceBytes, err := essence.Serialize(serializer.DeSeriModeNoValidation, nil)
		require.NoError(t, err)
		t.Logf("essence bytes len = %d", len(essenceBytes))
	})
	t.Run("consistency check 2", func(t *testing.T) {
		const runTimes = 100
		const testAmount = 100
		numTokenIDs = 5

		initTest()
		runConsume(runTimes, testAmount)
		essence, _ := txb.BuildTransactionEssence(state.RandL1Commitment())

		essenceBytes, err := essence.Serialize(serializer.DeSeriModeNoValidation, nil)
		require.NoError(t, err)
		t.Logf("essence bytes len = %d", len(essenceBytes))
	})
	t.Run("consistency check 3", func(t *testing.T) {
		const runTimes = 100
		const testAmount = 10
		numTokenIDs = 4

		initTest()
		runCreateBuilderAndConsumeRandomly(runTimes, testAmount)

		essence, _ := txb.BuildTransactionEssence(state.RandL1Commitment())

		essenceBytes, err := essence.Serialize(serializer.DeSeriModeNoValidation, nil)
		require.NoError(t, err)
		t.Logf("essence bytes len = %d", len(essenceBytes))
	})
	t.Run("exceed inputs", func(t *testing.T) {
		const runTimes = 150
		const testAmount = 10
		numTokenIDs = 4

		initTest()
		err := panicutil.CatchPanicReturnError(func() {
			runConsume(runTimes, testAmount)
		}, vmexceptions.ErrInputLimitExceeded)
		require.Error(t, err, vmexceptions.ErrInputLimitExceeded)

		_, _, err = txb.Totals()
		require.NoError(t, err)
		essence, _ := txb.BuildTransactionEssence(state.RandL1Commitment())

		essenceBytes, err := essence.Serialize(serializer.DeSeriModeNoValidation, nil)
		require.NoError(t, err)
		t.Logf("essence bytes len = %d", len(essenceBytes))
	})
	t.Run("exceeded outputs 1", func(t *testing.T) {
		const runTimesInputs = 100
		const runTimesOutputs = 130
		numTokenIDs = 5

		initTest()
		runConsume(runTimesInputs, 10, 1000)
		_, _, err := txb.Totals()
		require.NoError(t, err)

		err = panicutil.CatchPanicReturnError(func() {
			runPostRequest(runTimesOutputs, 1)
		}, vmexceptions.ErrOutputLimitExceeded)

		require.Error(t, err, vmexceptions.ErrOutputLimitExceeded)

		_, _, err = txb.Totals()
		require.NoError(t, err)
		essence, _ := txb.BuildTransactionEssence(state.RandL1Commitment())

		essenceBytes, err := essence.Serialize(serializer.DeSeriModeNoValidation, nil)
		require.NoError(t, err)
		t.Logf("essence bytes len = %d", len(essenceBytes))
	})
	t.Run("exceeded outputs 2", func(t *testing.T) {
		const runTimesInputs = 120
		const runTimesOutputs = 130
		numTokenIDs = 5

		initTest()
		runConsume(runTimesInputs, 10, 1000)
		_, _, err := txb.Totals()
		require.NoError(t, err)

		err = panicutil.CatchPanicReturnError(func() {
			runPostRequestRandomly(runTimesOutputs, 1)
		}, vmexceptions.ErrOutputLimitExceeded)

		require.Error(t, err, vmexceptions.ErrOutputLimitExceeded)

		_, _, err = txb.Totals()
		require.NoError(t, err)
		essence, _ := txb.BuildTransactionEssence(state.RandL1Commitment())

		essenceBytes, err := essence.Serialize(serializer.DeSeriModeNoValidation, nil)
		require.NoError(t, err)
		t.Logf("essence bytes len = %d", len(essenceBytes))
	})
	t.Run("randomize", func(t *testing.T) {
		const runTimes = 30
		numTokenIDs = 5

		initTest()
		for _, id := range nativeTokenIDs {
			consumeUTXO(t, txb, id, 10)
		}

		for i := 0; i < runTimes; i++ {
			idx1 := rand.Intn(numTokenIDs)
			consumeUTXO(t, txb, nativeTokenIDs[idx1], 1, 1000)
			idx2 := rand.Intn(numTokenIDs)
			addOutput(txb, 1, nativeTokenIDs[idx2])
			_, _, err := txb.Totals()
			require.NoError(t, err)
		}
		_, _, err := txb.Totals()
		require.NoError(t, err)

		t.Logf(">>>>>>>>>> \n%s", txb.String())

		essence, _ := txb.BuildTransactionEssence(state.RandL1Commitment())

		essenceBytes, err := essence.Serialize(serializer.DeSeriModeNoValidation, nil)
		require.NoError(t, err)
		t.Logf("essence bytes len = %d", len(essenceBytes))
	})
	t.Run("clone", func(t *testing.T) {
		const runTimes = 7
		numTokenIDs = 5

		initTest()
		for _, id := range nativeTokenIDs {
			consumeUTXO(t, txb, id, 100)
		}
		totals, _, err := txb.Totals()
		require.NoError(t, err)

		txbClone := txb.Clone()
		totalsClone, _, err := txbClone.Totals()
		require.NoError(t, err)
		require.NoError(t, totals.BalancedWith(totalsClone))

		for i := 0; i < runTimes; i++ {
			idx1 := rand.Intn(numTokenIDs)
			consumeUTXO(t, txb, nativeTokenIDs[idx1], 1, 100)
			idx2 := rand.Intn(numTokenIDs)
			addOutput(txb, 1, nativeTokenIDs[idx2])
			_, _, err = txb.Totals()
			require.NoError(t, err)
		}

		totalsClone, _, err = txbClone.Totals()
		require.NoError(t, err)
		require.NoError(t, totals.BalancedWith(totalsClone))
	})
	t.Run("in balance 1", func(t *testing.T) {
		numTokenIDs = 5

		initialBalance.SetUint64(100)
		balanceLoader = balanceLoaderWithInitialBalance
		initTest()

		// send 90 < 100 which is on-chain. 10 must be left and dust deposit should not disappear
		addOutput(txb, 90, nativeTokenIDs[0])

		totalIn, totalOut, err := txb.Totals()
		require.NoError(t, err)
		require.EqualValues(t, int(initialTotalBaseTokens-txb.dustDepositAssumption.AnchorOutput), int(totalOut.TotalBaseTokensInL2Accounts+totalOut.SentOutBaseTokens))
		require.EqualValues(t, int(txb.dustDepositAssumption.NativeTokenOutput+txb.dustDepositAssumption.AnchorOutput), int(totalIn.TotalBaseTokensInDustDeposit))
		require.EqualValues(t, int(txb.dustDepositAssumption.NativeTokenOutput+txb.dustDepositAssumption.AnchorOutput), int(totalOut.TotalBaseTokensInDustDeposit))
		beforeTokens, afterTokens := txb.InternalNativeTokenBalances()

		require.True(t, beforeTokens[nativeTokenIDs[0]].Cmp(new(big.Int).SetInt64(100)) == 0)
		require.True(t, afterTokens[nativeTokenIDs[0]].Cmp(new(big.Int).SetInt64(10)) == 0)
		t.Logf(">>>>>>>>>> \n%s", txb.String())

		essence, _ := txb.BuildTransactionEssence(state.RandL1Commitment())

		essenceBytes, err := essence.Serialize(serializer.DeSeriModeNoValidation, nil)
		require.NoError(t, err)
		t.Logf("essence bytes len = %d", len(essenceBytes))
	})
	t.Run("in balance 2", func(t *testing.T) {
		numTokenIDs = 5

		initialBalance.SetUint64(100)
		balanceLoader = balanceLoaderWithInitialBalance
		initTest()

		// output will close internal account
		sentOut := addOutput(txb, 100, nativeTokenIDs[0])

		totalIn, totalOut, err := txb.Totals()
		require.NoError(t, err)
		require.EqualValues(t, int(txb.dustDepositAssumption.NativeTokenOutput+txb.dustDepositAssumption.AnchorOutput), int(totalIn.TotalBaseTokensInDustDeposit))
		require.EqualValues(t, int(sentOut), totalOut.SentOutBaseTokens)
		require.EqualValues(t, int(initialTotalBaseTokens-txb.dustDepositAssumption.AnchorOutput-sentOut+txb.dustDepositAssumption.NativeTokenOutput), int(txb.totalBaseTokensInL2Accounts))
		require.EqualValues(t, txb.dustDepositAssumption.AnchorOutput, int(totalOut.TotalBaseTokensInDustDeposit))
		beforeTokens, afterTokens := txb.InternalNativeTokenBalances()

		require.True(t, beforeTokens[nativeTokenIDs[0]].Cmp(new(big.Int).SetInt64(100)) == 0)
		_, ok := afterTokens[nativeTokenIDs[0]]
		require.False(t, ok)

		t.Logf(">>>>>>>>>> \n%s", txb.String())

		essence, _ := txb.BuildTransactionEssence(state.RandL1Commitment())
		require.EqualValues(t, 2, len(essence.Inputs))
		require.EqualValues(t, 2, len(essence.Outputs))

		essenceBytes, err := essence.Serialize(serializer.DeSeriModeNoValidation, nil)
		require.NoError(t, err)
		t.Logf("essence bytes len = %d", len(essenceBytes))
	})
	t.Run("in balance 3", func(t *testing.T) {
		numTokenIDs = 5

		initialBalance.SetUint64(100)
		balanceLoader = balanceLoaderWithInitialBalance
		initTest()

		// send 90 < 100 which is on-chain. 10 must be left and dust deposit should not disappear
		for i := range nativeTokenIDs {
			addOutput(txb, 100, nativeTokenIDs[i])
		}

		totalIn, totalOut, err := txb.Totals()
		require.NoError(t, err)
		expectedBaseTokens := initialTotalBaseTokens - txb.dustDepositAssumption.AnchorOutput + txb.dustDepositAssumption.NativeTokenOutput*uint64(len(nativeTokenIDs))
		require.EqualValues(t, expectedBaseTokens, int(totalOut.TotalBaseTokensInL2Accounts+totalOut.SentOutBaseTokens))
		require.EqualValues(t, int(txb.dustDepositAssumption.NativeTokenOutput)*len(nativeTokenIDs)+int(txb.dustDepositAssumption.AnchorOutput), int(totalIn.TotalBaseTokensInDustDeposit))
		require.EqualValues(t, txb.dustDepositAssumption.AnchorOutput, int(totalOut.TotalBaseTokensInDustDeposit))
		beforeTokens, afterTokens := txb.InternalNativeTokenBalances()

		for i := range nativeTokenIDs {
			require.True(t, beforeTokens[nativeTokenIDs[i]].Cmp(new(big.Int).SetInt64(100)) == 0)
			_, ok := afterTokens[nativeTokenIDs[i]]
			require.False(t, ok)
		}

		t.Logf(">>>>>>>>>> \n%s", txb.String())

		essence, _ := txb.BuildTransactionEssence(state.RandL1Commitment())
		require.EqualValues(t, 6, len(essence.Inputs))
		require.EqualValues(t, 6, len(essence.Outputs))

		essenceBytes, err := essence.Serialize(serializer.DeSeriModeNoValidation, nil)
		require.NoError(t, err)
		t.Logf("essence bytes len = %d", len(essenceBytes))
	})
}

func TestDustDeposit(t *testing.T) {
	reqMetadata := iscp.RequestMetadata{
		SenderContract: 0,
		TargetContract: 0,
		EntryPoint:     0,
		Params:         dict.New(),
		Allowance:      iscp.NewEmptyAllowance(),
		GasBudget:      0,
	}
	t.Run("calc dust assumptions", func(t *testing.T) {
		d := transaction.NewStorageDepositEstimate()
		t.Logf("dust deposit assumptions:\n%s", d.String())

		d1, err := transaction.StorageDepositAssumptionFromBytes(d.Bytes())
		require.NoError(t, err)
		require.EqualValues(t, d.AnchorOutput, d1.AnchorOutput)
		require.EqualValues(t, d.NativeTokenOutput, d1.NativeTokenOutput)
	})
	t.Run("adjusts the output amount to the correct bytecost when needed", func(t *testing.T) {
		assets := iscp.NewEmptyAssets()
		out := transaction.MakeBasicOutput(
			&iotago.Ed25519Address{},
			&iotago.Ed25519Address{1, 2, 3},
			assets,
			&reqMetadata,
			iscp.SendOptions{},
		)
		expected := parameters.L1.Protocol.RentStructure.MinRent(out)
		require.Equal(t, out.Deposit(), expected)
	})
	t.Run("keeps the same amount of base tokens when enough for dust cost", func(t *testing.T) {
		assets := iscp.NewFungibleTokens(10000, nil)
		out := transaction.MakeBasicOutput(
			&iotago.Ed25519Address{},
			&iotago.Ed25519Address{1, 2, 3},
			assets,
			&reqMetadata,
			iscp.SendOptions{},
		)
		require.GreaterOrEqual(t, out.Deposit(), out.VBytes(&parameters.L1.Protocol.RentStructure, nil))
	})
}

func TestFoundries(t *testing.T) {
	const initialTotalBaseTokens = 1 * iscp.Mi
	addr := tpkg.RandEd25519Address()
	stateMetadata := hashing.HashStrings("test")
	aliasID := rndAliasID()
	anchor := &iotago.AliasOutput{
		Amount:       initialTotalBaseTokens,
		NativeTokens: nil,
		AliasID:      aliasID,
		Conditions: iotago.UnlockConditions{
			&iotago.StateControllerAddressUnlockCondition{Address: addr},
			&iotago.GovernorAddressUnlockCondition{Address: addr},
		},
		StateIndex:     0,
		StateMetadata:  stateMetadata[:],
		FoundryCounter: 0,
		Features: iotago.Features{
			&iotago.SenderFeature{
				Address: aliasID.ToAddress(),
			},
		},
	}
	anchorID := tpkg.RandOutputIDs(1)[0]

	var nativeTokenIDs []iotago.NativeTokenID
	var utxoInputsNativeTokens []iotago.UTXOInput
	// all token accounts initially are empty
	balanceLoader := func(_ *iotago.NativeTokenID) (*iotago.BasicOutput, *iotago.UTXOInput) {
		return nil, &iotago.UTXOInput{}
	}
	var txb *AnchorTransactionBuilder

	var numTokenIDs int

	initTest := func() {
		txb = NewAnchorTransactionBuilder(anchor, anchorID, balanceLoader, nil, nil,
			*transaction.NewStorageDepositEstimate(),
		)

		nativeTokenIDs = make([]iotago.NativeTokenID, 0)
		utxoInputsNativeTokens = make([]iotago.UTXOInput, 0)

		for i := 0; i < numTokenIDs; i++ {
			nativeTokenIDs = append(nativeTokenIDs, testiotago.RandNativeTokenID())
			utxoInputsNativeTokens = append(utxoInputsNativeTokens, testiotago.RandUTXOInput())
		}
	}
	createNFoundries := func(n int) {
		for i := 0; i < n; i++ {
			sn, _ := txb.CreateNewFoundry(
				&iotago.SimpleTokenScheme{MaximumSupply: big.NewInt(10_000_000), MeltedTokens: util.Big0, MintedTokens: util.Big0},
				nil,
			)
			require.EqualValues(t, i+1, int(sn))

			tin, tout, err := txb.Totals()
			require.NoError(t, err)
			t.Logf("%d. total base tokens IN: %d, total base tokens OUT: %d", i, tin.TotalBaseTokensInL2Accounts, tout.TotalBaseTokensInL2Accounts)
			t.Logf("%d. dust deposit IN: %d, dust deposit OUT: %d", i, tin.TotalBaseTokensInDustDeposit, tout.TotalBaseTokensInDustDeposit)
			t.Logf("%d. num foundries: %d", i, txb.nextFoundrySerialNumber())
		}
	}
	t.Run("create foundry ok", func(t *testing.T) {
		initTest()
		createNFoundries(3)
		essence, _ := txb.BuildTransactionEssence(state.RandL1Commitment())
		essenceBytes, err := essence.Serialize(serializer.DeSeriModeNoValidation, nil)
		require.NoError(t, err)
		t.Logf("essence bytes len = %d", len(essenceBytes))
	})
	t.Run("create foundry not enough", func(t *testing.T) {
		initTest()
		err := panicutil.CatchPanicReturnError(func() {
			createNFoundries(5000)
		}, vmexceptions.ErrNotEnoughFundsForInternalDustDeposit)
		require.Error(t, err, vmexceptions.ErrNotEnoughFundsForInternalDustDeposit)

		essence, _ := txb.BuildTransactionEssence(state.RandL1Commitment())
		essenceBytes, err := essence.Serialize(serializer.DeSeriModeNoValidation, nil)
		require.NoError(t, err)
		t.Logf("essence bytes len = %d", len(essenceBytes))
	})
}

func TestSerDe(t *testing.T) {
	t.Run("serde BasicOutput", func(t *testing.T) {
		reqMetadata := iscp.RequestMetadata{
			SenderContract: 0,
			TargetContract: 0,
			EntryPoint:     0,
			Params:         dict.New(),
			Allowance:      iscp.NewEmptyAllowance(),
			GasBudget:      0,
		}
		assets := iscp.NewEmptyAssets()
		out := transaction.MakeBasicOutput(
			&iotago.Ed25519Address{},
			&iotago.Ed25519Address{1, 2, 3},
			assets,
			&reqMetadata,
			iscp.SendOptions{},
		)
		data, err := out.Serialize(serializer.DeSeriModeNoValidation, nil)
		require.NoError(t, err)
		outBack := &iotago.BasicOutput{}
		_, err = outBack.Deserialize(data, serializer.DeSeriModeNoValidation, nil)
		require.NoError(t, err)
		condSet := out.Conditions.MustSet()
		condSetBack := outBack.Conditions.MustSet()
		require.True(t, condSet[iotago.UnlockConditionAddress].Equal(condSetBack[iotago.UnlockConditionAddress]))
		require.EqualValues(t, out.Deposit(), outBack.Amount)
		require.EqualValues(t, 0, len(outBack.NativeTokens))
		require.True(t, outBack.Features.Equal(out.Features))
	})
	t.Run("serde FoundryOutput", func(t *testing.T) {
		out := &iotago.FoundryOutput{
			Conditions: iotago.UnlockConditions{
				&iotago.ImmutableAliasUnlockCondition{Address: tpkg.RandAliasAddress()},
			},
			Amount:       1337,
			NativeTokens: nil,
			SerialNumber: 5,
			TokenScheme: &iotago.SimpleTokenScheme{
				MintedTokens:  big.NewInt(200),
				MeltedTokens:  big.NewInt(0),
				MaximumSupply: big.NewInt(2000),
			},
			Features: nil,
		}
		data, err := out.Serialize(serializer.DeSeriModeNoValidation, nil)
		require.NoError(t, err)
		outBack := &iotago.FoundryOutput{}
		_, err = outBack.Deserialize(data, serializer.DeSeriModeNoValidation, nil)
		require.NoError(t, err)
		require.True(t, identicalFoundries(out, outBack))
	})
}
