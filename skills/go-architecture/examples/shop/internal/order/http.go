package order

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"example.com/shop/internal/platform/web"
)

// Handler exposes orders over HTTP.
type Handler struct {
	svc    *Service
	logger *slog.Logger
}

// NewHandler returns a Handler for svc.
func NewHandler(svc *Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

// Routes registers the order endpoints on mux.
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /orders", h.place)
	mux.HandleFunc("GET /orders/{id}", h.get)
	mux.HandleFunc("POST /orders/{id}/pay", h.pay)
}

// Wire formats, kept separate from domain types.
type (
	placeRequest struct {
		Lines []struct {
			SKU      string `json:"sku"`
			Quantity int    `json:"quantity"`
		} `json:"lines"`
	}
	lineResponse struct {
		SKU            string `json:"sku"`
		Quantity       int    `json:"quantity"`
		UnitPriceCents int64  `json:"unit_price_cents"`
	}
	orderResponse struct {
		ID         string         `json:"id"`
		Status     string         `json:"status"`
		PaymentID  string         `json:"payment_id,omitempty"`
		Lines      []lineResponse `json:"lines"`
		TotalCents int64          `json:"total_cents"`
		CreatedAt  time.Time      `json:"created_at"`
	}
)

func toResponse(o Order) orderResponse {
	lines := make([]lineResponse, len(o.Lines))
	for i, l := range o.Lines {
		lines[i] = lineResponse{SKU: l.SKU, Quantity: l.Quantity, UnitPriceCents: l.UnitPriceCents}
	}
	return orderResponse{
		ID:         o.ID,
		Status:     string(o.Status),
		PaymentID:  o.PaymentID,
		Lines:      lines,
		TotalCents: o.TotalCents(),
		CreatedAt:  o.CreatedAt,
	}
}

// maxRequestKeyLen bounds the client's Idempotency-Key.
const maxRequestKeyLen = 255

// place creates an order. Clients that may retry (any client of an
// endpoint that moves money) send an Idempotency-Key header. A retry
// with the same key and body replays the original outcome without
// charging again: 201 with the order, or 409 if it is still awaiting
// payment. The same key with a different body is rejected with 422.
func (h *Handler) place(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("Idempotency-Key")
	if len(key) > maxRequestKeyLen {
		web.Problem(w, http.StatusBadRequest, "request.bad_idempotency_key", "Idempotency-Key is too long")
		return
	}
	var req placeRequest
	if !web.Decode(w, r, &req) {
		return
	}
	lines := make([]LineRequest, len(req.Lines))
	for i, l := range req.Lines {
		lines[i] = LineRequest{SKU: l.SKU, Quantity: l.Quantity}
	}

	o, err := h.svc.Place(r.Context(), key, lines)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Location", "/orders/"+o.ID)
	web.JSON(w, http.StatusCreated, toResponse(o))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	o, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	web.JSON(w, http.StatusOK, toResponse(o))
}

func (h *Handler) pay(w http.ResponseWriter, r *http.Request) {
	o, err := h.svc.Pay(r.Context(), r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	web.JSON(w, http.StatusOK, toResponse(o))
}

// fail maps order's own errors to statuses; everything else falls
// through to the shared internal-error response, which logs it and
// reveals nothing to the client.
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	var (
		verr *ValidationError
		perr *PaymentError
		aerr *AwaitingPaymentError
	)
	switch {
	case errors.As(err, &verr):
		web.Problem(w, http.StatusUnprocessableEntity, "order.invalid", verr.Error())
	case errors.Is(err, ErrUnknownProduct):
		web.Problem(w, http.StatusUnprocessableEntity, "order.unknown_product", err.Error())
	case errors.Is(err, ErrNotFound):
		web.Problem(w, http.StatusNotFound, "order.not_found", "order not found")
	case errors.Is(err, ErrRequestKeyReused):
		web.Problem(w, http.StatusUnprocessableEntity, "request.idempotency_key_reused",
			"this Idempotency-Key was already used for a different request")
	case errors.As(err, &aerr):
		w.Header().Set("Location", "/orders/"+aerr.OrderID)
		web.Problem(w, http.StatusConflict, "order.awaiting_payment",
			"order "+aerr.OrderID+" exists but is not paid; pay it with POST /orders/"+aerr.OrderID+"/pay")
	case errors.As(err, &perr):
		// The provider's error may hold details clients must not see; log
		// it and tell the client only which order is pending and how to
		// retry.
		h.logger.WarnContext(r.Context(), "payment did not go through", "order_id", perr.OrderID, "err", err)
		w.Header().Set("Location", "/orders/"+perr.OrderID)
		if perr.Declined {
			web.Problem(w, http.StatusPaymentRequired, "order.payment_declined",
				"payment declined; order "+perr.OrderID+" is pending")
			return
		}
		web.Problem(w, http.StatusBadGateway, "order.payment_failed",
			"payment provider unavailable; order "+perr.OrderID+" is pending, retry with POST /orders/"+perr.OrderID+"/pay")
	default:
		web.InternalError(w, r, h.logger, err)
	}
}
