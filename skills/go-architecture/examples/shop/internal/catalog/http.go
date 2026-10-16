package catalog

import (
	"errors"
	"log/slog"
	"net/http"

	"example.com/shop/internal/platform/web"
)

// Handler exposes the catalog over HTTP.
type Handler struct {
	svc    *Service
	logger *slog.Logger
}

// NewHandler returns a Handler for svc.
func NewHandler(svc *Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

// Routes registers the catalog's endpoints on mux.
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /products", h.create)
	mux.HandleFunc("GET /products/{sku}", h.get)
}

// productJSON is the wire format. It is separate from Product so the API
// contract can stay stable while the domain type evolves.
type productJSON struct {
	SKU        string `json:"sku"`
	Name       string `json:"name"`
	PriceCents int64  `json:"price_cents"`
}

func toJSON(p Product) productJSON {
	return productJSON{SKU: p.SKU, Name: p.Name, PriceCents: p.PriceCents}
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req productJSON
	if !web.Decode(w, r, &req) {
		return
	}
	p, err := h.svc.Add(r.Context(), Product{SKU: req.SKU, Name: req.Name, PriceCents: req.PriceCents})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	web.JSON(w, http.StatusCreated, toJSON(p))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.Get(r.Context(), r.PathValue("sku"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	web.JSON(w, http.StatusOK, toJSON(p))
}

// fail maps the catalog's own errors to statuses; everything else falls
// through to the shared internal-error response.
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidProduct):
		web.Problem(w, http.StatusUnprocessableEntity, "catalog.invalid_product", err.Error())
	case errors.Is(err, ErrNotFound):
		web.Problem(w, http.StatusNotFound, "catalog.not_found", "product not found")
	case errors.Is(err, ErrDuplicateSKU):
		web.Problem(w, http.StatusConflict, "catalog.duplicate_sku", "product already exists")
	default:
		web.InternalError(w, r, h.logger, err)
	}
}
