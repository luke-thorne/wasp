// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

// TODO: Test connect/reconnect - start node conn, and later the hornet.
// TODO: Test connect/reconnect - on a running node stop and later restart hornet.

package tests

import (
	"testing"

	"github.com/iotaledger/hive.go/logger"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/nodeclient"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/nodeconn"
	"github.com/iotaledger/wasp/packages/testutil"
	"github.com/iotaledger/wasp/packages/testutil/testlogger"
	"github.com/iotaledger/wasp/packages/testutil/testpeers"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/stretchr/testify/require"
)

func createChain(t *testing.T) *iscp.ChainID {
	originator := cryptolib.NewKeyPair()
	layer1Client := nodeconn.NewL1Client(l1.Config, testlogger.NewLogger(t))
	layer1Client.RequestFunds(originator.Address())
	utxoMap, err := layer1Client.OutputMap(originator.Address())
	require.NoError(t, err)

	var utxoIDs iotago.OutputIDs
	for id := range utxoMap {
		utxoIDs = append(utxoIDs, id)
	}

	originTx, chainID, err := transaction.NewChainOriginTransaction(
		originator,
		originator.Address(),
		originator.Address(),
		0,
		utxoMap,
		utxoIDs,
	)
	require.NoError(t, err)
	err = layer1Client.PostTx(originTx)
	require.NoError(t, err)

	return chainID
}

func TestNodeConn(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping nodeconn test in short mode")
	}

	l1.StartPrivtangleIfNecessary(t.Logf)

	log := testlogger.NewLogger(t)
	defer log.Sync()
	peerCount := 1

	//
	// Start a peering network.
	// peeringID := peering.RandomPeeringID()
	peerNetIDs, peerIdentities := testpeers.SetupKeys(uint16(peerCount))
	networkLog := testlogger.WithLevel(log.Named("Network"), logger.LevelInfo, false)
	_, networkCloser := testpeers.SetupNet(
		peerNetIDs,
		peerIdentities,
		testutil.NewPeeringNetReliable(networkLog),
		networkLog,
	)
	t.Logf("Peering network created.")

	nc := nodeconn.New(l1.Config, log)

	//
	// Check milestone attach/detach.
	mChan := make(chan *nodeclient.MilestoneInfo, 10)
	mSub := nc.AttachMilestones(func(m *nodeclient.MilestoneInfo) {
		mChan <- m
	})
	<-mChan
	nc.DetachMilestones(mSub)

	//
	// Check the chain operations.
	chainID := createChain(t)
	chainOuts := make(map[iotago.OutputID]iotago.Output)
	chainOICh := make(chan iotago.OutputID)
	chainStateOuts := make(map[iotago.OutputID]iotago.Output)
	chainStateOutsICh := make(chan iotago.OutputID)
	nc.RegisterChain(
		chainID,
		func(oi iotago.OutputID, o iotago.Output) {
			chainStateOuts[oi] = o
			chainStateOutsICh <- oi
		},
		func(oi iotago.OutputID, o iotago.Output) {
			chainOuts[oi] = o
			chainOICh <- oi
		})

	client := nodeconn.NewL1Client(l1.Config, log)
	// Post a TX directly, and wait for it in the message stream (e.g. a request).
	err := client.RequestFunds(chainID.AsAddress())
	require.NoError(t, err)
	t.Logf("Waiting for outputs posted via tangle...")
	oid := <-chainOICh
	t.Logf("Waiting for outputs posted via tangle... Done, have %v=%v", oid.ToHex(), chainOuts[oid])

	// Post a TX via the NodeConn (e.g. alias output).
	tiseCh := make(chan bool)
	tise, err := nc.AttachTxInclusionStateEvents(chainID, func(txID iotago.TransactionID, inclusionState string) {
		t.Logf("TX Inclusion state changed, txID=%v, state=%v", txID, inclusionState)
		if inclusionState == "included" {
			tiseCh <- true
		}
	})
	require.NoError(t, err)
	wallet := cryptolib.NewKeyPair()
	client.RequestFunds(wallet.Address())
	tx, err := nodeconn.MakeSimpleValueTX(client, wallet, chainID.AsAddress(), 1*iscp.Mi)
	require.NoError(t, err)
	err = nc.PublishStateTransaction(chainID, uint32(0), tx)
	require.NoError(t, err)
	t.Logf("Waiting for outputs posted via nodeConn...")
	oid = <-chainOICh
	t.Logf("Waiting for outputs posted via nodeConn... Done, have %v=%v", oid.ToHex(), chainOuts[oid])
	t.Logf("Waiting for TX incusion event...")
	<-tiseCh
	t.Logf("Waiting for TX incusion event... Done")

	nc.DetachTxInclusionStateEvents(chainID, tise)
	nc.UnregisterChain(chainID)

	//
	// Cleanup.
	require.NoError(t, networkCloser.Close())
}
