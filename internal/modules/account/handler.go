// Package account implements storefront-facing buyer self-service for the
// authenticated buying company: saved addresses (this slice), and — as later
// slices land — company users and order approvals. Every route is scoped to the
// customer-user's company from the storefront JWT (never the request body), so
// a buyer can only ever read or mutate their own company's data.
package account

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	mw "b2bcommerce/internal/server/middleware"
	"b2bcommerce/internal/server/response"
	"b2bcommerce/internal/store/gen"
)

type Handler struct {
	q *gen.Queries
}

func New(q *gen.Queries) *Handler { return &Handler{q: q} }

func (h *Handler) Routes(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(sr chi.Router) {
		sr.Use(authMW)
		sr.Use(mw.RequireAudience("storefront"))

		sr.Get("/storefront/account/addresses", h.listAddresses)
		sr.Post("/storefront/account/addresses", h.createAddress)
	})
}

// principal is the authenticated customer-user context.
type principal struct {
	orgID          int64
	customerID     int64
	customerUserID *int64
}

func actor(r *http.Request) (principal, bool) {
	c, ok := mw.ClaimsFrom(r.Context())
	if !ok || c.CustomerID == 0 {
		return principal{}, false
	}
	p := principal{orgID: c.OrgID, customerID: c.CustomerID}
	if id, err := strconv.ParseInt(c.Subject, 10, 64); err == nil && id != 0 {
		p.customerUserID = &id
	}
	return p, true
}

// ---- Addresses -----------------------------------------------------------

func (h *Handler) listAddresses(w http.ResponseWriter, r *http.Request) {
	p, ok := actor(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no customer context")
		return
	}
	rows, err := h.q.ListCustomerAddresses(r.Context(), p.customerID)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not list addresses")
		return
	}
	if rows == nil {
		rows = []gen.CustomerAddress{}
	}
	response.JSON(w, http.StatusOK, map[string]any{"items": rows})
}

func (h *Handler) createAddress(w http.ResponseWriter, r *http.Request) {
	p, ok := actor(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no customer context")
		return
	}
	var req struct {
		Type       string  `json:"type"`
		IsDefault  bool    `json:"is_default"`
		Line1      string  `json:"line1"`
		Line2      *string `json:"line2"`
		City       string  `json:"city"`
		Region     *string `json:"region"`
		PostalCode *string `json:"postal_code"`
		Country    string  `json:"country"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}
	if req.Line1 == "" || req.City == "" || len(req.Country) != 2 {
		response.Fail(w, http.StatusBadRequest, "bad_request", "line1, city, 2-letter country required")
		return
	}
	if req.Type != "billing" && req.Type != "shipping" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "type must be billing or shipping")
		return
	}
	a, err := h.q.CreateCustomerAddress(r.Context(), gen.CreateCustomerAddressParams{
		CustomerID: p.customerID,
		Type:       req.Type,
		IsDefault:  req.IsDefault,
		Line1:      req.Line1,
		Line2:      req.Line2,
		City:       req.City,
		Region:     req.Region,
		PostalCode: req.PostalCode,
		Country:    req.Country,
	})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not create address")
		return
	}
	response.JSON(w, http.StatusCreated, a)
}
