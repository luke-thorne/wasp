package webapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	metricspkg "github.com/iotaledger/wasp/packages/metrics"
	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/wasp"
	"github.com/iotaledger/wasp/packages/webapi"
	"github.com/iotaledger/wasp/packages/webapi/httperrors"
	"github.com/iotaledger/wasp/plugins/chains"
	"github.com/iotaledger/wasp/plugins/dkg"
	"github.com/iotaledger/wasp/plugins/gracefulshutdown"
	"github.com/iotaledger/wasp/plugins/metrics"
	"github.com/iotaledger/wasp/plugins/peering"
	"github.com/iotaledger/wasp/plugins/registry"
	"github.com/iotaledger/wasp/plugins/wal"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pangpanglabs/echoswagger/v2"
)

// PluginName is the name of the web API plugin.
const PluginName = "WebAPI"

var (
	Server echoswagger.ApiRoot

	log        *logger.Logger
	allMetrics *metricspkg.Metrics
)

func Init() *node.Plugin {
	return node.NewPlugin(PluginName, nil, node.Enabled, configure, run)
}

func configure(*node.Plugin) {
	log = logger.NewLogger(PluginName)
}

func run(_ *node.Plugin) {
	log.Infof("Starting %s ...", PluginName)
	if err := daemon.BackgroundWorker("WebAPI Server", worker, parameters.PriorityWebAPI); err != nil {
		log.Errorf("error starting as daemon: %s", err)
	}
}

func worker(ctx context.Context) {
	initWebAPI()
	stopped := make(chan struct{})
	server := Server.Echo()
	go func() {
		defer close(stopped)
		bindAddr := parameters.GetString(parameters.WebAPIBindAddress)
		log.Infof("%s started, bind-address=%s", PluginName, bindAddr)
		if err := server.Start(bindAddr); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				log.Errorf("error serving: %s", err)
			}
		}
	}()

	// stop if we are shutting down or the server could not be started
	select {
	case <-ctx.Done():
	case <-stopped:
	}

	log.Infof("Stopping %s ...", PluginName)
	defer log.Infof("Stopping %s ... done", PluginName)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Errorf("error stopping: %s", err)
	}
}

func initWebAPI() {
	Server = echoswagger.New(echo.New(), "/doc", &echoswagger.Info{
		Title:       "Wasp API",
		Description: "REST API for the Wasp node",
		Version:     wasp.Version,
	})

	Server.Echo().HideBanner = true
	Server.Echo().HidePort = true
	Server.Echo().HTTPErrorHandler = httperrors.HTTPErrorHandler
	Server.Echo().Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: `${time_rfc3339_nano} ${remote_ip} ${method} ${uri} ${status} error="${error}"` + "\n",
	}))
	Server.Echo().Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept},
		AllowMethods: []string{"*"},
	}))

	network := peering.DefaultNetworkProvider()
	if network == nil {
		panic("dependency NetworkProvider is missing in WebAPI")
	}
	tnm := peering.DefaultTrustedNetworkManager()
	if tnm == nil {
		panic("dependency TrustedNetworkManager is missing in WebAPI")
	}
	if parameters.GetBool(parameters.MetricsEnabled) {
		allMetrics = metrics.AllMetrics()
	}
	webapi.Init(
		Server,
		network,
		tnm,
		registry.DefaultRegistry,
		chains.AllChains,
		dkg.DefaultNode,
		gracefulshutdown.Shutdown,
		allMetrics,
		wal.GetWAL(),
	)
}
