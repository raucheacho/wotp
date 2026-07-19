package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/wotp/core/internal/store"
	"github.com/wotp/core/internal/ws"
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

	rt := runtimeFromContext(r.Context())

	if !rt.AllowSend() {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
		return
	}

	var msgID string
	var errMsg string
	var status = "sent"
	ctx := r.Context()

	if req.Type == "" {
		req.Type = "text"
	}

	if req.Type == "media" {
		result, err := rt.WA.SendMedia(ctx, req.Phone, req.URL, req.Base64, req.Caption)
		if err != nil {
			s.logger.Error("failed to send media", "error", err, "phone", req.Phone)
			status = "failed"
			errMsg = err.Error()
		} else {
			msgID = result.MessageID
		}
	} else {
		// default to text
		result, err := rt.WA.SendMessage(ctx, req.Phone, req.Text)
		if err != nil {
			s.logger.Error("failed to send message", "error", err, "phone", req.Phone)
			status = "failed"
			errMsg = err.Error()
		} else {
			msgID = result.MessageID
		}
	}

	content := req.Text
	if req.Type == "media" {
		content = req.Caption
		if content == "" {
			content = "[Media]"
		}
	}

	dbID := msgID
	if dbID == "" {
		dbID = uuid.New().String()
	}

	dbMsg := &store.GenericMessage{
		ID:          dbID,
		Phone:       req.Phone,
		MessageType: req.Type,
		Content:     content,
		Status:      status,
		Error:       errMsg,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := rt.Store.SaveGenericMessage(ctx, dbMsg); err != nil {
		s.logger.Error("failed to save generic message", "error", err)
	}

	if status == "sent" {
		s.wsHub.Broadcast(ws.Event{
			Type:      "generic.message.sent",
			ProjectID: rt.Project.ID,
			MessageID: msgID,
			Phone:     req.Phone,
			MsgType:   req.Type,
			Content:   content,
			At:        time.Now().UTC().Format(time.RFC3339),
		})
	}

	if status == "failed" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to send message"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message_id": msgID})
}

func (s *Server) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	rt := runtimeFromContext(r.Context())
	msgs, err := rt.Store.GetGenericMessages(r.Context(), 100)
	if err != nil {
		s.logger.Error("failed to get generic messages", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get messages"})
		return
	}
	if msgs == nil {
		msgs = []store.GenericMessage{}
	}
	writeJSON(w, http.StatusOK, msgs)
}

type PresenceRequest struct {
	Phone string `json:"phone"`
	State string `json:"state"` // "typing" or "paused"
}

func (s *Server) handlePresence(w http.ResponseWriter, r *http.Request) {
	var req PresenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Phone == "" || req.State == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "phone and state are required"})
		return
	}

	rt := runtimeFromContext(r.Context())
	if err := rt.WA.SetPresence(r.Context(), req.Phone, req.State); err != nil {
		s.logger.Error("failed to set presence", "error", err, "phone", req.Phone, "state", req.State)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleChats(w http.ResponseWriter, r *http.Request) {
	rt := runtimeFromContext(r.Context())
	chats, err := rt.WA.GetChats(r.Context())
	if err != nil {
		s.logger.Error("failed to get chats", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get chats"})
		return
	}

	writeJSON(w, http.StatusOK, chats)
}
