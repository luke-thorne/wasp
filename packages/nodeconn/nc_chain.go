// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package nodeconn

import (
	"context"
	"encoding/json"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/nodeclient"
	"github.com/iotaledger/wasp/packages/chain"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/parameters"
	"golang.org/x/xerrors"
)

// nodeconn_chain is responsible for maintaining the information related to a single chain.
type ncChain struct {
	nc                 *nodeConn
	chainID            *isc.ChainID
	outputHandler      func(iotago.OutputID, iotago.Output)
	stateOutputHandler func(iotago.OutputID, iotago.Output)
	inclusionStates    *events.Event
	log                *logger.Logger
}

func newNCChain(
	nc *nodeConn,
	chainID *isc.ChainID,
	stateOutputHandler,
	outputHandler func(iotago.OutputID, iotago.Output),
) *ncChain {
	inclusionStates := events.NewEvent(func(handler interface{}, params ...interface{}) {
		handler.(chain.NodeConnectionInclusionStateHandlerFun)(params[0].(iotago.TransactionID), params[1].(string))
	})
	ncc := ncChain{
		nc:                 nc,
		chainID:            chainID,
		outputHandler:      outputHandler,
		stateOutputHandler: stateOutputHandler,
		inclusionStates:    inclusionStates,
		log:                nc.log.Named(chainID.String()[:6]),
	}
	ncc.run()
	return &ncc
}

func (ncc *ncChain) Key() string {
	return ncc.chainID.Key()
}

func (ncc *ncChain) Close() {
	// Nothing. The ncc.nc.ctx is used for that.
}

func (ncc *ncChain) handleMetadataUpdate(txID iotago.TransactionID, metaDataChange *nodeclient.BlockMetadataResponse) {
	if metaDataChange.LedgerInclusionState != "" {
		str, err := json.Marshal(metaDataChange)
		if err != nil {
			ncc.log.Errorf("publishing transaction %v: unexpected error trying to marshal msgMetadataChange: %s", isc.TxID(txID), err)
		} else {
			ncc.log.Debugf("publishing transaction %v: msgMetadataChange: %s", isc.TxID(txID), str)
		}
		ncc.inclusionStates.Trigger(txID, metaDataChange.LedgerInclusionState)
	}
}

func (ncc *ncChain) PublishTransaction(tx *iotago.Transaction, timeout ...time.Duration) error {
	ctxWithTimeout, cancelContext := newCtx(ncc.nc.ctx, timeout...)
	defer cancelContext()

	txID, err := tx.ID()
	if err != nil {
		return xerrors.Errorf("publishing transaction: failed to get a tx ID: %w", err)
	}
	ncc.log.Debugf("publishing transaction %v...", isc.TxID(txID))
	txMsg, err := ncc.nc.doPostTx(ctxWithTimeout, tx)
	if err != nil {
		return err
	}

	txMsgID, err := txMsg.ID()
	if err != nil {
		return xerrors.Errorf("publishing transaction %v: failed to extract a tx Block ID: %w", isc.TxID(txID), err)
	}
	msgMetaChanges, subInfo := ncc.nc.mqttClient.BlockMetadataChange(txMsgID)
	ncc.log.Debugf("publishing transaction %v: posted", isc.TxID(txID))

	if subInfo.Error() != nil {
		return xerrors.Errorf("publishing transaction %v: failed to subscribe: %w", isc.TxID(txID), subInfo.Error())
	}

	go func() {
		ncc.log.Debugf("publishing transaction %v: listening to inclusion states...", isc.TxID(txID))

		for {
			select {
			case msgMetaChange := <-msgMetaChanges:
				ncc.handleMetadataUpdate(txID, msgMetaChange)
			case <-ctxWithTimeout.Done():
				return
			case <-time.After(20 * time.Second):
				ncc.log.Debugf("FOUND A BLOCKING METADATA CHANNEL\n")

				for i := 0; i < 10; i++ {
					state, err := ncc.nc.nodeAPIClient.BlockMetadataByBlockID(ctxWithTimeout, txMsgID)
					if err != nil {
						ncc.log.Debugf("BlockMEtaDataByBlockId ERROR: %v\n", err)
						time.Sleep(1 * time.Second)
						continue
					}

					if state != nil && state.LedgerInclusionState != "" {
						ncc.handleMetadataUpdate(txID, state)
						return
					}

					time.Sleep(1 * time.Second)
				}
			}
		}
	}()

	// TODO should promote/re-attach logic not be blocking?
	return ncc.nc.waitUntilConfirmed(ctxWithTimeout, txMsg)
}

func (ncc *ncChain) PullStateOutputByID(id iotago.OutputID) {
	ctxWithTimeout, cancelContext := newCtx(ncc.nc.ctx)
	res, err := ncc.nc.nodeAPIClient.OutputByID(ctxWithTimeout, id)
	cancelContext()
	if err != nil {
		ncc.log.Errorf("PullOutputByID: error querying API - chainID %s OutputID %s:  %s", ncc.chainID, id, err)
		return
	}
	out, err := res.Output()
	if err != nil {
		ncc.log.Errorf("PullOutputByID: error getting output from response - chainID %s OutputID %s:  %s", ncc.chainID, id, err)
		return
	}
	ncc.stateOutputHandler(id, out)
}

func (ncc *ncChain) queryChainUTXOs() {
	bech32Addr := ncc.chainID.AsAddress().Bech32(parameters.L1().Protocol.Bech32HRP)
	queries := []nodeclient.IndexerQuery{
		&nodeclient.BasicOutputsQuery{AddressBech32: bech32Addr},
		&nodeclient.FoundriesQuery{AliasAddressBech32: bech32Addr},
		&nodeclient.NFTsQuery{AddressBech32: bech32Addr},
		// &nodeclient.AliasesQuery{GovernorBech32: bech32Addr}, // TODO chains can't own alias outputs for now
	}

	var ctxWithTimeout context.Context
	var cancelContext context.CancelFunc
	for _, query := range queries {
		if ctxWithTimeout != nil && ctxWithTimeout.Err() == nil {
			// cancel the ctx of the last query
			cancelContext()
		}
		// TODO what should be an adequate timeout for each of these queries?
		ctxWithTimeout, cancelContext = newCtx(ncc.nc.ctx)
		res, err := ncc.nc.indexerClient.Outputs(ctxWithTimeout, query)
		if err != nil {
			ncc.log.Warnf("failed to query address outputs: %v", err)
			continue
		}

		for res.Next() {
			if res.Error != nil {
				ncc.log.Warnf("error iterating indexer results: %v", err)
			}
			outs, err := res.Outputs()
			if err != nil {
				ncc.log.Warnf("failed to fetch address outputs: %v", err)
				continue
			}
			oids, err := res.Response.Items.OutputIDs()
			if err != nil {
				ncc.log.Warnf("failed to get outputIDs from response items: %v", err)
				continue
			}
			for i, out := range outs {
				oid := oids[i]
				ncc.log.Debugf("received UTXO, outputID: %s", oid.ToHex())
				ncc.outputHandler(oid, out)
			}
		}
	}
	cancelContext()
}

func shouldBeProcessed(out iotago.Output) bool {
	// only outputs without SDRC should be processed.
	return !out.UnlockConditionSet().HasStorageDepositReturnCondition()
}

func (ncc *ncChain) subscribeToChainOwnedUTXOs() {
	init := true
	for {
		if init {
			init = false
		} else {
			ncc.log.Infof("Retrying output subscription for chainAddr=%v", ncc.chainID.String())
			time.Sleep(500 * time.Millisecond) // Delay between retries.
		}

		//
		// Subscribe to the new outputs first.
		eventsCh, subInfo := ncc.nc.mqttClient.OutputsByUnlockConditionAndAddress(
			ncc.chainID.AsAddress(),
			parameters.L1().Protocol.Bech32HRP,
			nodeclient.UnlockConditionAny,
		)
		if subInfo.Error() != nil {
			ncc.log.Panicf("failed to subscribe: %v", subInfo.Error())
		}
		//
		// Then fetch all the existing unspent outputs owned by the chain.
		ncc.queryChainUTXOs()

		//
		// Then receive all the subscribed new outputs.
		for {
			select {
			case outResponse := <-eventsCh:
				out, err := outResponse.Output()
				if err != nil {
					ncc.log.Warnf("error while receiving unspent output: %v", err)
					continue
				}
				if outResponse.Metadata == nil {
					ncc.log.Warnf("error while receiving unspent output, metadata is nil")
					continue
				}
				tid, err := outResponse.Metadata.TxID()
				if err != nil {
					ncc.log.Warnf("error while receiving unspent output tx id: %v", err)
					continue
				}
				outID := iotago.OutputIDFromTransactionIDAndIndex(*tid, outResponse.Metadata.OutputIndex)
				ncc.log.Debugf("received UTXO, outputID: %s", outID.ToHex())
				if shouldBeProcessed(out) {
					ncc.outputHandler(outID, out)
				}
			case <-ncc.nc.ctx.Done():
				return
			}
		}
	}
}

func (ncc *ncChain) queryLatestChainStateUTXO() {
	// TODO what should be an adequate timeout for this query?
	ctxWithTimeout, cancelContext := newCtx(ncc.nc.ctx)
	stateOutputID, stateOutput, err := ncc.nc.indexerClient.Alias(ctxWithTimeout, *ncc.chainID.AsAliasID())
	cancelContext()
	if err != nil {
		ncc.log.Panicf("error while fetching chain state output: %v", err)
	}
	ncc.log.Debugf("received chain state update, outputID: %s", stateOutputID.ToHex())
	ncc.stateOutputHandler(*stateOutputID, stateOutput)
}

func (ncc *ncChain) subscribeToChainStateUpdates() {
	//
	// Subscribe to the new outputs first.
	eventsCh, subInfo := ncc.nc.mqttClient.AliasOutputsByID(*ncc.chainID.AsAliasID())
	if subInfo.Error() != nil {
		ncc.log.Panicf("failed to subscribe: %v", subInfo.Error())
	}

	//
	// Then fetch the latest chain state UTXO.
	ncc.queryLatestChainStateUTXO()

	//
	// Then receive all the subscribed state outputs.
	for {
		select {
		case outResponse := <-eventsCh:
			out, err := outResponse.Output()
			if err != nil {
				ncc.log.Warnf("error while receiving chain state unspent output: %v", err)
				continue
			}
			if outResponse.Metadata == nil {
				ncc.log.Warnf("error while receiving chain state unspent output, metadata is nil")
				continue
			}
			tid, err := outResponse.Metadata.TxID()
			if err != nil {
				ncc.log.Warnf("error while receiving chain state unspent output tx id: %v", err)
				continue
			}
			outID := iotago.OutputIDFromTransactionIDAndIndex(*tid, outResponse.Metadata.OutputIndex)
			ncc.log.Debugf("received chain state update, outputID: %s", outID.ToHex())
			ncc.stateOutputHandler(outID, out)
		case <-ncc.nc.ctx.Done():
			return
		}
	}
}

func (ncc *ncChain) run() {
	go ncc.subscribeToChainStateUpdates()
	go ncc.subscribeToChainOwnedUTXOs()
}
