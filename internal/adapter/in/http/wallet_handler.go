package http

import (
	"net/http"
	"strconv"

	"github.com/snipkode/wertku/internal/adapter/in/http/response"
	"github.com/snipkode/wertku/internal/apperror"
	"github.com/snipkode/wertku/internal/core/domain"
	"github.com/snipkode/wertku/internal/core/port/in"
)

// WalletHandler handles wallet endpoints.
type WalletHandler struct {
	walletUC in.WalletUseCase
}

// NewWalletHandler creates a new WalletHandler.
func NewWalletHandler(walletUC in.WalletUseCase) *WalletHandler {
	return &WalletHandler{walletUC: walletUC}
}

// Create handles POST /wallets — creates a wallet for the authenticated user.
func (h *WalletHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, err := domain.UserFromContext(r.Context())
	if err != nil {
		response.Error(w, apperror.ErrUnauthenticated)
		return
	}

	resp, err := h.walletUC.Create(r.Context(), in.CreateWalletRequest{UserID: actor.ID})
	if err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, resp)
}

// GetByID handles GET /wallets/{id}
func (h *WalletHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	walletID, err := pathParamInt64(r, "id")
	if err != nil {
		response.ErrorMsg(w, http.StatusBadRequest, "invalid wallet id")
		return
	}

	resp, err := h.walletUC.GetByID(r.Context(), walletID)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// Reconcile handles GET /wallets/{id}/reconcile
func (h *WalletHandler) Reconcile(w http.ResponseWriter, r *http.Request) {
	walletID, err := pathParamInt64(r, "id")
	if err != nil {
		response.ErrorMsg(w, http.StatusBadRequest, "invalid wallet id")
		return
	}

	resp, err := h.walletUC.Reconcile(r.Context(), walletID)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, resp)
}

// pathParamInt64 extracts a named path parameter and parses it as int64.
// Compatible with Go 1.22+ ServeMux path parameters ({id}).
func pathParamInt64(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}
