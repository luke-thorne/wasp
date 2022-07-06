package rotate

import (
	"time"

	"github.com/iotaledger/hive.go/identity"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/iscp/coreutil"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/parameters"
)

// IsRotateStateControllerRequest determines if request may be a committee rotation request
func IsRotateStateControllerRequest(req iscp.Calldata) bool {
	target := req.CallTarget()
	return target.Contract == coreutil.CoreContractGovernanceHname && target.EntryPoint == coreutil.CoreEPRotateStateControllerHname
}

func NewRotateRequestOffLedger(chainID *iscp.ChainID, newStateAddress iotago.Address, keyPair *cryptolib.KeyPair) iscp.Request {
	args := dict.New()
	args.Set(coreutil.ParamStateControllerAddress, codec.EncodeAddress(newStateAddress))
	nonce := uint64(time.Now().UnixNano())
	ret := iscp.NewOffLedgerRequest(chainID, coreutil.CoreContractGovernanceHname, coreutil.CoreEPRotateStateControllerHname, args, nonce)
	return ret.Sign(keyPair)
}

func MakeRotateStateControllerTransaction(
	nextAddr iotago.Address,
	chainInput *iscp.AliasOutputWithID,
	ts time.Time,
	accessPledge, consensusPledge identity.ID,
) (*iotago.TransactionEssence, error) {
	output := chainInput.GetAliasOutput().Clone().(*iotago.AliasOutput)
	for i := range output.Conditions {
		if _, ok := output.Conditions[i].(*iotago.StateControllerAddressUnlockCondition); ok {
			output.Conditions[i] = &iotago.StateControllerAddressUnlockCondition{Address: nextAddr}
		}
		// TODO: it is probably not the correct way to do the governance transition
		if _, ok := output.Conditions[i].(*iotago.GovernorAddressUnlockCondition); ok {
			output.Conditions[i] = &iotago.GovernorAddressUnlockCondition{Address: nextAddr}
		}
	}
	result := &iotago.TransactionEssence{
		NetworkID: parameters.L1.Protocol.NetworkID(),
		Inputs:    iotago.Inputs{chainInput.ID()},
		Outputs:   iotago.Outputs{output},
		Payload:   nil,
	}
	inputsCommitment := iotago.Outputs{chainInput.GetAliasOutput()}.MustCommitment()
	copy(result.InputsCommitment[:], inputsCommitment)
	return result, nil
}
