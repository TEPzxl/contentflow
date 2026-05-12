package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tepzxl/contentflow/internal/http/requestctx"
	"github.com/tepzxl/contentflow/internal/http/response"
)

type Handler struct {
	service Service
}

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
	RefreshToken string           `json:"refresh_token"`
	TokenType    string           `json:"token_type"`
	ExpiresIn    int64            `json:"expires_in"`
	User         authUserHTTPResp `json:"user"`
}

type RefreshHTTPReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type refreshHTTPResp struct {
	AccessToken  string           `json:"access_token"`
	RefreshToken string           `json:"refresh_token"`
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
	response.OK(c, toLoginHTTPResp(result))
}

func (h *Handler) Refresh(c *gin.Context) {
	var req RefreshHTTPReq

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_request", "invalid refresh request")
		return
	}

	result, err := h.service.Refresh(c.Request.Context(), RefreshRequest{
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		h.handleAuthError(c, err)
		return
	}
	response.OK(c, toRefreshTokenHTTPResp(result))
}

func (h *Handler) Logout(c *gin.Context) {
	var req LogoutHTTPReq

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_request", "invalid logout request")
		return
	}

	err := h.service.Logout(c.Request.Context(), LogoutRequest{
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		h.handleAuthError(c, err)
		return
	}
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
		User:         toAuthUserHTTPResp(resp.User),
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		TokenType:    resp.TokenType,
		ExpiresIn:    resp.ExpiresIn,
	}
}

func toRefreshTokenHTTPResp(resp *RefreshResponse) refreshHTTPResp {
	return refreshHTTPResp{
		User:         toAuthUserHTTPResp(resp.User),
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		TokenType:    resp.TokenType,
		ExpiresIn:    resp.ExpiresIn,
	}
}

func toMeHTTPResp(resp *MeResponse) MeHTTPResp {
	return MeHTTPResp{
		User: toAuthUserHTTPResp(resp.User),
	}
}
