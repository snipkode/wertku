package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/snipkode/wertku/internal/adapter/in/http/response"
	"github.com/snipkode/wertku/internal/apperror"
	"github.com/snipkode/wertku/internal/core/domain"
	"github.com/snipkode/wertku/internal/core/port/in"
)

// TransferHandler handles POST /transfers.
type TransferHandler struct {
	transferUC in.TransferUseCase
}

// NewTransferHandler creates a new TransferHandler.
func NewTransferHandler(transferUC in.TransferUseCase) *TransferHandler {
	return &TransferHandler{transferUC: transferUC}
}

// Transfer handles POST /transfers
//
// Required headers:
//   - Authorization: Bearer <token>
//   - Idempotency-Key: <unique-key>
//   - X-Request-ID: <request-id> (set by middleware if absent)
//
// Request body:
//
//	{ "from_wallet_id": 1, "to_wallet_id": 2, "amount": 30000 }
//
// Response:
//
//	{ "transaction_id": 123, "status": "SUCCESS" }
func (h *TransferHandler) Transfer(w http.ResponseWriter, r *http.Request) {
	// Extract actor from context (set by Auth middleware)
	actor, err := domain.UserFromContext(r.Context())
	if err != nil {
		response.Error(w, apperror.ErrUnauthenticated)
		return
	}

	// Idempotency-Key is required
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		response.ErrorMsg(w, http.StatusBadRequest, "Idempotency-Key header is required")
		return
	}

	var body struct {
		FromWalletID int64 `json:"from_wallet_id"`
		ToWalletID   int64 `json:"to_wallet_id"`
		Amount       int64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.ErrorMsg(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.transferUC.Transfer(r.Context(), in.TransferRequest{
		FromWalletID:   body.FromWalletID,
		ToWalletID:     body.ToWalletID,
		Amount:         body.Amount,
		IdempotencyKey: idempotencyKey,
		ActorUserID:    actor.ID,
		RequestID:      domain.RequestIDFromContext(r.Context()),
		IPAddress:      r.RemoteAddr,
		UserAgent:      r.UserAgent(),
	})
	if err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"transaction_id": resp.TransactionID,
		"status":         resp.Status,
	})
}
