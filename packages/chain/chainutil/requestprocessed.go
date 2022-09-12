package chainutil

import (
	"github.com/iotaledger/wasp/packages/chain"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
)

func HasRequestBeenProcessed(ch chain.ChainCore, reqID isc.RequestID) (bool, error) {
	res, err := CallView(ch, blocklog.Contract.Hname(), blocklog.ViewIsRequestProcessed.Hname(),
		dict.Dict{
			blocklog.ParamRequestID: reqID.Bytes(),
		})
	if err != nil {
		return false, err
	}
	pEncoded, err := res.Get(blocklog.ParamRequestProcessed)
	if err != nil {
		return false, err
	}
	return codec.DecodeBool(pEncoded, false)
}
