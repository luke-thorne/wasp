// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"bytes"
	"crypto/ed25519"
	"fmt"
	"sort"
	"time"

	"github.com/iotaledger/hive.go/identity"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/packages/chain"
	"github.com/iotaledger/wasp/packages/chain/consensus/journal"
	"github.com/iotaledger/wasp/packages/chain/messages"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/rotate"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/subrealm"
	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/peering"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"go.uber.org/zap"
)

// takeAction triggers actions whenever relevant
func (c *consensus) takeAction() {
	if !c.workflow.IsStateReceived() || !c.workflow.IsInProgress() {
		c.log.Debugf("takeAction skipped: stateReceived: %v, workflow in progress: %v",
			c.workflow.IsStateReceived(), c.workflow.IsInProgress())
		return
	}

	c.proposeBatchIfNeeded()
	c.runVMIfNeeded()
	c.checkQuorum()
	c.postTransactionIfNeeded()
	c.pullInclusionStateIfNeeded()
}

// proposeBatchIfNeeded when non empty ready batch is available is in mempool propose it as a candidate
// for the ACS agreement
func (c *consensus) proposeBatchIfNeeded() {
	if !c.workflow.IsIndexProposalReceived() {
		c.log.Debugf("proposeBatch not needed: dss nonce proposals are not ready yet")
		return
	}
	if c.workflow.IsBatchProposalSent() {
		c.log.Debugf("proposeBatch not needed: batch proposal already sent")
		return
	}
	if c.workflow.IsConsensusBatchKnown() {
		c.log.Debugf("proposeBatch not needed: consensus batch already known")
		return
	}
	if time.Now().Before(c.delayBatchProposalUntil) {
		c.log.Debugf("proposeBatch not needed: delayed till %v", c.delayBatchProposalUntil)
		return
	}
	if time.Now().Before(c.stateTimestamp.Add(c.timers.ProposeBatchDelayForNewState)) {
		c.log.Debugf("proposeBatch not needed: delayed for %v from %v", c.timers.ProposeBatchDelayForNewState, c.stateTimestamp)
		return
	}
	if c.timeData.IsZero() {
		c.log.Debugf("proposeBatch not needed: time data hasn't been received yet")
		return
	}
	reqs := c.mempool.ReadyNow(c.timeData)
	if len(reqs) == 0 {
		c.log.Debugf("proposeBatch not needed: no ready requests in mempool")
		return
	}
	c.log.Debugf("proposeBatch needed: ready requests len = %d, requests: %+v", len(reqs), isc.ShortRequestIDsFromRequests(reqs))
	proposal := c.prepareBatchProposal(reqs, c.dssIndexProposal)
	// call the ACS consensus. The call should spawn goroutine itself
	journalLogIndex := c.consensusJournalLogIndex
	acsSessionID := journalLogIndex.AsUint64Key(c.consensusJournal.GetID())
	c.acsSessionID = acsSessionID
	c.committee.RunACSConsensus(proposal.Bytes(), c.acsSessionID, c.stateOutput.GetStateIndex(), func(sessionID uint64, acs [][]byte) {
		c.log.Debugf("proposeBatch RunACSConsensus callback: responding to ACS session ID %v: len = %d", sessionID, len(acs))
		go c.EnqueueAsynchronousCommonSubsetMsg(&messages.AsynchronousCommonSubsetMsg{
			ProposedBatchesBin: acs,
			SessionID:          acsSessionID, // Use the local copy here.
			LogIndex:           journalLogIndex,
		})
	})

	c.log.Infof("proposeBatch: proposed batch len = %d, ACS session ID: %d, state index: %d, timestamp: %v",
		len(reqs), c.acsSessionID, c.stateOutput.GetStateIndex(), proposal.TimeData)
	c.workflow.setBatchProposalSent()
}

// runVMIfNeeded attempts to extract deterministic batch of requests from ACS.
// If it succeeds (i.e. all requests are available) and the extracted batch is nonempty, it runs the request
func (c *consensus) runVMIfNeeded() { // nolint:funlen
	if !c.workflow.IsConsensusBatchKnown() {
		c.log.Debugf("runVM not needed: consensus batch is not known")
		return
	}
	if c.workflow.IsVMStarted() || c.workflow.IsVMResultSigned() {
		c.log.Debugf("runVM not needed: vmStarted %v, vmResultSigned %v",
			c.workflow.IsVMStarted(), c.workflow.IsVMResultSigned())
		return
	}
	if time.Now().Before(c.delayRunVMUntil) {
		c.log.Debugf("runVM not needed: delayed till %v", c.delayRunVMUntil)
		return
	}

	reqs, missingRequestIndexes, allArrived := c.mempool.ReadyFromIDs(c.consensusBatch.TimeData, c.consensusBatch.RequestIDs...)
	c.log.Debugf("runVM: retrieved %v requests; allArrived=%v, missing request indexes: %v, retrieved requests: %+v",
		len(reqs), allArrived, missingRequestIndexes, isc.ShortRequestIDsFromRequests(reqs))

	c.cleanMissingRequests()

	if !allArrived {
		c.pollMissingRequests(missingRequestIndexes)
		return
	}
	if len(reqs) == 0 {
		// due to change in time, all requests became non processable ACS must be run again
		c.log.Debugf("runVM not needed: empty list of processable requests. Reset workflow")
		c.resetWorkflow()
		return
	}

	if err := c.consensusBatch.EnsureTimestampConsistent(reqs, c.stateTimestamp); err != nil {
		c.log.Errorf("Unable to ensure consistent timestamp: %v", err)
		c.resetWorkflow()
		return
	}

	c.log.Debugf("runVM needed: total number of requests = %d", len(reqs))
	// here reqs as a set is deterministic. Must be sorted to have fully deterministic list
	c.sortBatch(reqs)
	c.log.Debugf("runVM: sorted requests: %+v", isc.ShortRequestIDsFromRequests(reqs))

	vmTask := c.prepareVMTask(reqs)
	if vmTask == nil {
		c.log.Errorf("runVM: error preparing VM task")
		return
	}
	chainID := isc.ChainIDFromAliasID(vmTask.AnchorOutput.AliasID)
	c.log.Debugw("runVMIfNeeded: starting VM task",
		"chainID", (&chainID).String(),
		"ACS session ID", vmTask.ACSSessionID,
		"timestamp", vmTask.TimeAssumption,
		"timestamp (Unix nano)", vmTask.TimeAssumption.UnixNano(),
		"anchor output ID", isc.OID(vmTask.AnchorOutputID.UTXOInput()),
		"block index", vmTask.AnchorOutput.StateIndex,
		"entropy", vmTask.Entropy.String(),
		"validator fee target", vmTask.ValidatorFeeTarget.String(),
		"num req", len(vmTask.Requests),
		"estimate gas mode", vmTask.EstimateGasMode,
		"state commitment", state.RootCommitment(vmTask.VirtualStateAccess.TrieNodeStore()),
	)
	c.workflow.setVMStarted()
	c.consensusMetrics.CountVMRuns()
	go func() {
		err := c.vmRunner.Run(vmTask)
		if err != nil {
			c.log.Errorf("runVM result: VM task failed: %v", err)
			return
		}
		finalRequestsCount := len(vmTask.Results)
		if finalRequestsCount == 0 {
			c.log.Debugf("runVM result: no requests included, ignoring the result and restarting the workflow")
			c.resetWorkflow()
			return
		}
		// NOTE: this loop is needed for logging purposes only; it can be removed for optimisation if needed.
		finalRequests := make([]isc.Request, finalRequestsCount)
		for i := range vmTask.Results {
			finalRequests[i] = vmTask.Results[i].Request
		}
		c.log.Debugf("runVM result: responding by state index: %d, state commitment: %s, included %v requests: %v",
			vmTask.VirtualStateAccess.BlockIndex(), state.RootCommitment(vmTask.VirtualStateAccess.TrieNodeStore()), finalRequestsCount, isc.ShortRequestIDsFromRequests(finalRequests))
		c.EnqueueVMResultMsg(&messages.VMResultMsg{
			Task: vmTask,
		})
		elapsed := time.Since(c.workflow.GetVMStartedTime())
		c.consensusMetrics.RecordVMRunTime(elapsed)
	}()
}

func (c *consensus) pollMissingRequests(missingRequestIndexes []int) {
	// some requests are not ready, so skip VM call this time. Maybe next time will be more luck
	c.delayRunVMUntil = time.Now().Add(c.timers.VMRunRetryToWaitForReadyRequests)
	c.log.Infof( // Was silently failing when entire arrays were logged instead of counts.
		"runVM not needed: some requests didn't arrive yet. #BatchRequestIDs: %v | #BatchHashes: %v | #MissingIndexes: %v",
		len(c.consensusBatch.RequestIDs), len(c.consensusBatch.RequestHashes), len(missingRequestIndexes),
	)

	// send message to other committee nodes asking for the missing requests
	if !c.pullMissingRequestsFromCommittee {
		return
	}
	missingRequestIds := []isc.RequestID{}
	missingRequestIDsString := ""
	for _, idx := range missingRequestIndexes {
		reqID := c.consensusBatch.RequestIDs[idx]
		reqHash := c.consensusBatch.RequestHashes[idx]
		c.missingRequestsFromBatch[reqID] = reqHash
		missingRequestIds = append(missingRequestIds, reqID)
		missingRequestIDsString += reqID.String() + ", "
	}
	c.log.Debugf("runVMIfNeeded: asking for missing requests, ids: [%v]", missingRequestIDsString)
	msg := &messages.MissingRequestIDsMsg{IDs: missingRequestIds}
	c.committeePeerGroup.SendMsgBroadcast(peering.PeerMessageReceiverChain, chain.PeerMsgTypeMissingRequestIDs, msg.Bytes())
}

// sortBatch deterministically sorts batch based on the value extracted from the consensus entropy
// It is needed for determinism and as a MEV prevention measure see [prevent-mev.md]
func (c *consensus) sortBatch(reqs []isc.Request) {
	if len(reqs) <= 1 {
		return
	}
	rnd := util.MustUint32From4Bytes(c.consensusEntropy[:4])

	type sortStru struct {
		num uint32
		req isc.Request
	}
	toSort := make([]sortStru, len(reqs))
	for i, req := range reqs {
		toSort[i] = sortStru{
			num: (util.MustUint32From4Bytes(req.ID().Bytes()[:4]) + rnd) & 0x0000FFFF,
			req: req,
		}
	}
	sort.Slice(toSort, func(i, j int) bool {
		switch {
		case toSort[i].num < toSort[j].num:
			return true
		case toSort[i].num > toSort[j].num:
			return false
		default: // ==
			return bytes.Compare(toSort[i].req.ID().Bytes(), toSort[j].req.ID().Bytes()) < 0
		}
	})
	for i := range reqs {
		reqs[i] = toSort[i].req
	}
}

func getMaintenanceStatus(store kv.KVStore) bool {
	govstate := subrealm.New(store, kv.Key(governance.Contract.Hname().Bytes()))
	r := govstate.MustGet(governance.VarMaintenanceStatus)
	if r == nil {
		return false // chain is being initialized, governance has not been initialized yet
	}
	return codec.MustDecodeBool(r)
}

func (c *consensus) prepareVMTask(reqs []isc.Request) *vm.VMTask {
	stateBaseline := c.chain.GlobalStateSync().GetSolidIndexBaseline()
	if !stateBaseline.IsValid() {
		c.log.Debugf("prepareVMTask: solid state baseline is invalid. Do not even start the VM")
		return nil
	}
	task := &vm.VMTask{
		ACSSessionID:           c.acsSessionID,
		Processors:             c.chain.Processors(),
		AnchorOutput:           c.stateOutput.GetAliasOutput(),
		AnchorOutputID:         c.stateOutput.OutputID(),
		SolidStateBaseline:     stateBaseline,
		Entropy:                c.consensusEntropy,
		ValidatorFeeTarget:     c.consensusBatch.FeeDestination,
		Requests:               reqs,
		TimeAssumption:         c.consensusBatch.TimeData,
		VirtualStateAccess:     c.currentState.Copy(),
		Log:                    c.log.Desugar().WithOptions(zap.AddCallerSkip(1)).Sugar(),
		MaintenanceModeEnabled: getMaintenanceStatus(c.currentState.KVStore()),
	}
	c.log.Debugf("prepareVMTask: VM task prepared")
	return task
}

// checkQuorum when relevant check if quorum of signatures to the own calculated result is available
// If so, it aggregates signatures and finalizes the transaction.
// Then it deterministically calculates a priority sequence among contributing nodes for posting
// the transaction to L1. The deadline por posting is set proportionally to the sequence number (deterministic)
// If the node sees the transaction of the L1 before its deadline, it cancels its posting
func (c *consensus) checkQuorum() { //nolint:funlen
	if c.workflow.IsTransactionFinalized() {
		c.log.Debugf("checkQuorum not needed: transaction already finalized")
		return
	}
	if !c.workflow.IsVMResultSigned() {
		// only can aggregate signatures if own result is calculated
		c.log.Debugf("checkQuorum not needed: vm result is not signed")
		return
	}

	tx, chainOutput, err := c.finalizeTransaction()
	if err != nil {
		c.log.Errorf("checkQuorum finalizeTransaction fail: %v", err)
		return
	}

	c.finalTx = tx

	if c.resultState != nil { // if it is not governance transaction (state controller rotation)
		// write block to WAL
		chainOutputID := chainOutput.ID()
		block, err := c.resultState.ExtractBlock()
		if err == nil {
			block.SetApprovingOutputID(chainOutputID)
			err = c.wal.Write(block.Bytes())
			if err == nil {
				c.log.Debugf("checkQuorum: block index %v written to wal", block.BlockIndex())
			} else {
				c.log.Warnf("checkQuorum: error writing block to wal: %v", err)
			}
		} else {
			c.log.Warnf("checkQuorum: skipping writing block to wal: error extracting block from state: %v", err)
		}

		// sending message to state manager
		// if it is state controller rotation, state manager is not notified
		c.workflow.setCurrentStateIndex(c.resultState.BlockIndex())
		c.chain.StateCandidateToStateManager(c.resultState, chainOutputID)
		c.log.Debugf("checkQuorum: StateCandidateMsg sent for state index %v, approving output ID %v, timestamp %v",
			c.resultState.BlockIndex(), isc.OID(chainOutputID), c.resultState.Timestamp())
	}

	// calculate deterministic and pseudo-random order and postTxDeadline among contributors
	var postSeqNumber uint16
	var permutation *util.Permutation16
	txID, err := tx.ID()
	if err != nil {
		c.log.Errorf("checkQuorum failed: cannot calculate transaction ID: %v", err)
		return
	}
	if c.iAmContributor {
		seed := int64(0)
		for i := range txID {
			seed = ((seed << 8) | (seed >> 56 & 0x0FF)) ^ int64(txID[i])
		}
		permutation, err = util.NewPermutation16(uint16(len(c.contributors)), seed)
		if err != nil {
			c.log.Panicf("This should not happen as the seed is provided: %v", err)
		}
		postSeqNumber = permutation.GetArray()[c.myContributionSeqNumber]
		c.postTxDeadline = time.Now().Add(time.Duration(postSeqNumber) * c.timers.PostTxSequenceStep)

		c.log.Debugf("checkQuorum: finalized tx %s, iAmContributor: true, postSeqNum: %d (time: %v), permutation: %+v",
			isc.TxID(txID), postSeqNumber, c.postTxDeadline, permutation.GetArray())
	} else {
		c.log.Debugf("checkQuorum: finalized tx %s, iAmContributor: false", isc.TxID(txID))
	}
	c.workflow.setTransactionFinalized()
	c.pullInclusionStateDeadline = time.Now()
}

// postTransactionIfNeeded posts a finalized transaction upon deadline unless it was evidenced on L1 before the deadline.
func (c *consensus) postTransactionIfNeeded() {
	if !c.workflow.IsTransactionFinalized() {
		c.log.Debugf("postTransaction not needed: transaction is not finalized")
		return
	}
	if !c.iAmContributor {
		// only contributors post transaction
		c.log.Debugf("postTransaction not needed: i am not a contributor")
		return
	}
	if c.workflow.IsTransactionPosted() {
		c.log.Debugf("postTransaction not needed: transaction already posted")
		return
	}
	if c.workflow.IsTransactionSeen() {
		c.log.Debugf("postTransaction not needed: transaction already seen")
		return
	}
	if time.Now().Before(c.postTxDeadline) {
		c.log.Debugf("postTransaction not needed: delayed till %v", c.postTxDeadline)
		return
	}
	var logMsgTypeStr string
	var logMsgStateIndexStr string
	if c.resultState == nil { // governance transaction
		if err := c.nodeConn.PublishGovernanceTransaction(c.finalTx); err != nil {
			c.log.Errorf("postTransaction: error publishing gov transaction: %w", err)
			return
		}
		logMsgTypeStr = "GOVERNANCE"
		logMsgStateIndexStr = ""
	} else {
		stateIndex := c.resultState.BlockIndex()
		if err := c.nodeConn.PublishStateTransaction(stateIndex, c.finalTx); err != nil {
			c.log.Errorf("postTransaction: error publishing state transaction: %v", err)
			return
		}
		logMsgTypeStr = "STATE"
		logMsgStateIndexStr = fmt.Sprintf(" for state %v", stateIndex)
	}

	c.workflow.setTransactionPosted() // TODO: Fix it, retries should be in place for robustness.
	logMsgStart := fmt.Sprintf("postTransaction: POSTED %s TRANSACTION%s:", logMsgTypeStr, logMsgStateIndexStr)
	logMsgEnd := fmt.Sprintf("number of inputs: %d, outputs: %d", len(c.finalTx.Essence.Inputs), len(c.finalTx.Essence.Outputs))
	txID, err := c.finalTx.ID()
	if err == nil {
		c.log.Infof("%s %s, %s", logMsgStart, isc.TxID(txID), logMsgEnd)
	} else {
		c.log.Warnf("%s %s", logMsgStart, logMsgEnd)
	}
}

// pullInclusionStateIfNeeded periodic pull to know the inclusions state of the transaction. Note that pulling
// starts immediately after finalization of the transaction, not after posting it
func (c *consensus) pullInclusionStateIfNeeded() {
	if !c.workflow.IsTransactionFinalized() {
		c.log.Debugf("pullInclusionState not needed: transaction is not finalized")
		return
	}
	if c.workflow.IsTransactionSeen() {
		c.log.Debugf("pullInclusionState not needed: transaction already seen")
		return
	}
	if time.Now().Before(c.pullInclusionStateDeadline) {
		c.log.Debugf("pullInclusionState not needed: delayed till %v", c.pullInclusionStateDeadline)
		return
	}
	finalTxID, err := c.finalTx.ID()
	if err != nil {
		c.log.Panicf("pullInclusionState: cannot calculate final transaction id: %v", err)
	}
	c.nodeConn.PullTxInclusionState(finalTxID)
	c.pullInclusionStateDeadline = time.Now().Add(c.timers.PullInclusionStateRetry)
	c.log.Debugf("pullInclusionState: request for inclusion state sent")
}

// prepareBatchProposal creates a batch proposal structure out of requests
func (c *consensus) prepareBatchProposal(reqs []isc.Request, dssNonceIndexProposal []int) *BatchProposal {
	consensusManaPledge := identity.ID{}
	accessManaPledge := identity.ID{}
	feeDestination := isc.NewContractAgentID(c.chain.ID(), 0)
	// sign state output ID. It will be used to produce unpredictable entropy in consensus
	outputID := c.stateOutput.OutputID()
	sigShare, err := c.committee.DKShare().BLSSignShare(outputID[:])
	c.assert.RequireNoError(err, fmt.Sprintf("prepareBatchProposal: signing output ID %v failed", isc.OID(c.stateOutput.ID())))

	timestamp := c.timeData
	if timestamp.Before(c.currentState.Timestamp()) {
		timestamp = c.currentState.Timestamp().Add(time.Nanosecond)
	}

	ret := &BatchProposal{
		ValidatorIndex:          c.committee.OwnPeerIndex(),
		StateOutputID:           c.stateOutput.ID(),
		RequestIDs:              make([]isc.RequestID, len(reqs)),
		RequestHashes:           make([][32]byte, len(reqs)),
		TimeData:                timestamp,
		ConsensusManaPledge:     consensusManaPledge,
		AccessManaPledge:        accessManaPledge,
		FeeDestination:          feeDestination,
		SigShareOfStateOutputID: sigShare,
		DSSNonceIndexProposal:   util.NewFixedSizeBitVector(int(c.committee.Size())).SetBits(dssNonceIndexProposal),
	}
	for i, req := range reqs {
		ret.RequestIDs[i] = req.ID()
		ret.RequestHashes[i] = isc.RequestHash(req)
	}

	c.log.Debugf("prepareBatchProposal: proposal prepared")
	return ret
}

// receiveACS processed new ACS received from ACS consensus
//
//nolint:funlen
func (c *consensus) receiveACS(values [][]byte, sessionID uint64, logIndex journal.LogIndex) {
	if c.acsSessionID != sessionID {
		c.log.Debugf("receiveACS: session id mismatch: expected %v, received %v", c.acsSessionID, sessionID)
		c.resetWorkflow() // TODO: That's temporary solution.
		return
	}
	if c.workflow.IsConsensusBatchKnown() {
		// should not happen
		c.log.Debugf("receiveACS: consensus batch already known (should not happen)")
		return
	}
	if len(values) < int(c.committee.Quorum()) {
		// should not happen. Something wrong with the ACS layer
		c.log.Errorf("receiveACS: ACS is shorter (len=%v) than required quorum (%v). Ignored", len(values), c.committee.Quorum())
		c.resetWorkflow()
		return
	}
	c.consensusJournal.ConsensusReached(logIndex)
	if c.markedForReset {
		c.log.Debugf("receiveACS: ignoring ACS result and resetting workflow as the consensus was marked for reset")
		c.resetWorkflowNoCheck()
		return
	}
	// decode ACS
	acs := make([]*BatchProposal, len(values))
	for i, data := range values {
		proposal, err := BatchProposalFromBytes(data)
		if err != nil {
			c.log.Errorf("receiveACS: wrong data received. Whole ACS ignored: %v", err)
			c.resetWorkflow()
			return
		}
		acs[i] = proposal
	}
	contributors := make([]uint16, 0, c.committee.Size())
	contributorSet := make(map[uint16]struct{})
	// validate ACS. Dismiss ACS if inconsistent. Should not happen
	for _, prop := range acs {
		if !prop.StateOutputID.Equals(c.stateOutput.ID()) {
			c.log.Warnf("receiveACS: ACS out of context or consensus failure: expected stateOuptudId: %v, generated stateOutputID: %v ",
				isc.OID(c.stateOutput.ID()), isc.OID(prop.StateOutputID))
			c.resetWorkflow()
			return
		}
		if prop.ValidatorIndex >= c.committee.Size() {
			c.log.Warnf("receiveACS: wrong validator index in ACS: committee size is %v, validator index is %v",
				c.committee.Size(), prop.ValidatorIndex)
			c.resetWorkflow()
			return
		}
		contributors = append(contributors, prop.ValidatorIndex)
		if _, already := contributorSet[prop.ValidatorIndex]; already {
			c.log.Errorf("receiveACS: duplicate contributor %v in ACS", prop.ValidatorIndex)
			c.resetWorkflow()
			return
		}
		c.log.Debugf("receiveACS: contributor %v of ACS included", prop.ValidatorIndex)
		contributorSet[prop.ValidatorIndex] = struct{}{}
	}

	// sort contributors for determinism because ACS returns sets ordered randomly
	sort.Slice(contributors, func(i, j int) bool {
		return contributors[i] < contributors[j]
	})
	iAmContributor := false
	myContributionSeqNumber := uint16(0)
	for i, contr := range contributors {
		if contr == c.committee.OwnPeerIndex() {
			iAmContributor = true
			myContributionSeqNumber = uint16(i)
		}
	}

	// calculate intersection of proposals
	inBatchIDs, inBatchHashes := calcIntersection(acs, c.committee.Size())
	if len(inBatchIDs) == 0 {
		// if intersection is empty, reset workflow and retry after some time. It means not all requests
		// reached nodes and we have give it a time. Should not happen often
		c.log.Warnf("receiveACS: ACS intersection (light) is empty. reset workflow. State index: %d, ACS sessionID %d",
			c.stateOutput.GetStateIndex(), sessionID)
		c.resetWorkflow()
		c.delayBatchProposalUntil = time.Now().Add(c.timers.ProposeBatchRetry)
		return
	}
	//
	// Collect the decided nonce proposals.
	dssIndexProposalsDecided := make([][]int, c.committee.Size())
	for i := range acs {
		dssIndexProposalsDecided[acs[i].ValidatorIndex] = acs[i].DSSNonceIndexProposal.AsInts()
	}
	c.dssIndexProposalsDecided = dssIndexProposalsDecided
	//
	// calculate other batch parameters in a deterministic way
	par, err := c.calcBatchParameters(acs)
	if err != nil {
		// should not happen, unless insider attack
		c.log.Errorf("receiveACS: inconsistent ACS. Reset workflow. State index: %d, ACS sessionID %d, reason: %v",
			c.stateOutput.GetStateIndex(), sessionID, err)
		c.resetWorkflow()
		c.delayBatchProposalUntil = time.Now().Add(c.timers.ProposeBatchRetry)
	}
	c.consensusBatch = &BatchProposal{
		ValidatorIndex:        c.committee.OwnPeerIndex(),
		StateOutputID:         c.stateOutput.ID(),
		RequestIDs:            inBatchIDs,
		RequestHashes:         inBatchHashes,
		TimeData:              par.timeData,
		ConsensusManaPledge:   par.consensusPledge,
		AccessManaPledge:      par.accessPledge,
		FeeDestination:        par.feeDestination,
		DSSNonceIndexProposal: nil, // Not needed in the final batch proposal.
	}
	c.consensusEntropy = par.entropy

	c.iAmContributor = iAmContributor
	c.myContributionSeqNumber = myContributionSeqNumber
	c.contributors = contributors

	c.workflow.setConsensusBatchKnown()

	if c.iAmContributor {
		c.log.Debugf("receiveACS: ACS received. Contributors to ACS: %+v, iAmContributor: true, seqnr: %d, %v reqs: %+v, timestamp: %v",
			c.contributors, c.myContributionSeqNumber, len(c.consensusBatch.RequestIDs), isc.ShortRequestIDs(c.consensusBatch.RequestIDs), c.consensusBatch.TimeData)
	} else {
		c.log.Debugf("receiveACS: ACS received. Contributors to ACS: %+v, iAmContributor: false, %v reqs: %+v, timestamp: %v",
			c.contributors, len(c.consensusBatch.RequestIDs), isc.ShortRequestIDs(c.consensusBatch.RequestIDs), c.consensusBatch.TimeData)
	}

	c.runVMIfNeeded()
}

func (c *consensus) processTxInclusionState(msg *messages.TxInclusionStateMsg) {
	if !c.workflow.IsTransactionFinalized() {
		c.log.Debugf("processTxInclusionState: transaction not finalized -> skipping.")
		return
	}
	finalTxID, err := c.finalTx.ID()
	finalTxIDStr := isc.TxID(finalTxID)
	if err != nil {
		c.log.Panicf("processTxInclusionState: cannot calculate final transaction id: %v", err)
	}
	if msg.TxID != finalTxID {
		c.log.Debugf("processTxInclusionState: current transaction id %v does not match the received one %v -> skipping.",
			finalTxIDStr, isc.TxID(msg.TxID))
		return
	}
	switch msg.State {
	case "noTransaction":
		c.log.Debugf("processTxInclusionState: transaction id %v is not known.", finalTxIDStr)
	case "included":
		c.workflow.setTransactionSeen()
		c.workflow.setCompleted()
		c.refreshConsensusInfo()
		c.log.Debugf("processTxInclusionState: transaction id %s is included; workflow finished", finalTxIDStr)
	case "conflicting":
		c.workflow.setTransactionSeen()
		c.log.Infof("processTxInclusionState: transaction id %s is conflicting; restarting consensus.", finalTxIDStr)
		c.resetWorkflow()
	default:
		c.log.Warnf("processTxInclusionState: unknown inclusion state %s for transaction id %s; ignoring", msg.State, finalTxIDStr)
	}
}

func (c *consensus) finalizeTransaction() (*iotago.Transaction, *isc.AliasOutputWithID, error) {
	if c.dssSignature == nil {
		return nil, nil, fmt.Errorf("DSS signature not ready yet")
	}
	signature := c.dssSignature

	// check consistency ---------------- check if chain inputs were consumed
	chainInput := c.stateOutput.ID()
	indexChainInput := -1
	for i, inp := range c.resultTxEssence.Inputs {
		if inp.Type() == iotago.InputUTXO {
			if inp.(*iotago.UTXOInput).Equals(chainInput) {
				indexChainInput = i
				break
			}
		}
	}
	c.assert.Requiref(
		indexChainInput >= 0,
		fmt.Sprintf("finalizeTransaction: cannot find tx input for state output %v. major inconsistency", isc.OID(c.stateOutput.ID())),
	)
	// check consistency ---------------- end

	publicKey := c.committee.DKShare().GetSharedPublic()
	var signatureArray [ed25519.SignatureSize]byte
	copy(signatureArray[:], signature)
	signatureForUnlock := &iotago.Ed25519Signature{
		PublicKey: publicKey.AsKey(),
		Signature: signatureArray,
	}
	tx := &iotago.Transaction{
		Essence: c.resultTxEssence,
		Unlocks: transaction.MakeSignatureAndAliasUnlockFeatures(len(c.resultTxEssence.Inputs), signatureForUnlock),
	}
	chained, err := transaction.GetAliasOutput(tx, c.chain.ID().AsAddress())
	if err != nil {
		return nil, nil, err
	}
	txID, err := tx.ID()
	if err != nil {
		return nil, nil, err
	}
	c.log.Debugf("finalizeTransaction: transaction %v finalized; approving output ID: %v", isc.TxID(txID), isc.OID(chained.ID()))
	return tx, chained, nil
}

func (c *consensus) setNewState(msg *messages.StateTransitionMsg) bool {
	c.consensusJournal.GetLocalView().AliasOutputReceived(msg.StateOutput)
	sameIndex := msg.State.BlockIndex() == msg.StateOutput.GetStateIndex()
	if !msg.IsGovernance && !sameIndex {
		// NOTE: should be a panic. However this situation may occur (and occurs) in normal circumstations:
		// 1) State manager synchronizes to state index n and passes state transmission message through event to consensus asynchronously
		// 2) Consensus is overwhelmed and receives a message after delay
		// 3) Meanwhile state manager is quick enough to synchronize to state index n+1 and commits a block of state index n+1
		// 4) Only then the consensus receives a message sent in step 1. Due to imperfect implementation of virtual state copying it thinks
		//    that state is at index n+1, however chain output is (as was transmitted) and at index n.
		// The virtual state copying (earlier called "cloning") works in a following way: it copies all the mutations, stored in buffered KVS,
		// however it obtains the same kvs object to access the database. BlockIndex method of virtual state checks if there are mutations editing
		// the index value. If so, it returns the newest value in respect to mutations. Otherwise it checks the database for newest index value.
		// In the described scenario, there are no mutations neither in step 1, nor in step 3, because just before completing state synchronization
		// all the mutations are written to the DB. However, reading the same DB in step 1 results in index n and in step 4 (after the commit of block
		// index n+1) -- in index n+1. Thus effectively the virtual state received is different than the virtual state sent.
		c.log.Errorf("consensus::setNewState: state index is inconsistent: block: #%d != chain output: #%d",
			msg.State.BlockIndex(), msg.StateOutput.GetStateIndex())
		return false
	}

	// If c.stateOutput.GetStateIndex() == msg.StateOutput.GetStateIndex() and the new state output is not a governance update, then either,
	// a) c.stateOutput is the same as msg.StateOutput and there is no need to reassign c.stateOutput or b) msg.StateOutput is a regular state update
	// output and c.stateOutput is a governance update output with the same index; in such case governance update is the last and should be taken
	// into account. Regular state output is overwritten by governance update output and should be ignored.
	// TODO: it is assumed, that at most one governance update transaction may occur in between regular state update transactions. The situation of
	// several consecutive governance update transactions are yet to be discussed, designed and implemented. The main problem is that there is no way
	// in knowing the exact order of governance updates, which have the same block index.
	// I.e., this situation is undefined:
	// ... -> Transaction to state index 15 -> Govenance update at state index 15 -> Another governance update at state index 15 -> Transaction to state index 16 -> ...
	// however, this situation should be handled normally:
	// ... -> Transaction to state index 15 -> Govenance update at state index 15 -> Transaction to state index 16 -> Governance update at state index 16 -> ...
	if (c.stateOutput == nil) || (c.stateOutput.GetStateIndex() < msg.StateOutput.GetStateIndex()) || msg.IsGovernance {
		c.stateOutput = msg.StateOutput
	} else {
		c.log.Debugf("consensus::setNewState: ignoring the received state output %s in favor of the current one %s", isc.OID(msg.StateOutput.ID()), isc.OID(c.stateOutput.ID()))
		return false
	}
	c.stateTimestamp = msg.StateTimestamp
	oid := c.stateOutput.OutputID()
	c.acsSessionID = util.MustUint64From8Bytes(hashing.HashData(oid[:]).Bytes()[:8])
	if msg.IsGovernance && !sameIndex {
		c.currentState = nil
		c.log.Debugf("SET NEW STATE #%d (rotate) and pausing consensus to wait for adequate state, output: %s",
			c.stateOutput.GetStateIndex(), isc.OID(c.stateOutput.ID()))
	} else {
		c.currentState = msg.State
		r := ""
		if msg.IsGovernance {
			r = " (rotate) "
		}
		c.log.Debugf("SET NEW STATE #%d%s, output: %s, state commitment: %s",
			c.stateOutput.GetStateIndex(), r, isc.OID(c.stateOutput.ID()), state.RootCommitment(c.currentState.TrieNodeStore()))
	}
	c.resetWorkflow()
	return true
}

// TODO: KP: All that workflow reset will stop working with the ConsensusJournal introduced, because nodes
// have to agree on the reset. I.e. consensus has to complete, then its results can be ignored. Is that OK?
func (c *consensus) resetWorkflow() {
	if c.workflow.IsStateReceived() && !c.workflow.IsConsensusBatchKnown() {
		c.markedForReset = true
		c.log.Debugf("resetWorkflow: consensus marked for reset; it will be done once ACS is finished")
		return
	}
	c.resetWorkflowNoCheck()
}

func (c *consensus) resetWorkflowNoCheck() {
	c.consensusJournalLogIndex = c.consensusJournal.GetLogIndex() // Should be the next one.
	dssKey := c.consensusJournalLogIndex.AsStringKey(c.consensusJournal.GetID())
	c.log.Debugf("resetWorkflow: LogIndex=%v, Starting DSS session with key %v", c.consensusJournalLogIndex.AsUint32(), dssKey)
	err := c.dssNode.Start(dssKey, 0, c.committee.DKShare(),
		func(indexProposal []int) {
			c.log.Debugf("resetWorkflow DSS proposal callback: index proposal of %s is %v", dssKey, indexProposal)
			c.EnqueueDssIndexProposalMsg(&messages.DssIndexProposalMsg{
				DssKey:        dssKey,
				IndexProposal: indexProposal,
			})
		},
		func(signature []byte) {
			c.log.Debugf("resetWorkflow DSS signature callback: signature of %s is %v", dssKey, signature)
			c.EnqueueDssSignatureMsg(&messages.DssSignatureMsg{
				DssKey:    dssKey,
				Signature: signature,
			})
		},
	)
	if err != nil {
		c.log.Errorf("resetWorkflow: failed to start the DSS session: %v", err) // TODO: XXX: Handle it better.
	}

	c.acsSessionID++
	c.resultState = nil
	c.resultTxEssence = nil
	c.finalTx = nil
	c.consensusBatch = nil
	c.contributors = nil
	c.workflow = newWorkflowStatus(c.stateOutput != nil && c.currentState != nil, c.workflow.stateIndex)
	c.dssIndexProposal = nil
	c.dssIndexProposalsDecided = nil
	c.dssSignature = nil
	c.markedForReset = false
	c.log.Debugf("resetWorkflow completed; DSS session with key %s started", dssKey)
}

func (c *consensus) processVMResult(result *vm.VMTask) {
	if !c.workflow.IsVMStarted() ||
		c.workflow.IsDssSigningStarted() ||
		c.acsSessionID != result.ACSSessionID {
		// out of context
		c.log.Debugf("processVMResult: out of context vmStarted %v, dssSigningStarted %v, expected ACS session ID %v, returned ACS session ID %v",
			c.workflow.IsVMStarted(), c.workflow.IsDssSigningStarted(), c.acsSessionID, result.ACSSessionID)
		return
	}
	rotation := result.RotationAddress != nil
	if rotation {
		// if VM returned rotation, we ignore the updated virtual state and produce governance state controller
		// rotation transaction. It does not change state
		c.resultTxEssence = c.makeRotateStateControllerTransaction(result)
		c.resultState = nil
	} else {
		// It is and ordinary state transition
		c.assert.Requiref(result.ResultTransactionEssence != nil, "processVMResult: result.ResultTransactionEssence != nil")
		c.resultTxEssence = result.ResultTransactionEssence
		c.resultState = result.VirtualStateAccess
	}

	signingMsg, err := c.resultTxEssence.SigningMessage()
	if err != nil {
		c.log.Errorf("processVMResult: cannot obtain signing message: %v", err)
		return
	}
	signingMsgHash := hashing.HashData(signingMsg)
	dssKey := c.consensusJournalLogIndex.AsStringKey(c.consensusJournal.GetID())
	c.log.Debugf("processVMResult: starting DSS signing with key %s for message: %s, rotate state controller: %v", dssKey, signingMsgHash, rotation)
	err = c.dssNode.DecidedIndexProposals(dssKey, 0, c.dssIndexProposalsDecided, signingMsg)
	c.assert.RequireNoError(err, "processVMResult: starting DSS signing failed")

	c.workflow.setDssSigningStarted()

	c.log.Debugf("processVMResult: DSS signing with key %s started for message %s", dssKey, signingMsgHash.String())
}

func (c *consensus) makeRotateStateControllerTransaction(task *vm.VMTask) *iotago.TransactionEssence {
	c.log.Debugf("makeRotateStateControllerTransaction: %s", task.RotationAddress.Bech32(parameters.L1().Protocol.Bech32HRP))

	// TODO access and consensus pledge
	essence, err := rotate.MakeRotateStateControllerTransaction(
		task.RotationAddress,
		isc.NewAliasOutputWithID(task.AnchorOutput, task.AnchorOutputID.UTXOInput()),
		task.TimeAssumption,
		identity.ID{},
		identity.ID{},
	)
	c.assert.RequireNoError(err, "makeRotateStateControllerTransaction: ")
	return essence
}

func (c *consensus) receiveDssIndexProposal(dssKey string, indexProposal []int) {
	if c.workflow.IsIndexProposalReceived() {
		c.log.Debugf("receiveDssIndexProposal: proposal already received, ignoring")
		return
	}
	if c.consensusJournalLogIndex.AsStringKey(c.consensusJournal.GetID()) != dssKey {
		c.log.Debugf("receiveDssIndexProposal: proposal for %s received but for %s expected, ignoring", dssKey, c.consensusJournalLogIndex.AsStringKey(c.consensusJournal.GetID()))
		return
	}
	c.dssIndexProposal = indexProposal
	c.workflow.setIndexProposalReceived()
	c.log.Debugf("receiveDssIndexProposal: proposal for %s handled", dssKey)
	c.takeAction()
}

func (c *consensus) receiveDssSignature(dssKey string, signature []byte) {
	if !c.workflow.IsDssSigningStarted() {
		c.log.Debugf("receiveDssSignature: signature of key %s received but DSS signing is not yet started; ignoring", dssKey)
		return
	}
	if c.workflow.IsVMResultSigned() {
		c.log.Debugf("receiveDssSignature: signature of key %s received but VM result is already signed; ignoring", dssKey)
		return
	}
	if dssKey != c.consensusJournalLogIndex.AsStringKey(c.consensusJournal.GetID()) {
		c.log.Debugf("receiveDssSignature: signature of key %s received but signature of key %s is expected; ignoring", dssKey, c.consensusJournalLogIndex.AsStringKey(c.consensusJournal.GetID()))
		return
	}
	c.dssSignature = signature
	c.workflow.setVMResultSigned()
	c.log.Debugf("receiveDssSignature: Signature of key %s handled", dssKey)
	c.takeAction()
}

// TODO mutex inside is not good

// ShouldReceiveMissingRequest returns whether the request is missing, if the incoming request matches the expects ID/Hash it is removed from the list
func (c *consensus) ShouldReceiveMissingRequest(req isc.Request) bool {
	reqHash := hashing.HashData(req.Bytes())
	c.log.Debugf("ShouldReceiveMissingRequest: reqID %s, hash %v", req.ID(), reqHash)

	c.missingRequestsMutex.Lock()
	defer c.missingRequestsMutex.Unlock()

	expectedHash, exists := c.missingRequestsFromBatch[req.ID()]
	if !exists {
		return false
	}
	result := bytes.Equal(expectedHash[:], reqHash[:])
	if result {
		delete(c.missingRequestsFromBatch, req.ID())
	}
	return result
}

func (c *consensus) cleanMissingRequests() {
	c.missingRequestsMutex.Lock()
	defer c.missingRequestsMutex.Unlock()

	c.missingRequestsFromBatch = make(map[isc.RequestID][32]byte) // reset list of missing requests
}
