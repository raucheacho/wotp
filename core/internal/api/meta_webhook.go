package api

import (
	"io"
	"net/http"

	"github.com/wotp/core/internal/whatsapp"
)

// handleMetaWebhookVerify handles Meta's GET verification handshake, fired
// whenever the operator (re)saves the webhook subscription URL in the Meta
// app dashboard. Unauthenticated by necessity — Meta can't send an apikey
// header on this call — authorized instead by VerifyToken, an arbitrary
// string set on both sides (see project.Settings.Cloud.VerifyToken).
func (s *Server) handleMetaWebhookVerify(w http.ResponseWriter, r *http.Request) {
	rt := s.runtime()
	verifyToken := rt.Settings.Cloud.VerifyToken

	if verifyToken == "" ||
		r.URL.Query().Get("hub.mode") != "subscribe" ||
		r.URL.Query().Get("hub.verify_token") != verifyToken {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "verification failed"})
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(r.URL.Query().Get("hub.challenge")))
}

// handleMetaWebhookEvents receives inbound message and delivery-status
// events pushed by Meta. Unauthenticated by necessity (see
// handleMetaWebhookVerify) — authenticity instead comes from verifying
// X-Hub-Signature-256 against Settings.Cloud.AppSecret before trusting
// anything in the body.
func (s *Server) handleMetaWebhookEvents(w http.ResponseWriter, r *http.Request) {
	rt := s.runtime()
	if rt.Cloud == nil || rt.Settings.Cloud.AppSecret == "" {
		// Refuse rather than silently accept unauthenticated inbound
		// traffic — inbound is opt-in, and it isn't safe to process until
		// an App Secret is configured to verify it came from Meta.
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "cloud inbound webhook not configured"})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read request body"})
		return
	}

	if !whatsapp.VerifyMetaSignature(body, r.Header.Get("X-Hub-Signature-256"), rt.Settings.Cloud.AppSecret) {
		s.logger.Error("meta webhook: signature verification failed")
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid signature"})
		return
	}

	events, err := whatsapp.ParseMetaWebhook(body)
	if err != nil {
		s.logger.Error("meta webhook: failed to parse payload", "error", err)
		// Still 200 — Meta retries aggressively on non-2xx, and a payload
		// that fails to parse now won't parse any better on retry.
		w.WriteHeader(http.StatusOK)
		return
	}

	for _, evt := range events {
		// ParseMetaWebhook does no network I/O — a media message only
		// carries Meta's reference (Data["mediaID"]) at this point.
		// Resolving it to actual bytes needs rt.Cloud (the access token),
		// which only this handler has — same fail-open policy as the rest
		// of this event path: a download failure still lets the event
		// through, just without retrievable media.
		if data, ok := evt.Data.(map[string]interface{}); ok {
			if mediaID, _ := data["mediaID"].(string); mediaID != "" {
				if mediaBytes, _, err := rt.Cloud.DownloadMedia(r.Context(), mediaID); err != nil {
					s.logger.Error("meta webhook: failed to download media", "media_id", mediaID, "error", err)
				} else {
					data["mediaBytes"] = mediaBytes
				}
			}
		}
		rt.Cloud.PushEvent(evt)
	}
	w.WriteHeader(http.StatusOK)
}
