package oidc

import (
	"net/http"

	"github.com/eduardolat/pgbackweb/internal/logger"
	"github.com/eduardolat/pgbackweb/internal/service"
	"github.com/eduardolat/pgbackweb/internal/view/middleware"
	"github.com/labstack/echo/v4"
)

type handlers struct {
	servs *service.Service
}

func MountRouter(
	parent *echo.Group, mids *middleware.Middleware, servs *service.Service,
) {
	if !servs.OIDCService.IsEnabled() {
		return
	}

	h := handlers{servs: servs}

	requireNoAuth := parent.Group("", mids.RequireNoAuth)

	requireNoAuth.GET("/oidc/login", h.oidcLoginHandler)
	requireNoAuth.GET("/oidc/callback", h.oidcCallbackHandler)
}

func (h *handlers) oidcLoginHandler(c echo.Context) error {
	state, err := h.servs.OIDCService.GenerateState()
	if err != nil {
		logger.Error("failed to generate OIDC state", logger.KV{
			"ip":    c.RealIP(),
			"ua":    c.Request().UserAgent(),
			"error": err,
		})
		return c.String(http.StatusInternalServerError, "Internal server error")
	}

	// Store state in session/cookie for verification
	c.SetCookie(&http.Cookie{
		Name:     "oidc_state",
		Value:    state,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   300, // 5 minutes
		Path:     "/",
	})

	authURL := h.servs.OIDCService.GetAuthURL(state)
	return c.Redirect(http.StatusFound, authURL)
}

func (h *handlers) oidcCallbackHandler(c echo.Context) error {
	ctx := c.Request().Context()

	// Verify state parameter
	state := c.QueryParam("state")
	stateCookie, err := c.Cookie("oidc_state")
	if err != nil || stateCookie.Value != state {
		logger.Error("OIDC state mismatch", logger.KV{
			"ip":       c.RealIP(),
			"ua":       c.Request().UserAgent(),
			"state":    state,
			"expected": stateCookie.Value,
		})
		return c.String(http.StatusBadRequest, "Invalid state parameter")
	}

	// Clear the state cookie
	c.SetCookie(&http.Cookie{
		Name:     "oidc_state",
		Value:    "",
		HttpOnly: true,
		MaxAge:   -1,
		Path:     "/",
	})

	code := c.QueryParam("code")
	if code == "" {
		return c.String(http.StatusBadRequest, "Missing authorization code")
	}

	// Exchange code for user info
	userInfo, err := h.servs.OIDCService.ExchangeCode(ctx, code)
	if err != nil {
		logger.Error("failed to exchange OIDC code", logger.KV{
			"ip":    c.RealIP(),
			"ua":    c.Request().UserAgent(),
			"error": err,
		})
		return c.String(http.StatusInternalServerError, "Failed to authenticate")
	}

	// Create or update user
	user, err := h.servs.OIDCService.CreateOrUpdateUser(ctx, userInfo)
	if err != nil {
		logger.Error("failed to create/update OIDC user", logger.KV{
			"ip":    c.RealIP(),
			"ua":    c.Request().UserAgent(),
			"email": userInfo.Email,
			"error": err,
		})
		return c.String(http.StatusInternalServerError, "Failed to create user account")
	}

	logger.Info("OIDC authentication successful", logger.KV{
		"email":   userInfo.Email,
		"name":    userInfo.Name,
		"subject": userInfo.Subject,
		"user_id": user.ID,
	})

	// Create session for the user
	session, err := h.servs.AuthService.LoginOIDC(
		ctx, user.ID, c.RealIP(), c.Request().UserAgent(),
	)
	if err != nil {
		logger.Error("failed to create session for OIDC user", logger.KV{
			"ip":      c.RealIP(),
			"ua":      c.Request().UserAgent(),
			"user_id": user.ID,
			"error":   err,
		})
		return c.String(http.StatusInternalServerError, "Failed to create session")
	}

	// Set session cookie and redirect to dashboard
	h.servs.AuthService.SetSessionCookie(c, session.DecryptedToken)
	return c.Redirect(http.StatusSeeOther, "/dashboard")
}
