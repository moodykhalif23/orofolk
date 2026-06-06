package marketplace

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"b2bcommerce/internal/server/response"
	"b2bcommerce/internal/store/gen"
)

// ---- vendor catalog ownership (audience 'vendor') ------------------------

type vendorProductRequest struct {
	Sku         string          `json:"sku"`
	Name        string          `json:"name"`
	Slug        string          `json:"slug"`
	Description *string         `json:"description"`
	Status      string          `json:"status"`
	Unit        string          `json:"unit"`
	Attributes  json.RawMessage `json:"attributes"`
}

func (r vendorProductRequest) attrs() []byte {
	if len(r.Attributes) == 0 {
		return []byte("{}")
	}
	return r.Attributes
}

// createProduct lets a vendor list a new product. It is created owned by the
// vendor and 'pending' operator approval, so it is invisible to buyers until the
// operator approves it.
func (h *Handler) createProduct(w http.ResponseWriter, r *http.Request) {
	vc, ok := vendorOf(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no vendor claims")
		return
	}
	var req vendorProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}
	if req.Sku == "" || req.Name == "" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "sku and name are required")
		return
	}
	if req.Slug == "" {
		req.Slug = slugify(req.Name)
	}
	if req.Status == "" {
		req.Status = "draft"
	}
	if req.Unit == "" {
		req.Unit = "each"
	}
	vid := vc.vendorID
	p, err := h.q.CreateVendorProduct(r.Context(), gen.CreateVendorProductParams{
		OrganizationID: vc.orgID, Sku: req.Sku, Name: req.Name, Slug: req.Slug,
		Description: req.Description, Status: req.Status, Attributes: req.attrs(), Unit: req.Unit, VendorID: &vid,
	})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not create product (sku/slug may be in use)")
		return
	}
	response.JSON(w, http.StatusCreated, p)
}

func (h *Handler) updateProduct(w http.ResponseWriter, r *http.Request) {
	vc, ok := vendorOf(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no vendor claims")
		return
	}
	id, err := pathID(r)
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var req vendorProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}
	if req.Name == "" {
		response.Fail(w, http.StatusBadRequest, "bad_request", "name is required")
		return
	}
	if req.Status == "" {
		req.Status = "draft"
	}
	if req.Unit == "" {
		req.Unit = "each"
	}
	vid := vc.vendorID
	p, err := h.q.UpdateVendorProduct(r.Context(), gen.UpdateVendorProductParams{
		ID: id, VendorID: &vid, Name: req.Name, Description: req.Description,
		Status: req.Status, Attributes: req.attrs(), Unit: req.Unit,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.Fail(w, http.StatusNotFound, "not_found", "product not found")
			return
		}
		response.Fail(w, http.StatusInternalServerError, "internal", "could not update product")
		return
	}
	response.JSON(w, http.StatusOK, p)
}

// submitProduct re-submits a draft/rejected product for operator approval.
func (h *Handler) submitProduct(w http.ResponseWriter, r *http.Request) {
	vc, ok := vendorOf(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no vendor claims")
		return
	}
	id, err := pathID(r)
	if err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	vid := vc.vendorID
	row, err := h.q.SubmitVendorProduct(r.Context(), gen.SubmitVendorProductParams{ID: id, VendorID: &vid})
	if err != nil {
		// Either not the vendor's product, or not in a submittable state.
		response.Fail(w, http.StatusConflict, "invalid_state", "product not found or not in a draft/rejected state")
		return
	}
	response.JSON(w, http.StatusOK, row)
}

// ---- operator moderation (audience 'admin') ------------------------------

func (h *Handler) listPendingProducts(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	rows, err := h.q.ListPendingProducts(r.Context(), org)
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not list pending products")
		return
	}
	if rows == nil {
		rows = []gen.ListPendingProductsRow{}
	}
	response.JSON(w, http.StatusOK, map[string]any{"items": rows})
}

func (h *Handler) moderateProduct(status string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		org, ok := orgID(r)
		if !ok {
			response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
			return
		}
		id, err := pathID(r)
		if err != nil {
			response.Fail(w, http.StatusBadRequest, "bad_request", "invalid id")
			return
		}
		row, err := h.q.SetProductApproval(r.Context(), gen.SetProductApprovalParams{OrganizationID: org, ID: id, ApprovalStatus: status})
		if err != nil {
			// Only vendor-owned products can be moderated.
			response.Fail(w, http.StatusNotFound, "not_found", "vendor product not found")
			return
		}
		response.JSON(w, http.StatusOK, row)
	}
}
