package dashboard

import (
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/collections"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/core/root"
)

type ChainInfo struct {
	*governance.ChainInfo
	Contracts map[isc.Hname]*root.ContractRecord
}

func (d *Dashboard) fetchChainInfo(chainID *isc.ChainID) (ret *ChainInfo, err error) {
	info, err := d.wasp.CallView(chainID, governance.Contract.Name, governance.ViewGetChainInfo.Name, nil)
	if err != nil {
		d.log.Error(err)
		return
	}

	ret = &ChainInfo{}

	if ret.ChainInfo, err = governance.GetChainInfo(info); err != nil {
		return nil, err
	}

	recs, err := d.wasp.CallView(chainID, root.Contract.Name, root.ViewGetContractRecords.Name, nil)
	if err != nil {
		return
	}
	ret.Contracts, err = root.DecodeContractRegistry(collections.NewMapReadOnly(recs, root.StateVarContractRegistry))
	if err != nil {
		return
	}

	return ret, err
}
