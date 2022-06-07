package testcore

import (
	"fmt"
	"testing"

	"github.com/iotaledger/wasp/packages/solo"
	"github.com/iotaledger/wasp/packages/vm/core/corecontracts"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/stretchr/testify/require"
)

func TestBlockInfoLatest(t *testing.T) {
	corecontracts.PrintWellKnownHnames()
	env := solo.New(t, &solo.InitOptions{AutoAdjustDustDeposit: true})
	chain := env.NewChain(nil, "chain1")

	bi := chain.GetLatestBlockInfo()
	require.NotNil(t, bi)
	require.EqualValues(t, 1, bi.BlockIndex)
	require.EqualValues(t, 1, bi.TotalRequests)
	require.EqualValues(t, 1, bi.NumSuccessfulRequests)
	require.EqualValues(t, 0, bi.NumOffLedgerRequests)
	t.Logf("%s", bi.String())
}

func TestBlockInfo(t *testing.T) {
	corecontracts.PrintWellKnownHnames()
	env := solo.New(t, &solo.InitOptions{AutoAdjustDustDeposit: true})
	chain := env.NewChain(nil, "chain1")

	bi, err := chain.GetBlockInfo(0)
	require.NoError(t, err)
	require.NotNil(t, bi)
	require.EqualValues(t, 0, bi.BlockIndex)
	require.EqualValues(t, 1, bi.TotalRequests)
	require.EqualValues(t, 1, bi.NumSuccessfulRequests)
	require.EqualValues(t, 0, bi.NumOffLedgerRequests)
	t.Logf("%s", bi.String())

	bi, err = chain.GetBlockInfo(1)
	require.NoError(t, err)
	require.NotNil(t, bi)
	require.EqualValues(t, 1, bi.BlockIndex)
	require.EqualValues(t, 1, bi.TotalRequests)
	require.EqualValues(t, 1, bi.NumSuccessfulRequests)
	require.EqualValues(t, 0, bi.NumOffLedgerRequests)
	t.Logf("%s", bi.String())
}

func TestBlockInfoLatestWithRequest(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustDustDeposit: true})

	ch := env.NewChain(nil, "chain1")

	err := ch.DepositIotasToL2(100_000, nil)
	require.NoError(t, err)

	bi := ch.GetLatestBlockInfo()
	t.Logf("after ch deployment:\n%s", bi.String())
	// uploading one blob
	_, err = ch.UploadBlob(nil, "field", "dummy blob data")
	require.NoError(t, err)

	bi = ch.GetLatestBlockInfo()
	require.NotNil(t, bi)
	require.EqualValues(t, 3, bi.BlockIndex)
	require.EqualValues(t, 1, bi.TotalRequests)
	require.EqualValues(t, 1, bi.NumSuccessfulRequests)
	require.EqualValues(t, 1, bi.NumOffLedgerRequests)
	t.Logf("%s", bi.String())
}

func TestBlockInfoSeveral(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustDustDeposit: true})
	ch := env.NewChain(nil, "chain1")

	err := ch.DepositIotasToL2(100_000, nil)
	require.NoError(t, err)

	const numReqs = 5
	for i := 0; i < numReqs; i++ {
		_, err := ch.UploadBlob(nil, "field", fmt.Sprintf("dummy blob data #%d", i))
		require.NoError(t, err)
	}

	bi := ch.GetLatestBlockInfo()
	require.EqualValues(t, 2+numReqs, int(bi.BlockIndex))

	for blockIndex := uint32(0); blockIndex <= bi.BlockIndex; blockIndex++ {
		bi1, err := ch.GetBlockInfo(blockIndex)
		require.NoError(t, err)
		require.NotNil(t, bi1)
		t.Logf("%s", bi1.String())
		require.EqualValues(t, blockIndex, bi1.BlockIndex)
		require.EqualValues(t, 1, bi1.TotalRequests)
		require.EqualValues(t, 1, bi1.NumSuccessfulRequests)
		require.LessOrEqual(t, bi1.NumOffLedgerRequests, bi1.TotalRequests)
	}
}

func TestRequestIsProcessed(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustDustDeposit: true})
	ch := env.NewChain(nil, "chain1")

	ch.MustDepositIotasToL2(10_000, nil)

	req := solo.NewCallParams(governance.Contract.Name, governance.FuncSetChainInfo.Name).
		WithGasBudget(100_000)
	tx, _, err := ch.PostRequestSyncTx(req, nil)
	require.NoError(t, err)

	reqs, err := env.RequestsForChain(tx, ch.ChainID)
	require.NoError(t, err)
	require.EqualValues(t, 1, len(reqs))

	bi := ch.GetLatestBlockInfo()
	require.NoError(t, err)
	require.True(t, ch.IsRequestProcessed(reqs[0].ID()))
	t.Logf("%s", bi.String())
}

func TestRequestReceipt(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustDustDeposit: true})
	ch := env.NewChain(nil, "chain1")

	ch.MustDepositIotasToL2(10_000, nil)

	req := solo.NewCallParams(governance.Contract.Name, governance.FuncSetChainInfo.Name).
		WithGasBudget(100_000)
	tx, _, err := ch.PostRequestSyncTx(req, nil)
	require.NoError(t, err)

	reqs, err := env.RequestsForChain(tx, ch.ChainID)
	require.NoError(t, err)
	require.EqualValues(t, 1, len(reqs))
	require.True(t, ch.IsRequestProcessed(reqs[0].ID()))

	receipt, ok := ch.GetRequestReceipt(reqs[0].ID())
	require.True(t, ok)
	a := reqs[0].Bytes()
	b := receipt.Request.Bytes()
	require.Equal(t, a, b)
	require.Nil(t, receipt.Error)
	require.EqualValues(t, 3, int(receipt.BlockIndex))
	require.EqualValues(t, 0, receipt.RequestIndex)
	t.Logf("%s", receipt.String())
}

func TestRequestReceiptsForBlocks(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustDustDeposit: true})
	ch := env.NewChain(nil, "chain1")

	ch.MustDepositIotasToL2(10_000, nil)

	req := solo.NewCallParams(governance.Contract.Name, governance.FuncSetChainInfo.Name).
		WithGasBudget(100_000)
	tx, _, err := ch.PostRequestSyncTx(req, nil)
	require.NoError(t, err)

	reqs, err := env.RequestsForChain(tx, ch.ChainID)
	require.NoError(t, err)
	require.EqualValues(t, 1, len(reqs))

	require.True(t, ch.IsRequestProcessed(reqs[0].ID()))

	recs := ch.GetRequestReceiptsForBlock(3)
	require.EqualValues(t, 1, len(recs))
	require.EqualValues(t, reqs[0].ID(), recs[0].Request.ID())
	t.Logf("%s\n", recs[0].String())
}

func TestRequestIDsForBlocks(t *testing.T) {
	env := solo.New(t, &solo.InitOptions{AutoAdjustDustDeposit: true})
	ch := env.NewChain(nil, "chain1")

	ch.MustDepositIotasToL2(10_000, nil)

	req := solo.NewCallParams(governance.Contract.Name, governance.FuncSetChainInfo.Name).
		WithGasBudget(100_000)
	tx, _, err := ch.PostRequestSyncTx(req, nil)
	require.NoError(t, err)

	reqs, err := env.RequestsForChain(tx, ch.ChainID)
	require.NoError(t, err)
	require.EqualValues(t, 1, len(reqs))

	require.True(t, ch.IsRequestProcessed(reqs[0].ID()))

	ids := ch.GetRequestIDsForBlock(3)
	require.EqualValues(t, 1, len(ids))
	require.EqualValues(t, reqs[0].ID(), ids[0])
}
