// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	_ "embed"
	"encoding/hex"
	"html/template"
	"strings"

	"github.com/iotaledger/wasp/packages/authentication"
	"github.com/iotaledger/wasp/packages/webapi/routes"

	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/wasp/packages/chain"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/metrics/nodeconnmetrics"
	"github.com/iotaledger/wasp/packages/registry"
	"github.com/iotaledger/wasp/packages/wasp"
	"github.com/labstack/echo/v4"
	"github.com/mr-tron/base58"
)

//go:embed templates/base.tmpl
var tplBase string

type Tab struct {
	Path       string
	Title      string
	Href       string
	Breadcrumb bool
}

type BaseTemplateParams struct {
	IsAuthenticated bool
	NavPages        []Tab
	Breadcrumbs     []Tab
	Path            string
	MyNetworkID     string
	Version         string
}

type WaspServices interface {
	ConfigDump() map[string]interface{}
	ExploreAddressBaseURL() string
	WebAPIPort() string
	PeeringStats() (*PeeringStats, error)
	MyNetworkID() string
	GetChainRecords() ([]*registry.ChainRecord, error)
	GetChainRecord(chainID *isc.ChainID) (*registry.ChainRecord, error)
	GetChainCommitteeInfo(chainID *isc.ChainID) (*chain.CommitteeInfo, error)
	CallView(chainID *isc.ChainID, scName, fname string, params dict.Dict) (dict.Dict, error)
	GetChainNodeConnectionMetrics(*isc.ChainID) (nodeconnmetrics.NodeConnectionMessagesMetrics, error)
	GetNodeConnectionMetrics() (nodeconnmetrics.NodeConnectionMetrics, error)
	GetChainConsensusWorkflowStatus(*isc.ChainID) (chain.ConsensusWorkflowStatus, error)
	GetChainConsensusPipeMetrics(*isc.ChainID) (chain.ConsensusPipeMetrics, error)
}

type Dashboard struct {
	navPages []Tab
	stop     chan bool
	wasp     WaspServices
	log      *logger.Logger
}

func Init(server *echo.Echo, waspServices WaspServices, log *logger.Logger) *Dashboard {
	r := renderer{}
	server.Renderer = r

	d := &Dashboard{
		stop: make(chan bool),
		wasp: waspServices,
		log:  log.Named("dashboard"),
	}

	d.errorInit(server, r)

	d.navPages = []Tab{
		d.authInit(server, r),
		d.configInit(server, r),
		d.peeringInit(server, r),
		d.chainsInit(server, r),
		d.metricsInit(server, r),
	}

	d.webSocketInit(server)

	return d
}

func (d *Dashboard) Stop() {
	close(d.stop)
}

func (d *Dashboard) BaseParams(c echo.Context, breadcrumbs ...Tab) BaseTemplateParams {
	var isAuthenticated bool

	auth, ok := c.Get("auth").(*authentication.AuthContext)

	if !ok {
		isAuthenticated = false
	} else {
		isAuthenticated = auth.IsAuthenticated()
	}

	return BaseTemplateParams{
		IsAuthenticated: isAuthenticated,
		NavPages:        d.navPages,
		Breadcrumbs:     breadcrumbs,
		Path:            c.Path(),
		MyNetworkID:     d.wasp.MyNetworkID(),
		Version:         wasp.Version,
	}
}

func (d *Dashboard) makeTemplate(e *echo.Echo, parts ...string) *template.Template {
	t := template.New("").Funcs(template.FuncMap{
		"formatTimestamp":        formatTimestamp,
		"formatTimestampOrNever": formatTimestampOrNever,
		"exploreAddressUrl":      d.exploreAddressURL,
		"args":                   args,
		"hashref":                hashref,
		"chainidref":             chainIDref,
		"assedID":                assetID,
		"trim":                   trim,
		"incUint32":              incUint32,
		"decUint32":              decUint32,
		"bytesToString":          bytesToString,
		"addressToString":        d.addressToString,
		"agentIDToString":        d.agentIDToString,
		"addressFromAgentID":     d.addressFromAgentID,
		"getETHAddress":          d.getETHAddress,
		"isETHAddress":           d.isETHAddress,
		"isValidAddress":         d.isValidAddress,
		"keyToString":            keyToString,
		"anythingToString":       anythingToString,
		"base58":                 base58.Encode,
		"hex":                    hex.EncodeToString,
		"replace":                strings.Replace,
		"webapiPort":             d.wasp.WebAPIPort,
		"evmJSONRPCEndpoint":     routes.EVMJSONRPC,
		"uri":                    func(s string, p ...interface{}) string { return e.Reverse(s, p...) },
	})
	t = template.Must(t.Parse(tplBase))
	for _, part := range parts {
		t = template.Must(t.Parse(part))
	}
	return t
}
