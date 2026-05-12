package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
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

type RegisterHTTPReq struct {
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=6"`
	DisplayName string `json:"display_name"`
}

type LoginHTTPReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
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
	response.OK(c, result)
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
	response.OK(c, result)
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
	default:
		response.Error(c, http.StatusInternalServerError, "internal_server_error", "internal server error")
	}
}
