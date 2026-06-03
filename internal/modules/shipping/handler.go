// Package shipping (module) exposes admin shipping-rate management, storefront
// rate quotes, and shipment label/tracking over the local table-rate provider
// (internal/shipping).
package shipping

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"b2bcommerce/internal/money"
	mw "b2bcommerce/internal/server/middleware"
	"b2bcommerce/internal/server/response"
	shipeng "b2bcommerce/internal/shipping"
	"b2bcommerce/internal/store/gen"
)

type Handler struct {
	q        *gen.Queries
	provider shipeng.Adapter
}

func New(pool *pgxpool.Pool) *Handler {
	return &Handler{q: gen.New(pool), provider: shipeng.Local{}}
}

func (h *Handler) Routes(r chi.Router, authMW func(http.Handler) http.Handler) {
	// Storefront rate quotes feed checkout (public, catalog-grade).
	r.Post("/storefront/shipping/rates", h.rates)

	r.Group(func(ar chi.Router) {
		ar.Use(authMW)
		ar.Use(mw.RequireAudience("admin"))

		ar.With(mw.RequirePermission("shipping.view")).Get("/admin/shipping-rates", h.list)
		ar.With(mw.RequirePermission("shipping.manage")).Post("/admin/shipping-rates", h.upsert)
		ar.With(mw.RequirePermission("shipping.manage")).Delete("/admin/shipping-rates/{id}", h.delete)
		ar.With(mw.RequirePermission("shipping.view")).Post("/admin/shipping/rates", h.ratesAdmin)
		ar.With(mw.RequirePermission("shipping.manage")).Post("/admin/shipments/{id}/label", h.createLabel)
		ar.With(mw.RequirePermission("shipping.view")).Get("/admin/shipments/{id}/track", h.track)
	})
}

func orgID(r *http.Request) (int64, bool) {
	c, ok := mw.ClaimsFrom(r.Context())
	if !ok {
		return 0, false
	}
	return c.OrgID, true
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	rows, err := h.q.ListShippingRates(r.Context(), org)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not list rates")
		return
	}
	if rows == nil {
		rows = []gen.ShippingRate{}
	}
	response.JSON(w, http.StatusOK, map[string]any{"items": rows})
}

func (h *Handler) upsert(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	var req struct {
		Country  string  `json:"country"`
		Service  string  `json:"service"`
		Carrier  string  `json:"carrier"`
		Amount   string  `json:"amount"`
		FreeOver *string `json:"free_over"`
		IsActive *bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Country) != 2 || req.Service == "" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "country (2-letter) and service required")
		return
	}
	if req.Carrier == "" {
		req.Carrier = "local"
	}
	if req.Amount == "" {
		req.Amount = "0"
	}
	if _, err := money.Parse(req.Amount); err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "amount must be a decimal")
		return
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	rate, err := h.q.UpsertShippingRate(r.Context(), gen.UpsertShippingRateParams{
		OrganizationID: org, Country: req.Country, Service: req.Service, Carrier: req.Carrier,
		Amount: req.Amount, FreeOver: req.FreeOver, IsActive: active,
	})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not save rate")
		return
	}
	response.JSON(w, http.StatusCreated, rate)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	if err := h.q.DeleteShippingRate(r.Context(), gen.DeleteShippingRateParams{OrganizationID: org, ID: id}); err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not delete rate")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// rates is the public storefront rate quote. Org is resolved from the request
// host's website (falls back to org 1) since checkout is unauthenticated here.
func (h *Handler) rates(w http.ResponseWriter, r *http.Request) {
	h.quote(w, r, h.resolveOrg(r))
}

func (h *Handler) ratesAdmin(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	h.quote(w, r, org)
}

func (h *Handler) quote(w http.ResponseWriter, r *http.Request, org int64) {
	var req shipeng.RateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Country) != 2 {
		response.Fail(w, http.StatusBadRequest, "bad_request", "country (2-letter) required")
		return
	}
	if req.Subtotal == "" {
		req.Subtotal = "0"
	}
	rows, err := h.q.ListShippingRatesByCountry(r.Context(), gen.ListShippingRatesByCountryParams{OrganizationID: org, Country: req.Country})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not load rates")
		return
	}
	adapterRows := make([]shipeng.RateRow, len(rows))
	for i, row := range rows {
		adapterRows[i] = shipeng.RateRow{Service: row.Service, Carrier: row.Carrier, Amount: row.Amount, FreeOver: row.FreeOver}
	}
	quotes, _ := h.provider.Rates(r.Context(), adapterRows, req)
	if quotes == nil {
		quotes = []shipeng.RateQuote{}
	}
	response.JSON(w, http.StatusOK, map[string]any{"country": req.Country, "quotes": quotes})
}

// resolveOrg maps the request host to a website's org (mirrors catalog), default 1.
func (h *Handler) resolveOrg(r *http.Request) int64 {
	host := r.Host
	if i := indexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	if ws, err := h.q.GetWebsiteByDomain(r.Context(), host); err == nil {
		return ws.OrganizationID
	}
	return 1
}

func (h *Handler) createLabel(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	sh, err := h.q.GetShipmentWithOrg(r.Context(), gen.GetShipmentWithOrgParams{OrganizationID: org, ID: id})
	if err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "shipment not found")
		return
	}
	var req struct {
		Service string `json:"service"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	label, err := h.provider.CreateLabel(r.Context(), shipeng.LabelRequest{
		ShipmentRef: sh.PublicID.String(), Service: req.Service, Country: countryOf(sh.ShippingAddress),
	})
	if err != nil {
		response.Fail(w, http.StatusBadGateway, "carrier_error", "could not create label")
		return
	}
	carrier := label.Carrier
	if _, err := h.q.SetShipmentTracking(r.Context(), gen.SetShipmentTrackingParams{ID: id, TrackingNumber: &label.TrackingNumber, Carrier: &carrier}); err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not save tracking")
		return
	}
	response.JSON(w, http.StatusCreated, label)
}

func (h *Handler) track(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	sh, err := h.q.GetShipmentWithOrg(r.Context(), gen.GetShipmentWithOrgParams{OrganizationID: org, ID: id})
	if err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "shipment not found")
		return
	}
	if sh.TrackingNumber == nil || *sh.TrackingNumber == "" {
		response.Fail(w, http.StatusUnprocessableEntity, "no_label", "shipment has no label/tracking yet")
		return
	}
	status, err := h.provider.Track(r.Context(), *sh.TrackingNumber)
	if err != nil {
		response.Fail(w, http.StatusBadGateway, "carrier_error", "could not track")
		return
	}
	response.JSON(w, http.StatusOK, status)
}

func countryOf(addr []byte) string {
	if len(addr) == 0 {
		return ""
	}
	var a struct {
		Country string `json:"country"`
	}
	_ = json.Unmarshal(addr, &a)
	return a.Country
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
