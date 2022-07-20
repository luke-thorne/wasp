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
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/iscp/coreutil"
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
	ID() *iscp.ChainID
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
	EnqueueAliasOutput(*iscp.AliasOutputWithID)
}

// ChainEntry interface to access chain from the chain registry side
type ChainEntry interface {
	Dismiss(reason string)
	IsDismissed() bool
}

// ChainRequests is an interface to query status of the request
type ChainRequests interface {
	GetRequestReceipt(id iscp.RequestID) (*blocklog.RequestReceipt, error)
	ResolveError(e *iscp.UnresolvedVMError) (*iscp.VMError, error)
	AttachToRequestProcessed(func(iscp.RequestID)) (attachID *events.Closure)
	DetachFromRequestProcessed(attachID *events.Closure)
	EnqueueOffLedgerRequestMsg(msg *messages.OffLedgerRequestMsgIn)
}

type ChainMetrics interface {
	GetNodeConnectionMetrics() nodeconnmetrics.NodeConnectionMessagesMetrics
	GetConsensusWorkflowStatus() ConsensusWorkflowStatus
	GetConsensusPipeMetrics() ConsensusPipeMetrics
}

type ChainRunner interface {
	GetAnchorOutput() *iscp.AliasOutputWithID
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
	NodeConnectionAliasOutputHandlerFun     func(*iscp.AliasOutputWithID)
	NodeConnectionOnLedgerRequestHandlerFun func(iscp.OnLedgerRequest)
	NodeConnectionInclusionStateHandlerFun  func(iotago.TransactionID, string)
	NodeConnectionMilestonesHandlerFun      func(*nodeclient.MilestoneInfo)
)

type NodeConnection interface {
	RegisterChain(chainID *iscp.ChainID, stateOutputHandler, outputHandler func(iotago.OutputID, iotago.Output))
	UnregisterChain(chainID *iscp.ChainID)

	PublishStateTransaction(chainID *iscp.ChainID, stateIndex uint32, tx *iotago.Transaction) error
	PublishGovernanceTransaction(chainID *iscp.ChainID, tx *iotago.Transaction) error
	PullLatestOutput(chainID *iscp.ChainID)
	PullTxInclusionState(chainID *iscp.ChainID, txid iotago.TransactionID)
	PullStateOutputByID(chainID *iscp.ChainID, id *iotago.UTXOInput)

	AttachTxInclusionStateEvents(chainID *iscp.ChainID, handler NodeConnectionInclusionStateHandlerFun) (*events.Closure, error)
	DetachTxInclusionStateEvents(chainID *iscp.ChainID, closure *events.Closure) error
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
	EnqueueAliasOutput(*iscp.AliasOutputWithID)
	EnqueueStateCandidateMsg(state.VirtualStateAccess, *iotago.UTXOInput)
	EnqueueTimerMsg(msg messages.TimerTick)
	GetStatusSnapshot() *SyncInfo
	SetChainPeers(peers []*cryptolib.PublicKey)
	Close()
}

type Consensus interface {
	EnqueueStateTransitionMsg(bool, state.VirtualStateAccess, *iscp.AliasOutputWithID, time.Time)
	EnqueueSignedResultMsg(*messages.SignedResultMsgIn)
	EnqueueSignedResultAckMsg(*messages.SignedResultAckMsgIn)
	EnqueueTxInclusionsStateMsg(iotago.TransactionID, string)
	EnqueueAsynchronousCommonSubsetMsg(msg *messages.AsynchronousCommonSubsetMsg)
	EnqueueVMResultMsg(msg *messages.VMResultMsg)
	EnqueueTimerMsg(messages.TimerTick)
	IsReady() bool
	Close()
	GetStatusSnapshot() *ConsensusInfo
	GetWorkflowStatus() ConsensusWorkflowStatus
	ShouldReceiveMissingRequest(req iscp.Request) bool
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
	StateOutput           *iscp.AliasOutputWithID
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
	GetEventSignedResultMsgPipeSize() int
	GetEventSignedResultAckMsgPipeSize() int
	GetEventInclusionStateMsgPipeSize() int
	GetEventACSMsgPipeSize() int
	GetEventVMResultMsgPipeSize() int
	GetEventTimerMsgPipeSize() int
}

type ReadyListRecord struct {
	Request iscp.Calldata
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
	ChainOutput     *iscp.AliasOutputWithID
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
