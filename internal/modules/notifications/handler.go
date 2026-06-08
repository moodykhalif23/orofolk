// Package notifications serves the in-app notification feed for all three
// portals (admin, storefront, vendor) and the Pusher private-channel auth used
// for real-time delivery. The feed is always scoped to the authenticated user
// taken from the JWT subject — never the request — so a caller can only read or
// mark their own notifications, and can only subscribe to their own channel.
package notifications

import (
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"b2bcommerce/internal/notify"
	mw "b2bcommerce/internal/server/middleware"
	"b2bcommerce/internal/server/response"
	"b2bcommerce/internal/store/gen"
)

type Handler struct {
	q       *gen.Queries
	authz   notify.Authorizer
	pubKey  string
	cluster string
}

// New builds the notifications handler. authz signs Pusher subscriptions (a
// no-op authorizer means real-time is off); pubKey/cluster are the public
// Pusher coordinates handed to the browser to bootstrap a connection.
func New(pool *pgxpool.Pool, authz notify.Authorizer, pubKey, cluster string) *Handler {
	if authz == nil {
		authz = notify.NoopPublisher{}
	}
	return &Handler{q: gen.New(pool), authz: authz, pubKey: pubKey, cluster: cluster}
}

// Routes mounts the same feed endpoints under each portal prefix, each gated to
// its own token audience.
func (h *Handler) Routes(r chi.Router, authMW func(http.Handler) http.Handler) {
	for _, aud := range []string{"admin", "storefront", "vendor"} {
		aud := aud
		r.Group(func(sr chi.Router) {
			sr.Use(authMW)
			sr.Use(mw.RequireAudience(aud))

			base := "/" + aud + "/notifications"
			sr.Get(base, h.list(aud))
			sr.Get(base+"/unread-count", h.unreadCount(aud))
			sr.Post(base+"/read-all", h.readAll(aud))
			sr.Post(base+"/{publicID}/read", h.markRead(aud))
			sr.Post(base+"/pusher-auth", h.pusherAuth(aud))
		})
	}
}

// recipient resolves (orgID, recipientID) from the JWT. The recipient is the
// token subject: a users.id (admin), customer_users.id (storefront) or
// vendor_users.id (vendor).
func recipient(r *http.Request) (orgID, recipientID int64, ok bool) {
	c, found := mw.ClaimsFrom(r.Context())
	if !found {
		return 0, 0, false
	}
	id, err := strconv.ParseInt(c.Subject, 10, 64)
	if err != nil || id == 0 {
		return 0, 0, false
	}
	return c.OrgID, id, true
}

func (h *Handler) list(aud string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, rid, ok := recipient(r)
		if !ok {
			response.Fail(w, http.StatusUnauthorized, "unauthorized", "no user context")
			return
		}
		limit := clampInt(r.URL.Query().Get("limit"), 20, 1, 50)
		offset := clampInt(r.URL.Query().Get("offset"), 0, 0, 1_000_000)

		rows, err := h.q.ListNotifications(r.Context(), gen.ListNotificationsParams{
			OrganizationID: orgID, Audience: aud, RecipientID: rid,
			Limit: int32(limit), Offset: int32(offset),
		})
		if err != nil {
			response.Fail(w, http.StatusInternalServerError, "internal", "could not load notifications")
			return
		}
		unread, err := h.q.CountUnreadNotifications(r.Context(), gen.CountUnreadNotificationsParams{
			OrganizationID: orgID, Audience: aud, RecipientID: rid,
		})
		if err != nil {
			response.Fail(w, http.StatusInternalServerError, "internal", "could not count notifications")
			return
		}
		items := make([]notify.DTO, 0, len(rows))
		for _, n := range rows {
			items = append(items, notify.ToDTO(n))
		}
		response.JSON(w, http.StatusOK, map[string]any{
			"items":        items,
			"unread_count": unread,
			"realtime": map[string]any{
				"enabled": h.authz.Enabled(),
				"key":     h.pubKey,
				"cluster": h.cluster,
				"channel": notify.ChannelName(aud, orgID, rid),
			},
		})
	}
}

func (h *Handler) unreadCount(aud string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, rid, ok := recipient(r)
		if !ok {
			response.Fail(w, http.StatusUnauthorized, "unauthorized", "no user context")
			return
		}
		unread, err := h.q.CountUnreadNotifications(r.Context(), gen.CountUnreadNotificationsParams{
			OrganizationID: orgID, Audience: aud, RecipientID: rid,
		})
		if err != nil {
			response.Fail(w, http.StatusInternalServerError, "internal", "could not count notifications")
			return
		}
		response.JSON(w, http.StatusOK, map[string]any{"unread_count": unread})
	}
}

func (h *Handler) markRead(aud string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, rid, ok := recipient(r)
		if !ok {
			response.Fail(w, http.StatusUnauthorized, "unauthorized", "no user context")
			return
		}
		pid, err := uuid.Parse(chi.URLParam(r, "publicID"))
		if err != nil {
			response.Fail(w, http.StatusBadRequest, "bad_request", "invalid id")
			return
		}
		// Idempotent: marking an already-read (or non-existent-for-you) row is a
		// no-op, not an error.
		if _, err := h.q.MarkNotificationRead(r.Context(), gen.MarkNotificationReadParams{
			PublicID: pid, OrganizationID: orgID, Audience: aud, RecipientID: rid,
		}); err != nil && err != pgx.ErrNoRows {
			response.Fail(w, http.StatusInternalServerError, "internal", "could not update notification")
			return
		}
		h.respondUnread(w, r, aud, orgID, rid)
	}
}

func (h *Handler) readAll(aud string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, rid, ok := recipient(r)
		if !ok {
			response.Fail(w, http.StatusUnauthorized, "unauthorized", "no user context")
			return
		}
		if err := h.q.MarkAllNotificationsRead(r.Context(), gen.MarkAllNotificationsReadParams{
			OrganizationID: orgID, Audience: aud, RecipientID: rid,
		}); err != nil {
			response.Fail(w, http.StatusInternalServerError, "internal", "could not update notifications")
			return
		}
		response.JSON(w, http.StatusOK, map[string]any{"unread_count": 0})
	}
}

// pusherAuth signs a private-channel subscription, but only for the caller's own
// channel. Pusher posts a form body (socket_id, channel_name); we validate the
// channel against the JWT before signing the raw params.
func (h *Handler) pusherAuth(aud string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.authz.Enabled() {
			response.Fail(w, http.StatusServiceUnavailable, "unavailable", "realtime notifications not configured")
			return
		}
		orgID, rid, ok := recipient(r)
		if !ok {
			response.Fail(w, http.StatusUnauthorized, "unauthorized", "no user context")
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 4096))
		if err != nil {
			response.Fail(w, http.StatusBadRequest, "bad_request", "invalid body")
			return
		}
		vals, _ := url.ParseQuery(string(body))
		want := notify.ChannelName(aud, orgID, rid)
		if vals.Get("channel_name") != want {
			response.Fail(w, http.StatusForbidden, "forbidden", "channel does not belong to you")
			return
		}
		authBytes, err := h.authz.Authorize(body)
		if err != nil {
			response.Fail(w, http.StatusInternalServerError, "internal", "could not authorize channel")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(authBytes)
	}
}

func (h *Handler) respondUnread(w http.ResponseWriter, r *http.Request, aud string, orgID, rid int64) {
	unread, err := h.q.CountUnreadNotifications(r.Context(), gen.CountUnreadNotificationsParams{
		OrganizationID: orgID, Audience: aud, RecipientID: rid,
	})
	if err != nil {
		response.JSON(w, http.StatusOK, map[string]any{})
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"unread_count": unread})
}

// clampInt parses s as an int, falling back to def, then clamps to [min,max].
func clampInt(s string, def, min, max int) int {
	n := def
	if s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			n = v
		}
	}
	if n < min {
		n = min
	}
	if n > max {
		n = max
	}
	return n
}
