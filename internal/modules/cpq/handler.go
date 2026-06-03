// Package cpq exposes the Configure-Price-Quote module (PRD §8): admins define a
// configurable product's option groups, options, and rules; the storefront
// fetches that definition and prices a chosen configuration; and a validated
// configuration becomes a priced quote line. The validation/pricing logic lives
// in internal/cpq.
package cpq

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	cpqeng "b2bcommerce/internal/cpq"
	"b2bcommerce/internal/money"
	mw "b2bcommerce/internal/server/middleware"
	"b2bcommerce/internal/server/response"
	"b2bcommerce/internal/store/gen"
)

type Handler struct {
	pool *pgxpool.Pool
	q    *gen.Queries
}

func New(pool *pgxpool.Pool) *Handler { return &Handler{pool: pool, q: gen.New(pool)} }

func (h *Handler) Routes(r chi.Router, authMW func(http.Handler) http.Handler) {
	// Public configurator (catalog-grade reads).
	r.Get("/storefront/products/{publicID}/config", h.storefrontConfig)
	r.Post("/storefront/products/{publicID}/configure", h.storefrontConfigure)

	r.Group(func(ar chi.Router) {
		ar.Use(authMW)
		ar.Use(mw.RequireAudience("admin"))

		ar.With(mw.RequirePermission("product.view")).Get("/admin/products/{id}/config", h.getConfig)
		ar.With(mw.RequirePermission("product.manage")).Put("/admin/products/{id}/config", h.upsertConfig)
		ar.With(mw.RequirePermission("product.manage")).Post("/admin/products/{id}/option-groups", h.createGroup)
		ar.With(mw.RequirePermission("product.manage")).Post("/admin/option-groups/{gid}/options", h.createOption)
		ar.With(mw.RequirePermission("product.manage")).Post("/admin/products/{id}/config-rules", h.createRule)

		ar.With(mw.RequirePermission("quote.manage")).Post("/admin/quotes/{id}/configured-lines", h.addConfiguredLine)
	})
}

func orgID(r *http.Request) (int64, bool) {
	c, ok := mw.ClaimsFrom(r.Context())
	if !ok {
		return 0, false
	}
	return c.OrgID, true
}

func pathInt(r *http.Request, key string) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, key), 10, 64)
}

// loadDefinition assembles the full cpq.Definition for a product from its
// config, groups, options, and rules. Returns ok=false when the product has no
// active configuration.
func (h *Handler) loadDefinition(r *http.Request, productID int64) (cpqeng.Definition, bool) {
	cfg, err := h.q.GetProductConfig(r.Context(), productID)
	if err != nil || !cfg.IsActive {
		return cpqeng.Definition{}, false
	}
	groups, _ := h.q.ListOptionGroups(r.Context(), productID)
	opts, _ := h.q.ListOptionsForProduct(r.Context(), productID)
	rules, _ := h.q.ListConfigRules(r.Context(), productID)

	byGroup := map[int64][]cpqeng.Option{}
	for _, o := range opts {
		byGroup[o.GroupID] = append(byGroup[o.GroupID], cpqeng.Option{
			ID: o.ID, GroupID: o.GroupID, Code: o.Code, Name: o.Name,
			PriceDelta: o.PriceDelta, IsDefault: o.IsDefault,
		})
	}
	def := cpqeng.Definition{ProductID: productID, BasePrice: cfg.BasePrice, Currency: cfg.Currency}
	for _, g := range groups {
		def.Groups = append(def.Groups, cpqeng.Group{
			ID: g.ID, Code: g.Code, Name: g.Name, Required: g.Required,
			MinSelect: int(g.MinSelect), MaxSelect: int(g.MaxSelect), Options: byGroup[g.ID],
		})
	}
	for _, rl := range rules {
		def.Rules = append(def.Rules, cpqeng.Rule{Kind: rl.Kind, OptionID: rl.OptionID, RelatedOptionID: rl.RelatedOptionID})
	}
	return def, true
}

// ---- admin: configuration management --------------------------------------

func (h *Handler) upsertConfig(w http.ResponseWriter, r *http.Request) {
	org, id, ok := h.ownedProduct(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		BasePrice string `json:"base_price"`
		Currency  string `json:"currency"`
		IsActive  *bool  `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.BasePrice == "" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "base_price is required")
		return
	}
	if _, err := money.Parse(req.BasePrice); err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "base_price must be a decimal")
		return
	}
	if len(req.Currency) != 3 {
		req.Currency = "USD"
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	cfg, err := h.q.UpsertProductConfig(r.Context(), gen.UpsertProductConfigParams{
		ProductID: id, BasePrice: req.BasePrice, Currency: req.Currency, IsActive: active,
	})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not save config")
		return
	}
	_ = org
	response.JSON(w, http.StatusOK, map[string]any{
		"product_id": cfg.ProductID, "base_price": cfg.BasePrice, "currency": cfg.Currency, "is_active": cfg.IsActive,
	})
}

func (h *Handler) getConfig(w http.ResponseWriter, r *http.Request) {
	_, id, ok := h.ownedProduct(w, r, "id")
	if !ok {
		return
	}
	def, ok := h.loadDefinition(r, id)
	if !ok {
		response.Fail(w, http.StatusNotFound, "not_found", "product is not configurable")
		return
	}
	response.JSON(w, http.StatusOK, def)
}

func (h *Handler) createGroup(w http.ResponseWriter, r *http.Request) {
	_, id, ok := h.ownedProduct(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Code      string `json:"code"`
		Name      string `json:"name"`
		Required  *bool  `json:"required"`
		MinSelect int32  `json:"min_select"`
		MaxSelect int32  `json:"max_select"`
		SortOrder int32  `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" || req.Name == "" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "code and name are required")
		return
	}
	required := true
	if req.Required != nil {
		required = *req.Required
	}
	if req.MaxSelect == 0 {
		req.MaxSelect = 1
	}
	g, err := h.q.CreateOptionGroup(r.Context(), gen.CreateOptionGroupParams{
		ProductID: id, Code: req.Code, Name: req.Name, Required: required,
		MinSelect: req.MinSelect, MaxSelect: req.MaxSelect, SortOrder: req.SortOrder,
	})
	if err != nil {
		response.Fail(w, http.StatusConflict, "conflict", "group code may already exist")
		return
	}
	response.JSON(w, http.StatusCreated, g)
}

func (h *Handler) createOption(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	gid, err := pathInt(r, "gid")
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid group id")
		return
	}
	// Authorize: the group's product must belong to the caller's org.
	grp, err := h.q.GetOptionGroup(r.Context(), gid)
	if err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "option group not found")
		return
	}
	if _, err := h.q.GetCpqProduct(r.Context(), gen.GetCpqProductParams{OrganizationID: org, ID: grp.ProductID}); err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "option group not found")
		return
	}
	var req struct {
		Code       string `json:"code"`
		Name       string `json:"name"`
		PriceDelta string `json:"price_delta"`
		IsDefault  bool   `json:"is_default"`
		SortOrder  int32  `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" || req.Name == "" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "code and name are required")
		return
	}
	if req.PriceDelta == "" {
		req.PriceDelta = "0"
	}
	if _, err := money.Parse(req.PriceDelta); err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "price_delta must be a decimal")
		return
	}
	o, err := h.q.CreateOption(r.Context(), gen.CreateOptionParams{
		GroupID: gid, Code: req.Code, Name: req.Name, PriceDelta: req.PriceDelta,
		IsDefault: req.IsDefault, SortOrder: req.SortOrder,
	})
	if err != nil {
		response.Fail(w, http.StatusConflict, "conflict", "option code may already exist")
		return
	}
	response.JSON(w, http.StatusCreated, o)
}

func (h *Handler) createRule(w http.ResponseWriter, r *http.Request) {
	_, id, ok := h.ownedProduct(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Kind            string `json:"kind"`
		OptionID        int64  `json:"option_id"`
		RelatedOptionID int64  `json:"related_option_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OptionID == 0 || req.RelatedOptionID == 0 {
		response.Fail(w, http.StatusBadRequest, "bad_request", "option_id and related_option_id are required")
		return
	}
	if req.Kind != "requires" && req.Kind != "excludes" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "kind must be requires or excludes")
		return
	}
	rule, err := h.q.CreateConfigRule(r.Context(), gen.CreateConfigRuleParams{
		ProductID: id, Kind: req.Kind, OptionID: req.OptionID, RelatedOptionID: req.RelatedOptionID,
	})
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "could not create rule (options must exist)")
		return
	}
	response.JSON(w, http.StatusCreated, rule)
}

// ownedProduct resolves {id} to a product in the caller's org.
func (h *Handler) ownedProduct(w http.ResponseWriter, r *http.Request, key string) (org, id int64, ok bool) {
	org, ok = orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return 0, 0, false
	}
	id, err := pathInt(r, key)
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid id")
		return 0, 0, false
	}
	if _, err := h.q.GetCpqProduct(r.Context(), gen.GetCpqProductParams{OrganizationID: org, ID: id}); err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "product not found")
		return 0, 0, false
	}
	return org, id, true
}

// ---- storefront: configurator ---------------------------------------------

func (h *Handler) storefrontProductID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	pid, err := uuid.Parse(chi.URLParam(r, "publicID"))
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid id")
		return 0, false
	}
	id, err := h.q.GetProductIDByPublicIDGlobal(r.Context(), pid)
	if err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "product not found")
		return 0, false
	}
	return id, true
}

func (h *Handler) storefrontConfig(w http.ResponseWriter, r *http.Request) {
	id, ok := h.storefrontProductID(w, r)
	if !ok {
		return
	}
	def, ok := h.loadDefinition(r, id)
	if !ok {
		response.Fail(w, http.StatusNotFound, "not_found", "product is not configurable")
		return
	}
	response.JSON(w, http.StatusOK, def)
}

func (h *Handler) storefrontConfigure(w http.ResponseWriter, r *http.Request) {
	id, ok := h.storefrontProductID(w, r)
	if !ok {
		return
	}
	def, ok := h.loadDefinition(r, id)
	if !ok {
		response.Fail(w, http.StatusNotFound, "not_found", "product is not configurable")
		return
	}
	var req struct {
		Selections []int64 `json:"selections"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	// Always 200 with the Result so a live configurator can show errors inline.
	response.JSON(w, http.StatusOK, cpqeng.Configure(def, req.Selections))
}

// ---- quote integration -----------------------------------------------------

func (h *Handler) addConfiguredLine(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	quoteID, err := pathInt(r, "id")
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid quote id")
		return
	}
	quote, err := h.q.GetCpqQuote(r.Context(), gen.GetCpqQuoteParams{OrganizationID: org, ID: quoteID})
	if err != nil {
		response.Fail(w, http.StatusNotFound, "not_found", "quote not found")
		return
	}
	var req struct {
		ProductID  int64   `json:"product_id"`
		Quantity   string  `json:"quantity"`
		Selections []int64 `json:"selections"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ProductID == 0 {
		response.Fail(w, http.StatusBadRequest, "bad_request", "product_id is required")
		return
	}
	if req.Quantity == "" {
		req.Quantity = "1"
	}
	def, ok := h.loadDefinition(r, req.ProductID)
	if !ok {
		response.Fail(w, http.StatusUnprocessableEntity, "not_configurable", "product is not configurable")
		return
	}

	result := cpqeng.Configure(def, req.Selections)
	if !result.Valid {
		response.JSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "invalid_configuration", "errors": result.Errors})
		return
	}
	rowTotal, err := money.LineTotal(req.Quantity, result.UnitPrice)
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid quantity")
		return
	}
	configJSON, _ := json.Marshal(result)

	item, err := h.q.AddConfiguredQuoteItem(r.Context(), gen.AddConfiguredQuoteItemParams{
		QuoteID: quoteID, ProductID: req.ProductID, Quantity: req.Quantity, Unit: "each",
		UnitPrice: result.UnitPrice, RowTotal: rowTotal, Configuration: configJSON,
	})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not add line")
		return
	}
	// Recompute the quote subtotal from its lines.
	subtotal, err := h.q.SumQuoteItems(r.Context(), quoteID)
	if err == nil {
		_ = h.q.SetQuoteSubtotal(r.Context(), gen.SetQuoteSubtotalParams{ID: quoteID, Subtotal: subtotal})
	}
	_ = quote
	response.JSON(w, http.StatusCreated, map[string]any{
		"item": map[string]any{
			"id": item.ID, "product_id": item.ProductID, "quantity": item.Quantity,
			"unit_price": item.UnitPrice, "row_total": item.RowTotal,
			"configuration": json.RawMessage(item.Configuration),
		},
		"quote_subtotal": subtotal,
	})
}
