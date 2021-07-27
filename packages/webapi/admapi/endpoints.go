// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package admapi

import (
	"net"
	"strings"

	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/wasp/packages/chains"
	"github.com/iotaledger/wasp/packages/dkg"
	"github.com/iotaledger/wasp/packages/peering"
	"github.com/iotaledger/wasp/packages/registry"
	"github.com/labstack/echo/v4"
	"github.com/pangpanglabs/echoswagger/v2"
)

var log *logger.Logger

func initLogger() {
	log = logger.NewLogger("webapi/adm")
}

func AddEndpoints(
	adm echoswagger.ApiGroup,
	adminWhitelist []net.IP,
	network peering.NetworkProvider,
	tnm peering.TrustedNetworkManager,
	registryProvider registry.Provider,
	chainsProvider chains.Provider,
	nodeProvider dkg.NodeProvider,
	shutdown ShutdownFunc,
) {
	initLogger()

	adm.EchoGroup().Use(protected(adminWhitelist))

	addShutdownEndpoint(adm, shutdown)
	addChainRecordEndpoints(adm, registryProvider)
	addCommitteeRecordEndpoints(adm, registryProvider, chainsProvider)
	addChainEndpoints(adm, registryProvider, chainsProvider)
	addDKSharesEndpoints(adm, registryProvider, nodeProvider)
	addPeeringEndpoints(adm, network, tnm)
}

// allow only if the remote address is private or in whitelist
// TODO this is a very basic/limited form of protection
func protected(whitelist []net.IP) echo.MiddlewareFunc {
	isAllowed := func(ip net.IP) bool {
		if ip.IsLoopback() {
			return true
		}
		for _, whitelistedIP := range whitelist {
			if ip.Equal(whitelistedIP) {
				return true
			}
		}
		return false
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			parts := strings.Split(c.Request().RemoteAddr, ":")
			if len(parts) == 2 {
				ip := net.ParseIP(parts[0])
				if ip != nil && isAllowed(ip) {
					return next(c)
				}
			}
			log.Warnf("Blocking request from %s: %s %s", c.Request().RemoteAddr, c.Request().Method, c.Request().RequestURI)
			return echo.ErrUnauthorized
		}
	}
}
