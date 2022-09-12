package tests

import (
	"time"

	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/contracts/native/inccounter"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/vm/core/root"
	"github.com/stretchr/testify/require"
)

const nativeIncCounterSCName = "NativeIncCounter"

var nativeIncCounterSCHname = isc.Hn(nativeIncCounterSCName)

func (e *ChainEnv) deployNativeIncCounterSC(initCounter ...int) *iotago.Transaction {
	counterStartValue := 42
	if len(initCounter) > 0 {
		counterStartValue = initCounter[0]
	}
	description := "testing contract deployment with inccounter" //nolint:goconst
	programHash := inccounter.Contract.ProgramHash

	tx, err := e.Chain.DeployContract(nativeIncCounterSCName, programHash.String(), description, map[string]interface{}{
		inccounter.VarCounter: counterStartValue,
		root.ParamName:        nativeIncCounterSCName,
	})
	require.NoError(e.t, err)

	blockIndex, err := e.Chain.BlockIndex()
	require.NoError(e.t, err)
	require.Greater(e.t, blockIndex, uint32(1))

	// wait until all nodes (including access nodes) are at least at block `blockIndex`
	retries := 0
	for i := 1; i < len(e.Chain.AllPeers); i++ {
		peerIdx := e.Chain.AllPeers[i]
		b, err := e.Chain.BlockIndex(peerIdx)
		if err != nil || b < blockIndex {
			if retries >= 10 {
				e.t.Fatalf("error on deployIncCounterSC, failed to wait for all peers to be on the same block index after 5 retries. Peer index: %d", peerIdx)
			}
			// retry (access nodes might take slightly more time to sync)
			retries++
			i--
			time.Sleep(1 * time.Second)
			continue
		}
	}

	e.checkCoreContracts()

	for i := range e.Chain.AllPeers {
		contractRegistry, err := e.Chain.ContractRegistry(i)
		require.NoError(e.t, err)

		cr := contractRegistry[nativeIncCounterSCHname]
		require.NotNil(e.t, cr)

		require.EqualValues(e.t, programHash, cr.ProgramHash)
		require.EqualValues(e.t, description, cr.Description)
		require.EqualValues(e.t, cr.Name, nativeIncCounterSCName)

		counterValue, err := e.Chain.GetCounterValue(nativeIncCounterSCHname, i)
		require.NoError(e.t, err)
		require.EqualValues(e.t, counterStartValue, counterValue)
	}

	return tx
}
