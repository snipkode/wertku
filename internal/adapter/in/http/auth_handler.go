package http

import (
	"encoding/json"
	"net/http"

	"github.com/snipkode/wertku/internal/adapter/in/http/response"
	"github.com/snipkode/wertku/internal/apperror"
	"github.com/snipkode/wertku/internal/core/domain"
	"github.com/snipkode/wertku/internal/core/port/in"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	authUC in.AuthUseCase
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(authUC in.AuthUseCase) *AuthHandler {
	return &AuthHandler{authUC: authUC}
}

// Register handles POST /auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorMsg(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.authUC.Register(r.Context(), in.RegisterRequest{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, map[string]any{
		"user_id": resp.UserID,
		"uid":     resp.UID,
		"email":   resp.Email,
	})
}

// Login handles POST /auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorMsg(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.authUC.Login(r.Context(), in.LoginRequest{
		Email:     req.Email,
		Password:  req.Password,
		IPAddress: r.RemoteAddr,
		UserAgent: r.UserAgent(),
		RequestID: domain.RequestIDFromContext(r.Context()),
	})
	if err != nil {
		response.Error(w, err)
		return
	}

	// Return token — client stores it, we never log it
	response.JSON(w, http.StatusOK, map[string]any{
		"token":   resp.Token,
		"user_id": resp.UserID,
		"uid":     resp.UID,
	})
}

// Logout handles POST /auth/logout — requires Auth middleware
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	user, err := domain.UserFromContext(r.Context())
	if err != nil {
		response.Error(w, apperror.ErrUnauthenticated)
		return
	}

	if err := h.authUC.Logout(r.Context(), user.ID, domain.RequestIDFromContext(r.Context())); err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{"message": "logged out"})
}
