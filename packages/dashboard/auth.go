// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	_ "embed"
	"net/http"

	"github.com/iotaledger/wasp/packages/authentication/shared"

	"github.com/iotaledger/wasp/packages/authentication"

	"github.com/labstack/echo/v4"
)

//go:embed templates/auth.tmpl
var tplLogin string

func (d *Dashboard) authInit(e *echo.Echo, r renderer) Tab {
	e.GET(shared.AuthRouteSuccess(), d.handleAuthCheck)
	e.GET("/", d.handleAuthCheck)

	route := e.GET(shared.AuthRoute(), d.RenderAuthView)
	route.Name = "auth"

	r[route.Path] = d.makeTemplate(e, tplLogin)

	return Tab{
		Path:  route.Path,
		Title: "Auth",
		Href:  route.Path,
	}
}

func (d *Dashboard) RenderAuthView(c echo.Context) error {
	auth, ok := c.Get("auth").(*authentication.AuthContext)

	if ok && auth.IsAuthenticated() {
		return c.Redirect(http.StatusFound, "/config")
	}

	// TODO: Add sessions?
	loginError := c.QueryParam("error")

	return c.Render(http.StatusOK, "/auth", &AuthTemplateParams{
		BaseTemplateParams: d.BaseParams(c),
		Configuration:      d.wasp.ConfigDump(),
		LoginError:         loginError,
	})
}

func (d *Dashboard) handleAuthCheck(c echo.Context) error {
	auth, ok := c.Get("auth").(*authentication.AuthContext)

	if !ok {
		return c.Redirect(http.StatusFound, shared.AuthRoute())
	}

	if auth.IsAuthenticated() {
		return c.Redirect(http.StatusFound, "/config")
	}

	return c.Redirect(http.StatusFound, shared.AuthRoute())
}

type AuthTemplateParams struct {
	BaseTemplateParams
	Configuration map[string]interface{}
	LoginError    string
}
