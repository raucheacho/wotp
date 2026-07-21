package api

import (
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
)

// handleGetMedia streams the raw bytes of an inbound media message
// (image/video/audio/document) downloaded by routeInboundMessage at receive
// time. Same auth tier as the conversation-read endpoints — a dev's bot
// already reads /v1/conversations/*, this isn't a new trust boundary.
func (s *Server) handleGetMedia(w http.ResponseWriter, r *http.Request) {
	rt := s.runtime()
	messageID := chi.URLParam(r, "message_id")

	msg, err := rt.Store.GetInboundMessageByMessageID(r.Context(), messageID)
	if err != nil {
		s.logger.Error("failed to get inbound message", "message_id", messageID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get media"})
		return
	}
	if msg == nil || msg.MediaKind == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "media not found"})
		return
	}

	// message_id is only ever trusted once GetInboundMessageByMessageID
	// confirms it names a row wotp itself wrote — isSafeMediaFilename was
	// already enforced on write in routeInboundMessage, filepath.Base here
	// is just defense in depth against the same path-traversal class.
	f, err := os.Open(filepath.Join(rt.MediaDir, filepath.Base(msg.MessageID)))
	if err != nil {
		// DB row exists but the file doesn't — the download or write
		// failed at receive time. Documented, not a server error.
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "media file not available"})
		return
	}
	defer f.Close()

	if msg.MediaMimeType != "" {
		w.Header().Set("Content-Type", msg.MediaMimeType)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, f)
}
