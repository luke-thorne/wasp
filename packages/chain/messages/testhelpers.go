package messages

import "github.com/iotaledger/wasp/packages/peering"

func MsgTypeToString(msg interface{}) string {
	switch msgt := msg.(type) {
	case *peering.PeerMessage:
		return "PeerMessage::" + PeerMessageTypeToString(msgt.MsgType)
	case *DismissChainMsg:
		return "DismissChainMsg"
	case *StateTransitionMsg:
		return "StateTransitionMsg"
	case *StateCandidateMsg:
		return "StateCandidateMsg"
	case *InclusionStateMsg:
		return "InclusionStateMsg"
	case *StateMsg:
		return "StateMsg"
	case *VMResultMsg:
		return "VMResultMsg"
	case *AsynchronousCommonSubsetMsg:
		return "AsynchronousCommonSubsetMsg"
	case TimerTick:
		return "TimerTick"
	default:
		return "(unknown msg)"
	}
}

var peerMsgTypesString = map[byte]string{
	MsgGetBlock:          "MsgGetBlock",
	MsgBlock:             "MsgBlock",
	MsgSignedResult:      "MsgSignedResult",
	MsgSignedResultAck:   "MsgSignedResultAck",
	MsgOffLedgerRequest:  "MsgOffLedgerRequest",
	MsgMissingRequestIDs: "MsgMissingRequestIDs",
	MsgMissingRequest:    "MsgMissingRequest",
	MsgRequestAck:        "MsgRequestAck",
}

func PeerMessageTypeToString(msg byte) string {
	ret, ok := peerMsgTypesString[msg]
	if !ok {
		return "(wrong msg type)"
	}
	return ret
}
