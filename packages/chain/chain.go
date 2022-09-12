// SPDX-License-Identifier: Apache-2.0

package chain

import (
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/logger"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/nodeclient"
	"github.com/iotaledger/trie.go/trie"
	"github.com/iotaledger/wasp/packages/chain/mempool"
	"github.com/iotaledger/wasp/packages/chain/messages"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/coreutil"
	"github.com/iotaledger/wasp/packages/metrics/nodeconnmetrics"
	"github.com/iotaledger/wasp/packages/peering"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/tcrypto"
	"github.com/iotaledger/wasp/packages/util/ready"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/processors"
)

type ChainCore interface {
	ID() *isc.ChainID
	GetCommitteeInfo() *CommitteeInfo
	StateCandidateToStateManager(state.VirtualStateAccess, *iotago.UTXOInput)
	TriggerChainTransition(*ChainTransitionEventData)
	Processors() *processors.Cache
	GlobalStateSync() coreutil.ChainStateSync
	GetStateReader() state.OptimisticStateReader
	GetChainNodes() []peering.PeerStatusProvider     // CommitteeNodes + AccessNodes
	GetCandidateNodes() []*governance.AccessNodeInfo // All the current candidates.
	Log() *logger.Logger
	EnqueueDismissChain(reason string)
	EnqueueAliasOutput(*isc.AliasOutputWithID)
}

// ChainEntry interface to access chain from the chain registry side
type ChainEntry interface {
	Dismiss(reason string)
	IsDismissed() bool
}

// ChainRequests is an interface to query status of the request
type ChainRequests interface {
	GetRequestReceipt(id isc.RequestID) (*blocklog.RequestReceipt, error)
	ResolveError(e *isc.UnresolvedVMError) (*isc.VMError, error)
	AttachToRequestProcessed(func(isc.RequestID)) (attachID *events.Closure)
	DetachFromRequestProcessed(attachID *events.Closure)
	EnqueueOffLedgerRequestMsg(msg *messages.OffLedgerRequestMsgIn)
}

type ChainMetrics interface {
	GetNodeConnectionMetrics() nodeconnmetrics.NodeConnectionMessagesMetrics
	GetConsensusWorkflowStatus() ConsensusWorkflowStatus
	GetConsensusPipeMetrics() ConsensusPipeMetrics
}

type ChainRunner interface {
	GetAnchorOutput() *isc.AliasOutputWithID
	GetTimeData() time.Time
	GetDB() kvstore.KVStore
}

type Chain interface {
	ChainCore
	ChainRequests
	ChainEntry
	ChainMetrics
	ChainRunner
}

// Committee is ordered (indexed 0..size-1) list of peers which run the consensus
type Committee interface {
	Address() iotago.Address
	Size() uint16
	Quorum() uint16
	OwnPeerIndex() uint16
	DKShare() tcrypto.DKShare
	IsAlivePeer(peerIndex uint16) bool
	QuorumIsAlive(quorum ...uint16) bool
	PeerStatus() []*PeerStatus
	IsReady() bool
	Close()
	RunACSConsensus(value []byte, sessionID uint64, stateIndex uint32, callback func(sessionID uint64, acs [][]byte))
	GetRandomValidators(upToN int) []*cryptolib.PublicKey // TODO: Remove after OffLedgerRequest dissemination is changed.
}

type (
	NodeConnectionAliasOutputHandlerFun     func(*isc.AliasOutputWithID)
	NodeConnectionOnLedgerRequestHandlerFun func(isc.OnLedgerRequest)
	NodeConnectionInclusionStateHandlerFun  func(iotago.TransactionID, string)
	NodeConnectionMilestonesHandlerFun      func(*nodeclient.MilestoneInfo)
)

type NodeConnection interface {
	RegisterChain(chainID *isc.ChainID, stateOutputHandler, outputHandler func(iotago.OutputID, iotago.Output))
	UnregisterChain(chainID *isc.ChainID)

	PublishStateTransaction(chainID *isc.ChainID, stateIndex uint32, tx *iotago.Transaction) error
	PublishGovernanceTransaction(chainID *isc.ChainID, tx *iotago.Transaction) error
	PullLatestOutput(chainID *isc.ChainID)
	PullTxInclusionState(chainID *isc.ChainID, txid iotago.TransactionID)
	PullStateOutputByID(chainID *isc.ChainID, id *iotago.UTXOInput)

	AttachTxInclusionStateEvents(chainID *isc.ChainID, handler NodeConnectionInclusionStateHandlerFun) (*events.Closure, error)
	DetachTxInclusionStateEvents(chainID *isc.ChainID, closure *events.Closure) error
	AttachMilestones(handler NodeConnectionMilestonesHandlerFun) *events.Closure
	DetachMilestones(attachID *events.Closure)

	SetMetrics(metrics nodeconnmetrics.NodeConnectionMetrics)
	GetMetrics() nodeconnmetrics.NodeConnectionMetrics
	Close()
}

type ChainNodeConnection interface {
	AttachToAliasOutput(NodeConnectionAliasOutputHandlerFun)
	DetachFromAliasOutput()
	AttachToOnLedgerRequest(NodeConnectionOnLedgerRequestHandlerFun)
	DetachFromOnLedgerRequest()
	AttachToTxInclusionState(NodeConnectionInclusionStateHandlerFun)
	DetachFromTxInclusionState()
	AttachToMilestones(NodeConnectionMilestonesHandlerFun)
	DetachFromMilestones()
	Close()

	PublishStateTransaction(stateIndex uint32, tx *iotago.Transaction) error
	PublishGovernanceTransaction(tx *iotago.Transaction) error
	PullLatestOutput()
	PullTxInclusionState(txid iotago.TransactionID)
	PullStateOutputByID(*iotago.UTXOInput)

	GetMetrics() nodeconnmetrics.NodeConnectionMessagesMetrics
}

type StateManager interface {
	Ready() *ready.Ready
	EnqueueGetBlockMsg(msg *messages.GetBlockMsgIn)
	EnqueueBlockMsg(msg *messages.BlockMsgIn)
	EnqueueAliasOutput(*isc.AliasOutputWithID)
	EnqueueStateCandidateMsg(state.VirtualStateAccess, *iotago.UTXOInput)
	EnqueueTimerMsg(msg messages.TimerTick)
	GetStatusSnapshot() *SyncInfo
	SetChainPeers(peers []*cryptolib.PublicKey)
	Close()
}

type Consensus interface {
	EnqueueStateTransitionMsg(bool, state.VirtualStateAccess, *isc.AliasOutputWithID, time.Time)
	EnqueueDssIndexProposalMsg(msg *messages.DssIndexProposalMsg)
	EnqueueDssSignatureMsg(msg *messages.DssSignatureMsg)
	EnqueueTxInclusionsStateMsg(iotago.TransactionID, string)
	EnqueueAsynchronousCommonSubsetMsg(msg *messages.AsynchronousCommonSubsetMsg)
	EnqueueVMResultMsg(msg *messages.VMResultMsg)
	EnqueueTimerMsg(messages.TimerTick)
	IsReady() bool
	Close()
	GetStatusSnapshot() *ConsensusInfo
	GetWorkflowStatus() ConsensusWorkflowStatus
	ShouldReceiveMissingRequest(req isc.Request) bool
	GetPipeMetrics() ConsensusPipeMetrics
}

type AsynchronousCommonSubsetRunner interface {
	RunACSConsensus(value []byte, sessionID uint64, stateIndex uint32, callback func(sessionID uint64, acs [][]byte))
	Close()
}

type WAL interface {
	Write(bytes []byte) error
	Contains(i uint32) bool
	Read(i uint32) ([]byte, error)
}

type SyncInfo struct {
	Synced                bool
	SyncedBlockIndex      uint32
	SyncedStateCommitment trie.VCommitment
	SyncedStateTimestamp  time.Time
	StateOutput           *isc.AliasOutputWithID
	StateOutputCommitment trie.VCommitment
	StateOutputTimestamp  time.Time
}

type ConsensusInfo struct {
	StateIndex uint32
	Mempool    mempool.MempoolInfo
	TimerTick  int
	TimeData   time.Time
}

type ConsensusWorkflowStatus interface {
	IsStateReceived() bool
	IsBatchProposalSent() bool
	IsConsensusBatchKnown() bool
	IsVMStarted() bool
	IsVMResultSigned() bool
	IsTransactionFinalized() bool
	IsTransactionPosted() bool
	IsTransactionSeen() bool
	IsInProgress() bool
	GetBatchProposalSentTime() time.Time
	GetConsensusBatchKnownTime() time.Time
	GetVMStartedTime() time.Time
	GetVMResultSignedTime() time.Time
	GetTransactionFinalizedTime() time.Time
	GetTransactionPostedTime() time.Time
	GetTransactionSeenTime() time.Time
	GetCompletedTime() time.Time
	GetCurrentStateIndex() uint32
}

type ConsensusPipeMetrics interface {
	GetEventStateTransitionMsgPipeSize() int
	GetEventPeerLogIndexMsgPipeSize() int
	GetEventInclusionStateMsgPipeSize() int
	GetEventACSMsgPipeSize() int
	GetEventVMResultMsgPipeSize() int
	GetEventTimerMsgPipeSize() int
}

type ReadyListRecord struct {
	Request isc.Calldata
	Seen    map[uint16]bool
}

type CommitteeInfo struct {
	Address       iotago.Address
	Size          uint16
	Quorum        uint16
	QuorumIsAlive bool
	PeerStatus    []*PeerStatus
}

type PeerStatus struct {
	Index     int
	PubKey    *cryptolib.PublicKey
	NetID     string
	Connected bool
}

type ChainTransitionEventData struct {
	IsGovernance    bool
	VirtualState    state.VirtualStateAccess
	ChainOutput     *isc.AliasOutputWithID
	OutputTimestamp time.Time
}

func (p *PeerStatus) String() string {
	return fmt.Sprintf("%+v", *p)
}

type RequestProcessingStatus int

const (
	RequestProcessingStatusUnknown = RequestProcessingStatus(iota)
	RequestProcessingStatusBacklog
	RequestProcessingStatusCompleted
)

const (
	// TimerTickPeriod time tick for consensus and state manager objects
	TimerTickPeriod = 100 * time.Millisecond
)

const (
	PeerMsgTypeMissingRequestIDs = iota
	PeerMsgTypeMissingRequest
	PeerMsgTypeOffLedgerRequest
)
