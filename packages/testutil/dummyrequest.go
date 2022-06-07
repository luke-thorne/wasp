package testutil

import (
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/testutil/testkey"
)

func DummyOffledgerRequest(chainID *iscp.ChainID) iscp.OffLedgerRequest {
	contract := iscp.Hn("somecontract")
	entrypoint := iscp.Hn("someentrypoint")
	args := dict.Dict{}
	req := iscp.NewOffLedgerRequest(chainID, contract, entrypoint, args, 0)
	keys, _ := testkey.GenKeyAddr()
	return req.Sign(keys)
}
