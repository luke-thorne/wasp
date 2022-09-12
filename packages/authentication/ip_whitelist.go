package authentication

import (
	"net"
	"strings"

	"github.com/labstack/echo/v4"
)

func AddIPWhiteListAuth(webAPI WebAPI, config IPWhiteListAuthConfiguration) {
	ipWhiteList := createIPWhiteList(config)
	webAPI.Use(protected(ipWhiteList))
}

func createIPWhiteList(config IPWhiteListAuthConfiguration) []net.IP {
	r := make([]net.IP, 0)
	for _, ip := range config.IPWhiteList {
		r = append(r, net.ParseIP(ip))
	}
	return r
}

func isAllowed(ip net.IP, whitelist []net.IP) bool {
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

func protected(whitelist []net.IP) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authContext := c.Get("auth").(*AuthContext)

			parts := strings.Split(c.Request().RemoteAddr, ":")
			if len(parts) == 2 {
				ip := net.ParseIP(parts[0])
				if ip != nil && isAllowed(ip, whitelist) {
					authContext.isAuthenticated = true
					return next(c)
				}
			}

			c.Logger().Infof("Blocking request from %s: %s %s", c.Request().RemoteAddr, c.Request().Method, c.Request().RequestURI)
			return echo.ErrUnauthorized
		}
	}
}
