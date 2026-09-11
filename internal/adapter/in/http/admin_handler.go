package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/snipkode/wertku/internal/adapter/in/http/response"
	"github.com/snipkode/wertku/internal/apperror"
	"github.com/snipkode/wertku/internal/core/domain"
	"github.com/snipkode/wertku/internal/core/port/in"
)

// AdminHandler handles administrative endpoints.
type AdminHandler struct {
	adminUC in.AdminUseCase
	roleUC  in.RoleUseCase
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(adminUC in.AdminUseCase, roleUC in.RoleUseCase) *AdminHandler {
	return &AdminHandler{adminUC: adminUC, roleUC: roleUC}
}

// ListUsers handles GET /admin/users — requires user:read
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)
	users, err := h.adminUC.ListUsers(r.Context(), limit, offset)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, users)
}

// ListTransactions handles GET /admin/transactions — requires transaction:read
func (h *AdminHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)
	txs, err := h.adminUC.ListTransactions(r.Context(), limit, offset)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, txs)
}

// ListAuditLogs handles GET /admin/audit-logs — requires audit:read
func (h *AdminHandler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)
	logs, err := h.adminUC.ListAuditLogs(r.Context(), limit, offset)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, logs)
}

// AssignRole handles POST /admin/users/{id}/roles — requires role:assign
func (h *AdminHandler) AssignRole(w http.ResponseWriter, r *http.Request) {
	actor, err := domain.UserFromContext(r.Context())
	if err != nil {
		response.Error(w, apperror.ErrUnauthenticated)
		return
	}

	targetUserID, err := pathParamInt64(r, "id")
	if err != nil {
		response.ErrorMsg(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var body struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.ErrorMsg(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.roleUC.AssignRole(r.Context(), actor.ID, targetUserID, body.Role); err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"message":        "role assigned",
		"target_user_id": targetUserID,
		"role":           body.Role,
	})
}

// RemoveRole handles DELETE /admin/users/{id}/roles/{role} — requires role:assign
func (h *AdminHandler) RemoveRole(w http.ResponseWriter, r *http.Request) {
	actor, err := domain.UserFromContext(r.Context())
	if err != nil {
		response.Error(w, apperror.ErrUnauthenticated)
		return
	}

	targetUserID, err := pathParamInt64(r, "id")
	if err != nil {
		response.ErrorMsg(w, http.StatusBadRequest, "invalid user id")
		return
	}

	roleName := r.PathValue("role")
	if roleName == "" {
		response.ErrorMsg(w, http.StatusBadRequest, "role name is required")
		return
	}

	if err := h.roleUC.RemoveRole(r.Context(), actor.ID, targetUserID, roleName); err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"message":        "role removed",
		"target_user_id": targetUserID,
		"role":           roleName,
	})
}

// pagination extracts limit and offset query parameters with safe defaults.
func pagination(r *http.Request) (limit, offset int) {
	limit = 20
	offset = 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return
}
