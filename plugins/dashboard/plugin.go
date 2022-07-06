// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/wasp/packages/authentication"
	"github.com/iotaledger/wasp/packages/authentication/shared/permissions"
	"github.com/iotaledger/wasp/packages/chain"
	"github.com/iotaledger/wasp/packages/dashboard"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/kv/optimism"
	"github.com/iotaledger/wasp/packages/metrics/nodeconnmetrics"
	"github.com/iotaledger/wasp/packages/parameters"
	registry_pkg "github.com/iotaledger/wasp/packages/registry"
	"github.com/iotaledger/wasp/packages/vm/viewcontext"
	"github.com/iotaledger/wasp/plugins/chains"
	"github.com/iotaledger/wasp/plugins/peering"
	"github.com/iotaledger/wasp/plugins/registry"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

const PluginName = "Dashboard"

var (
	Server = echo.New()
	log    *logger.Logger
	d      *dashboard.Dashboard
)

func Init() *node.Plugin {
	return node.NewPlugin(PluginName, nil, node.Enabled, configure, run)
}

type waspServices struct{}

var _ dashboard.WaspServices = &waspServices{}

func (w *waspServices) ConfigDump() map[string]interface{} {
	return parameters.Dump()
}

func (*waspServices) WebAPIPort() string {
	port := "80"
	parts := strings.Split(parameters.GetString(parameters.WebAPIBindAddress), ":")
	if len(parts) == 2 {
		port = parts[1]
	}
	return port
}

func (w *waspServices) ExploreAddressBaseURL() string {
	baseURL := parameters.GetString(parameters.DashboardExploreAddressURL)
	if baseURL != "" {
		return baseURL
	}
	// TODO what should be this URL?
	return exploreAddressURLFromL1URI(parameters.GetString("TODO"))
}

func (w *waspServices) PeeringStats() (*dashboard.PeeringStats, error) {
	ret := &dashboard.PeeringStats{}
	peers := peering.DefaultNetworkProvider().PeerStatus()
	ret.Peers = make([]dashboard.Peer, len(peers))
	for i, p := range peers {
		ret.Peers[i] = dashboard.Peer{
			NumUsers: p.NumUsers(),
			NetID:    p.NetID(),
			IsAlive:  p.IsAlive(),
		}
	}
	tpeers, err := peering.DefaultTrustedNetworkManager().TrustedPeers()
	if err != nil {
		return nil, err
	}
	ret.TrustedPeers = make([]dashboard.TrustedPeer, len(tpeers))
	for i, t := range tpeers {
		ret.TrustedPeers[i] = dashboard.TrustedPeer{
			NetID:  t.NetID,
			PubKey: *t.PubKey,
		}
	}
	return ret, nil
}

func (w *waspServices) MyNetworkID() string {
	return peering.DefaultNetworkProvider().Self().NetID()
}

func (w *waspServices) GetChainRecords() ([]*registry_pkg.ChainRecord, error) {
	return registry.DefaultRegistry().GetChainRecords()
}

func (w *waspServices) GetChainRecord(chainID *iscp.ChainID) (*registry_pkg.ChainRecord, error) {
	ch, err := registry.DefaultRegistry().GetChainRecordByChainID(chainID)
	if err != nil {
		return nil, err
	}
	if ch == nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Chain record not found")
	}
	return ch, nil
}

func (w *waspServices) GetChainCommitteeInfo(chainID *iscp.ChainID) (*chain.CommitteeInfo, error) {
	ch := chains.AllChains().Get(chainID)
	if ch == nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Chain not found")
	}
	return ch.GetCommitteeInfo(), nil
}

func (w *waspServices) GetChainNodeConnectionMetrics(chainID *iscp.ChainID) (nodeconnmetrics.NodeConnectionMessagesMetrics, error) {
	ch := chains.AllChains().Get(chainID)
	if ch == nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Chain not found")
	}
	return ch.GetNodeConnectionMetrics(), nil
}

func (w *waspServices) GetNodeConnectionMetrics() (nodeconnmetrics.NodeConnectionMetrics, error) {
	chs := chains.AllChains()
	return chs.GetNodeConnectionMetrics(), nil
}

func (w *waspServices) GetChainConsensusWorkflowStatus(chainID *iscp.ChainID) (chain.ConsensusWorkflowStatus, error) {
	ch := chains.AllChains().Get(chainID)
	if ch == nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Chain not found")
	}
	return ch.GetConsensusWorkflowStatus(), nil
}

func (w *waspServices) GetChainConsensusPipeMetrics(chainID *iscp.ChainID) (chain.ConsensusPipeMetrics, error) {
	ch := chains.AllChains().Get(chainID)
	if ch == nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Chain not found")
	}
	return ch.GetConsensusPipeMetrics(), nil
}

func (w *waspServices) CallView(chainID *iscp.ChainID, scName, funName string, params dict.Dict) (dict.Dict, error) {
	ch := chains.AllChains().Get(chainID)
	if ch == nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Chain not found")
	}
	vctx := viewcontext.New(ch)
	var ret dict.Dict
	err := optimism.RetryOnStateInvalidated(func() error {
		var err error
		ret, err = vctx.CallViewExternal(iscp.Hn(scName), iscp.Hn(funName), params)
		return err
	})
	return ret, err
}

func exploreAddressURLFromL1URI(uri string) string {
	url := strings.Split(uri, ":")[0] + ":8081/explorer/address"
	if !strings.HasPrefix(url, "http") {
		return "http://" + url
	}
	return url
}

func configure(*node.Plugin) {
	log = logger.NewLogger(PluginName)

	Server.HideBanner = true
	Server.HidePort = true
	Server.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: `${time_rfc3339_nano} ${remote_ip} ${method} ${uri} ${status} error="${error}"` + "\n",
	}))
	Server.Use(middleware.Recover())

	claimValidator := func(claims *authentication.WaspClaims) bool {
		// The Dashboard will be accessible if the token has a 'Dashboard' claim
		return claims.HasPermission(permissions.Dashboard)
	}

	authentication.AddAuthentication(Server, registry.DefaultRegistry, parameters.DashboardAuth, claimValidator)

	d = dashboard.Init(Server, &waspServices{}, log)
}

func run(_ *node.Plugin) {
	log.Infof("Starting %s ...", PluginName)
	if err := daemon.BackgroundWorker(PluginName, worker); err != nil {
		log.Errorf("error starting as daemon: %s", err)
	}
}

func worker(ctx context.Context) {
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		bindAddr := parameters.GetString(parameters.DashboardBindAddress)
		log.Infof("%s started, bind address=%s", PluginName, bindAddr)
		if err := Server.Start(bindAddr); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				log.Errorf("error serving: %s", err)
			}
		}
	}()

	select {
	case <-ctx.Done():
	case <-stopped:
	}

	log.Infof("Stopping %s ...", PluginName)
	defer log.Infof("Stopping %s ... done", PluginName)

	d.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := Server.Shutdown(ctx); err != nil {
		log.Errorf("error stopping: %s", err)
	}
}
