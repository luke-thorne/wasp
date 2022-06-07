package dashboard

import (
	_ "embed"
	"fmt"
	"net/http"

	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/collections"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
	"github.com/iotaledger/wasp/packages/vm/core/root"
	"github.com/labstack/echo/v4"
)

//go:embed templates/chaincontract.tmpl
var tplChainContract string

func (d *Dashboard) initChainContract(e *echo.Echo, r renderer) {
	route := e.GET("/chain/:chainid/contract/:hname", d.handleChainContract)
	route.Name = "chainContract"
	r[route.Path] = d.makeTemplate(e, tplChainContract, tplWebSocket)
}

func (d *Dashboard) handleChainContract(c echo.Context) error {
	chainID, err := iscp.ChainIDFromString(c.Param("chainid"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("chainid: %v", err))
	}

	hname, err := iscp.HnameFromString(c.Param("hname"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("hname: %v", err))
	}

	result := &ChainContractTemplateParams{
		BaseTemplateParams: d.BaseParams(c, chainBreadcrumb(c.Echo(), chainID), Tab{
			Path:  c.Path(),
			Title: fmt.Sprintf("Contract %d", hname),
			Href:  "#",
		}),
		ChainID: chainID,
		Hname:   hname,
	}

	r, err := d.wasp.CallView(chainID, root.Contract.Name, root.ViewFindContract.Name, codec.MakeDict(map[string]interface{}{
		root.ParamHname: codec.EncodeHname(hname),
	}))
	if err != nil {
		return fmt.Errorf("call view failed: %v", err)
	}
	if r[root.ParamContractRecData] == nil {
		return fmt.Errorf("contract not found")
	}
	result.ContractRecord, err = root.ContractRecordFromBytes(r[root.ParamContractRecData])
	if err != nil {
		return fmt.Errorf("cannot decode contract record: %v", err)
	}

	r, err = d.wasp.CallView(chainID, blocklog.Contract.Name, blocklog.ViewGetEventsForContract.Name, codec.MakeDict(map[string]interface{}{
		blocklog.ParamContractHname: codec.EncodeHname(hname),
	}))
	if err != nil {
		return fmt.Errorf("call view failed: %v", err)
	}

	recs := collections.NewArray16ReadOnly(r, blocklog.ParamEvent)
	result.Log = make([]string, recs.MustLen())
	for i := range result.Log {
		data, err := recs.GetAt(uint16(i))
		if err != nil {
			return err
		}
		result.Log[i] = string(data)
	}

	return c.Render(http.StatusOK, c.Path(), result)
}

type ChainContractTemplateParams struct {
	BaseTemplateParams

	ChainID *iscp.ChainID
	Hname   iscp.Hname

	ContractRecord *root.ContractRecord
	Log            []string
}
