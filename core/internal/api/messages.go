package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/wotp/core/internal/store"
	"github.com/wotp/core/internal/whatsapp"
	"github.com/wotp/core/internal/ws"
)

// mediaKindsByType maps SendMessageRequest.Type values (other than "text")
// onto whatsapp.MediaKind — "media" is kept as a legacy alias for "image"
// so requests written before Kind existed keep working unchanged.
var mediaKindsByType = map[string]whatsapp.MediaKind{
	"media":    whatsapp.MediaKindImage,
	"image":    whatsapp.MediaKindImage,
	"video":    whatsapp.MediaKindVideo,
	"audio":    whatsapp.MediaKindAudio,
	"document": whatsapp.MediaKindDocument,
}

type SendMessageRequest struct {
	Phone   string `json:"phone"`
	Type    string `json:"type"` // "text" | "image" | "video" | "audio" | "document" | "location" | "media" (legacy alias for "image")
	Text    string `json:"text,omitempty"`
	URL     string `json:"url,omitempty"`
	Base64  string `json:"base64,omitempty"`
	Caption string `json:"caption,omitempty"`
	// Filename is shown as the file name in the recipient's chat.
	// document type only — ignored otherwise.
	Filename string `json:"filename,omitempty"`
	// Latitude/Longitude/Name/Address are location type only. Name/Address
	// are optional; Latitude/Longitude are required (both zero is treated
	// as "not provided" — legitimate (0,0) sends are vanishingly rare).
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
	Name      string  `json:"name,omitempty"`
	Address   string  `json:"address,omitempty"`
}

func (s *Server) handleMessagesSend(w http.ResponseWriter, r *http.Request) {
	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Phone == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "phone is required"})
		return
	}

	rt := s.runtime()

	if !rt.AllowSend() {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
		return
	}

	if req.Type == "" {
		req.Type = "text"
	}

	ctx := r.Context()
	var result *whatsapp.SendResult
	var sendErr error

	// Prefer Cloud when the instance has it enabled — same policy
	// handleOTPSend uses. Without this, a Cloud-only instance (no
	// whatsmeow number ever paired) would get a confusing "no device"
	// error here even though its OTP sends work fine.
	switch req.Type {
	case "text":
		if rt.Cloud != nil {
			result, sendErr = rt.Cloud.SendMessage(ctx, req.Phone, req.Text)
		} else {
			result, sendErr = rt.WA.SendMessage(ctx, req.Phone, req.Text)
		}

	case "location":
		if req.Latitude == 0 && req.Longitude == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "latitude and longitude are required for a location send"})
			return
		}
		opts := whatsapp.LocationSendOptions{Latitude: req.Latitude, Longitude: req.Longitude, Name: req.Name, Address: req.Address}
		if rt.Cloud != nil {
			result, sendErr = rt.Cloud.SendLocation(ctx, req.Phone, opts)
		} else {
			result, sendErr = rt.WA.SendLocation(ctx, req.Phone, opts)
		}

	default:
		kind, ok := mediaKindsByType[req.Type]
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": `type must be "text", "location", "image", "video", "audio", or "document"`})
			return
		}
		req.Type = string(kind) // normalize the legacy "media" alias before it's stored/broadcast below
		opts := whatsapp.MediaSendOptions{URL: req.URL, Base64Data: req.Base64, Caption: req.Caption, Kind: kind, Filename: req.Filename}
		if rt.Cloud != nil {
			result, sendErr = rt.Cloud.SendMedia(ctx, req.Phone, opts)
		} else {
			result, sendErr = rt.WA.SendMedia(ctx, req.Phone, opts)
		}
	}

	var msgID, errMsg string
	status := "sent"
	if sendErr != nil {
		s.logger.Error("failed to send message", "error", sendErr, "phone", req.Phone, "type", req.Type)
		status = "failed"
		errMsg = sendErr.Error()
	} else {
		msgID = result.MessageID
	}

	var content string
	switch req.Type {
	case "text":
		content = req.Text
	case "location":
		content = req.Name
		if content == "" {
			content = fmt.Sprintf("%.6f, %.6f", req.Latitude, req.Longitude)
		}
	default:
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
	rt := s.runtime()
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

	rt := s.runtime()
	var err error
	if rt.Cloud != nil {
		err = rt.Cloud.SetPresence(r.Context(), req.Phone, req.State)
	} else {
		err = rt.WA.SetPresence(r.Context(), req.Phone, req.State)
	}
	if err != nil {
		s.logger.Error("failed to set presence", "error", err, "phone", req.Phone, "state", req.State)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleChats(w http.ResponseWriter, r *http.Request) {
	rt := s.runtime()
	var chats []whatsapp.Chat
	var err error
	if rt.Cloud != nil {
		chats, err = rt.Cloud.GetChats(r.Context())
	} else {
		chats, err = rt.WA.GetChats(r.Context())
	}
	if err != nil {
		s.logger.Error("failed to get chats", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get chats"})
		return
	}

	writeJSON(w, http.StatusOK, chats)
}
