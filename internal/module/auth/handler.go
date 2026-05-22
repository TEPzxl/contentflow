package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tepzxl/contentflow/internal/http/requestctx"
	"github.com/tepzxl/contentflow/internal/http/response"
)

type Handler struct {
	service Service
}

const refreshCookieName = "contentflow_refresh_token"
const refreshCookieMaxAge = 7 * 24 * 60 * 60

func NewHandler(service Service) *Handler {
	return &Handler{
		service: service,
	}
}

type authUserHTTPResp struct {
	ID          int64  `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

type RegisterHTTPReq struct {
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=6"`
	DisplayName string `json:"display_name"`
}

type registerHTTPResp struct {
	User authUserHTTPResp `json:"user"`
}

type LoginHTTPReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type loginHTTPResp struct {
	AccessToken  string           `json:"access_token"`
	RefreshToken string           `json:"refresh_token,omitempty"`
	TokenType    string           `json:"token_type"`
	ExpiresIn    int64            `json:"expires_in"`
	User         authUserHTTPResp `json:"user"`
}

type RefreshHTTPReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type refreshHTTPResp struct {
	AccessToken  string           `json:"access_token"`
	RefreshToken string           `json:"refresh_token,omitempty"`
	TokenType    string           `json:"token_type"`
	ExpiresIn    int64            `json:"expires_in"`
	User         authUserHTTPResp `json:"user"`
}

type LogoutHTTPReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type MeHTTPResp struct {
	User authUserHTTPResp `json:"user"`
}

func (h *Handler) Register(c *gin.Context) {
	var req RegisterHTTPReq

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_request", "invalid register request")
		return
	}

	result, err := h.service.Register(c.Request.Context(), RegisterRequest{
		Email:       req.Email,
		Password:    req.Password,
		DisplayName: req.DisplayName,
	})
	if err != nil {
		h.handleAuthError(c, err)
		return
	}
	response.OK(c, toRegisterHTTPResp(result))
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginHTTPReq

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_request", "invalid login request")
		return
	}

	result, err := h.service.Login(c.Request.Context(), LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		h.handleAuthError(c, err)
		return
	}
	setRefreshCookie(c, result.RefreshToken)
	response.OK(c, toLoginHTTPResp(result))
}

func (h *Handler) Refresh(c *gin.Context) {
	refreshToken, ok := refreshTokenFromRequest(c, func(req *RefreshHTTPReq) string {
		return req.RefreshToken
	})
	if !ok {
		response.Error(c, http.StatusBadRequest, "invalid_request", "invalid refresh request")
		return
	}

	result, err := h.service.Refresh(c.Request.Context(), RefreshRequest{
		RefreshToken: refreshToken,
	})
	if err != nil {
		h.handleAuthError(c, err)
		return
	}
	setRefreshCookie(c, result.RefreshToken)
	response.OK(c, toRefreshTokenHTTPResp(result))
}

func (h *Handler) Logout(c *gin.Context) {
	refreshToken, ok := refreshTokenFromRequest(c, func(req *LogoutHTTPReq) string {
		return req.RefreshToken
	})
	if !ok {
		response.Error(c, http.StatusBadRequest, "invalid_request", "invalid logout request")
		return
	}

	err := h.service.Logout(c.Request.Context(), LogoutRequest{
		RefreshToken: refreshToken,
	})
	if err != nil {
		h.handleAuthError(c, err)
		return
	}
	clearRefreshCookie(c)
	response.OK(c, gin.H{
		"message": "logged out",
	})
}

func (h *Handler) Me(c *gin.Context) {
	userID, ok := requestctx.UserID(c.Request.Context())
	if !ok {
		response.Error(c, http.StatusUnauthorized, "unauthorized", "missing user context")
		return
	}

	result, err := h.service.Me(c.Request.Context(), userID)
	if err != nil {
		h.handleAuthError(c, err)
		return
	}
	response.OK(c, toMeHTTPResp(result))
}

func (h *Handler) handleAuthError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrEmailAlreadyExists):
		response.Error(c, http.StatusConflict, "email_already_exists", "email already exists")
	case errors.Is(err, ErrInvalidCredentials):
		response.Error(c, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
	case errors.Is(err, ErrInvalidEmail):
		response.Error(c, http.StatusBadRequest, "invalid_email", "invalid email")
	case errors.Is(err, ErrWeakPassword):
		response.Error(c, http.StatusBadRequest, "weak_password", "weak password")
	case errors.Is(err, ErrInvalidRefreshToken):
		response.Error(c, http.StatusUnauthorized, "invalid_refresh_token", "invalid refresh token")
	case errors.Is(err, ErrUserNotFound):
		response.Error(c, http.StatusUnauthorized, "unauthorized", "user not found")
	default:
		response.Error(c, http.StatusInternalServerError, "internal_server_error", "internal server error")
	}
}

func refreshTokenFromRequest[T RefreshHTTPReq | LogoutHTTPReq](c *gin.Context, pick func(*T) string) (string, bool) {
	var req T
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			return "", false
		}
	}

	token := strings.TrimSpace(pick(&req))
	if token == "" {
		if cookieToken, err := c.Cookie(refreshCookieName); err == nil {
			token = strings.TrimSpace(cookieToken)
		}
	}
	return token, token != ""
}

func setRefreshCookie(c *gin.Context, token string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     "/api/v1/auth",
		MaxAge:   refreshCookieMaxAge,
		HttpOnly: true,
		Secure:   requestIsHTTPS(c),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearRefreshCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/api/v1/auth",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   requestIsHTTPS(c),
		SameSite: http.SameSiteLaxMode,
	})
}

func requestIsHTTPS(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	return strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
}

func toAuthUserHTTPResp(u AuthUser) authUserHTTPResp {
	return authUserHTTPResp{
		ID:          u.ID,
		Email:       u.Email,
		DisplayName: u.DisplayName,
	}
}

func toRegisterHTTPResp(resp *RegisterResponse) registerHTTPResp {
	return registerHTTPResp{
		User: toAuthUserHTTPResp(resp.User),
	}
}

func toLoginHTTPResp(resp *LoginResponse) loginHTTPResp {
	return loginHTTPResp{
		User:        toAuthUserHTTPResp(resp.User),
		AccessToken: resp.AccessToken,
		TokenType:   resp.TokenType,
		ExpiresIn:   resp.ExpiresIn,
	}
}

func toRefreshTokenHTTPResp(resp *RefreshResponse) refreshHTTPResp {
	return refreshHTTPResp{
		User:        toAuthUserHTTPResp(resp.User),
		AccessToken: resp.AccessToken,
		TokenType:   resp.TokenType,
		ExpiresIn:   resp.ExpiresIn,
	}
}

func toMeHTTPResp(resp *MeResponse) MeHTTPResp {
	return MeHTTPResp{
		User: toAuthUserHTTPResp(resp.User),
	}
}
