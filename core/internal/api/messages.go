package api

import (
	"encoding/json"
	"net/http"
	"time"
)

type SendMessageRequest struct {
	Phone   string `json:"phone"`
	Type    string `json:"type"` // "text" or "media"
	Text    string `json:"text,omitempty"`
	URL     string `json:"url,omitempty"`
	Base64  string `json:"base64,omitempty"`
	Caption string `json:"caption,omitempty"`
}

func (s *Server) handleMessagesSend(w http.ResponseWriter, r *http.Request) {
	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Phone == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "phone is required"})
		return
	}

	// Basic Rate Limiting Check
	s.msgMu.Lock()
	now := time.Now()
	if now.Sub(s.msgWindow) >= time.Minute {
		s.msgWindow = now
		s.msgCount = 0
	}
	s.msgCount++
	if s.config.Messaging.MaxMessagesPerMinute > 0 && s.msgCount > s.config.Messaging.MaxMessagesPerMinute {
		s.msgMu.Unlock()
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
		return
	}
	s.msgMu.Unlock()

	var msgID string
	ctx := r.Context()

	if req.Type == "media" {
		result, err := s.waClient.SendMedia(ctx, req.Phone, req.URL, req.Base64, req.Caption)
		if err != nil {
			s.logger.Error("failed to send media", "error", err, "phone", req.Phone)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to send media"})
			return
		}
		msgID = result.MessageID
	} else {
		// default to text
		result, err := s.waClient.SendMessage(ctx, req.Phone, req.Text)
		if err != nil {
			s.logger.Error("failed to send message", "error", err, "phone", req.Phone)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to send message"})
			return
		}
		msgID = result.MessageID
	}

	writeJSON(w, http.StatusOK, map[string]string{"message_id": msgID})
}

func (s *Server) handleChats(w http.ResponseWriter, r *http.Request) {
	chats, err := s.waClient.GetChats(r.Context())
	if err != nil {
		s.logger.Error("failed to get chats", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get chats"})
		return
	}

	writeJSON(w, http.StatusOK, chats)
}
