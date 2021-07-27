package webapi

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/util/auth"
	"github.com/iotaledger/wasp/packages/webapi"
	"github.com/iotaledger/wasp/packages/webapi/httperrors"
	"github.com/iotaledger/wasp/plugins/chains"
	"github.com/iotaledger/wasp/plugins/dkg"
	"github.com/iotaledger/wasp/plugins/gracefulshutdown"
	"github.com/iotaledger/wasp/plugins/peering"
	"github.com/iotaledger/wasp/plugins/registry"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pangpanglabs/echoswagger/v2"
)

// PluginName is the name of the web API plugin.
const PluginName = "WebAPI"

var (
	Server echoswagger.ApiRoot

	log *logger.Logger
)

func Init() *node.Plugin {
	Plugin := node.NewPlugin(PluginName, node.Enabled, configure, run)
	return Plugin
}

func configure(*node.Plugin) {
	log = logger.NewLogger(PluginName)

	Server = echoswagger.New(echo.New(), "/doc", &echoswagger.Info{
		Title:       "Wasp API",
		Description: "REST API for the IOTA Wasp node",
		Version:     "0.1",
	})

	Server.Echo().HideBanner = true
	Server.Echo().HidePort = true
	Server.Echo().HTTPErrorHandler = customHTTPErrorHandler
	Server.Echo().Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: `${time_rfc3339_nano} ${remote_ip} ${method} ${uri} ${status} error="${error}"` + "\n",
	}))

	auth.AddAuthentication(Server.Echo(), parameters.GetStringToString(parameters.WebAPIAuth))

	network := peering.DefaultNetworkProvider()
	if network == nil {
		panic("dependency NetworkProvider is missing in WebAPI")
	}
	tnm := peering.DefaultTrustedNetworkManager()
	if tnm == nil {
		panic("dependency TrustedNetworkManager is missing in WebAPI")
	}
	webapi.Init(
		Server,
		adminWhitelist(),
		network,
		tnm,
		registry.DefaultRegistry,
		chains.AllChains,
		dkg.DefaultNode,
		gracefulshutdown.Shutdown,
	)
}

func customHTTPErrorHandler(err error, c echo.Context) {
	he, ok := err.(*httperrors.HTTPError)
	if ok {
		if !c.Response().Committed {
			if c.Request().Method == http.MethodHead { // Issue #608
				err = c.NoContent(he.Code)
			} else {
				err = c.JSON(he.Code, he)
			}
		}
	}
	c.Echo().DefaultHTTPErrorHandler(err, c)
}

func adminWhitelist() []net.IP {
	r := make([]net.IP, 0)
	for _, ip := range parameters.GetStringSlice(parameters.WebAPIAdminWhitelist) {
		r = append(r, net.ParseIP(ip))
	}
	return r
}

func run(_ *node.Plugin) {
	log.Infof("Starting %s ...", PluginName)
	if err := daemon.BackgroundWorker("WebAPI Server", worker, parameters.PriorityWebAPI); err != nil {
		log.Errorf("Error starting as daemon: %s", err)
	}
}

func worker(shutdownSignal <-chan struct{}) {
	stopped := make(chan struct{})
	server := Server.Echo()
	go func() {
		defer close(stopped)
		bindAddr := parameters.GetString(parameters.WebAPIBindAddress)
		log.Infof("%s started, bind-address=%s", PluginName, bindAddr)
		if err := server.Start(bindAddr); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				log.Errorf("Error serving: %s", err)
			}
		}
	}()

	// stop if we are shutting down or the server could not be started
	select {
	case <-shutdownSignal:
	case <-stopped:
	}

	log.Infof("Stopping %s ...", PluginName)
	defer log.Infof("Stopping %s ... done", PluginName)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Errorf("Error stopping: %s", err)
	}
}
