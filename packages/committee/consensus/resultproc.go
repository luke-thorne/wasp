package consensus

import (
	"fmt"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	valuetransaction "github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/wasp/packages/committee"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/sctransaction"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm"
	"github.com/iotaledger/wasp/plugins/runvm"
)

type runCalculationsParams struct {
	requests        []*request
	leaderPeerIndex uint16
	balances        map[valuetransaction.ID][]*balance.Balance
	rewardAddress   address.Address
	timestamp       int64
}

// runs the VM for the request and posts result to committee's queue
func (op *operator) runCalculationsAsync(par runCalculationsParams) {
	if op.variableState == nil {
		op.log.Debugf("runCalculationsAsync: variable state is not known")
		return
	}
	if !op.processorReady {
		op.log.Debugf("runCalculationsAsync: processor not ready")
		return
	}

	progHashStr, ok := op.getProgramHashStr()
	if !ok {
		return
	}
	progHash, err := hashing.HashValueFromString(progHashStr)
	if err != nil {
		return
	}
	ctx := &vm.VMTask{
		LeaderPeerIndex: par.leaderPeerIndex,
		ProgramHash:     progHash,
		Address:         *op.committee.Address(),
		Color:           *op.committee.Color(),
		Balances:        par.balances,
		OwnerAddress:    op.getOwnerAddress(),
		RewardAddress:   op.getRewardAddress(),
		Requests:        takeRefs(par.requests),
		Timestamp:       par.timestamp,
		VariableState:   op.variableState,
		Log:             op.log,
	}
	ctx.OnFinish = func() {
		op.committee.ReceiveMessage(ctx)
	}
	if err := runvm.RunComputationsAsync(ctx); err != nil {
		op.log.Errorf("RunComputationsAsync: %v", err)
	}
}

func (op *operator) sendResultToTheLeader(result *vm.VMTask) {
	op.log.Debugw("sendResultToTheLeader")

	sigShare, err := op.dkshare.SignShare(result.ResultTransaction.EssenceBytes())
	if err != nil {
		op.log.Errorf("error while signing transaction %v", err)
		return
	}

	reqids := make([]sctransaction.RequestId, len(result.Requests))
	for i := range reqids {
		reqids[i] = *result.Requests[i].RequestId()
	}

	essenceHash := hashing.HashData(result.ResultTransaction.EssenceBytes())
	batchHash := vm.BatchHash(reqids, result.Timestamp)

	op.log.Debugw("sendResultToTheLeader",
		"leader", result.LeaderPeerIndex,
		"batchHash", batchHash.String(),
		"essenceHash", essenceHash.String(),
		"ts", result.Timestamp,
	)

	msgData := util.MustBytes(&committee.SignedHashMsg{
		PeerMsgHeader: committee.PeerMsgHeader{
			StateIndex: op.mustStateIndex(),
		},
		BatchHash:     batchHash,
		OrigTimestamp: result.Timestamp,
		EssenceHash:   *essenceHash,
		SigShare:      sigShare,
	})

	if err := op.committee.SendMsg(result.LeaderPeerIndex, committee.MsgSignedHash, msgData); err != nil {
		op.log.Error(err)
	}
}

func (op *operator) saveOwnResult(result *vm.VMTask) {
	sigShare, err := op.dkshare.SignShare(result.ResultTransaction.EssenceBytes())
	if err != nil {
		op.log.Errorf("error while signing transaction %v", err)
		return
	}

	reqids := make([]sctransaction.RequestId, len(result.Requests))
	for i := range reqids {
		reqids[i] = *result.Requests[i].RequestId()
	}

	bh := vm.BatchHash(reqids, result.Timestamp)
	if bh != op.leaderStatus.batchHash {
		panic("bh != op.leaderStatus.batchHash")
	}
	if len(result.Requests) != int(result.ResultBatch.Size()) {
		panic("len(result.Requests) != int(result.ResultBatch.Size())")
	}

	essenceHash := hashing.HashData(result.ResultTransaction.EssenceBytes())
	op.log.Debugw("saveOwnResult",
		"batchHash", bh.String(),
		"ts", result.Timestamp,
		"essenceHash", essenceHash.String(),
	)

	op.leaderStatus.resultTx = result.ResultTransaction
	op.leaderStatus.batch = result.ResultBatch
	op.leaderStatus.signedResults[op.committee.OwnPeerIndex()] = &signedResult{
		essenceHash: *essenceHash,
		sigShare:    sigShare,
	}
}

func (op *operator) aggregateSigShares(sigShares [][]byte) error {
	resTx := op.leaderStatus.resultTx

	finalSignature, err := op.dkshare.RecoverFullSignature(sigShares, resTx.EssenceBytes())
	if err != nil {
		return err
	}
	if err := resTx.PutSignature(finalSignature); err != nil {
		return fmt.Errorf("something wrong while aggregating final signature: %v", err)
	}
	return nil
}
