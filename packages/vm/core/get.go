package core

import (
	"fmt"

	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/iscp/coreutil"
	"github.com/iotaledger/wasp/packages/vm/core/_default"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/core/accounts/commonaccount"
	"github.com/iotaledger/wasp/packages/vm/core/blob"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/core/root"
)

var AllCoreContractsByHash = map[hashing.HashValue]*coreutil.ContractProcessor{
	_default.Contract.ProgramHash:   _default.Processor,
	root.Contract.ProgramHash:       root.Processor,
	accounts.Contract.ProgramHash:   accounts.Processor,
	blob.Contract.ProgramHash:       blob.Processor,
	blocklog.Contract.ProgramHash:   blocklog.Processor,
	governance.Contract.ProgramHash: governance.Processor,
}

func init() {
	for _, rec := range AllCoreContractsByHash {
		commonaccount.SetCoreHname(rec.Contract.Hname())
	}
}

func GetProcessor(programHash hashing.HashValue) (iscp.VMProcessor, error) {
	ret, ok := AllCoreContractsByHash[programHash]
	if !ok {
		return nil, fmt.Errorf("can't find builtin processor with hash %s", programHash.String())
	}
	return ret, nil
}
