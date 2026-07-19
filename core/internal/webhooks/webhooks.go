package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/wotp/core/internal/store"
	"github.com/wotp/core/internal/whatsapp"
	"github.com/wotp/core/internal/ws"
)

// Config holds the per-project webhook settings (endpoint, event filter,
// signing secret). Deliberately not core/internal/config.Config — that
// package only holds instance-wide settings now, while webhooks are
// per-project (see core/internal/project.Settings.Webhooks).
type Config struct {
	Endpoint string
	Events   []string
	Secret   string
}

type WebhookPayload struct {
	Event     string `json:"event"`
	Timestamp int64  `json:"timestamp"`
	Data      any    `json:"data"`
}

type Service struct {
	cfg        Config
	httpClient *http.Client
	store      store.ProjectStore
	wsHub      *ws.Hub
}

func NewService(cfg Config, st store.ProjectStore, wsHub *ws.Hub) *Service {
	return &Service{
		cfg:   cfg,
		store: st,
		wsHub: wsHub,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *Service) ProcessEvent(evt whatsapp.Event) {
	if s.cfg.Endpoint == "" {
		return
	}

	if len(s.cfg.Events) > 0 {
		matched := false
		for _, e := range s.cfg.Events {
			if e == evt.Type {
				matched = true
				break
			}
		}
		if !matched {
			return
		}
	}

	go s.sendWebhook(evt)
}

func (s *Service) sendWebhook(evt whatsapp.Event) {
	payload := WebhookPayload{
		Event:     evt.Type,
		Timestamp: evt.At.Unix(),
		Data:      evt,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	req, err := http.NewRequest(http.MethodPost, s.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	if s.cfg.Secret != "" {
		h := hmac.New(sha256.New, []byte(s.cfg.Secret))
		h.Write(body)
		signature := hex.EncodeToString(h.Sum(nil))
		req.Header.Set("X-Wotp-Signature", signature)
	}

	statusCode := 0
	errMsg := ""

	resp, err := s.httpClient.Do(req)
	if err != nil {
		errMsg = err.Error()
	} else {
		statusCode = resp.StatusCode
		resp.Body.Close()
	}

	if s.store != nil {
		log := &store.WebhookLog{
			ID:         uuid.New().String(),
			EventType:  evt.Type,
			Payload:    string(body),
			StatusCode: statusCode,
			Error:      errMsg,
			CreatedAt:  time.Now().UTC(),
		}
		// Using context.Background() as we are in a background goroutine
		_ = s.store.SaveWebhookLog(context.Background(), log)
	}

	if s.wsHub != nil {
		s.wsHub.Broadcast(ws.Event{
			Type:       "webhook.event",
			EventName:  evt.Type,
			URL:        s.cfg.Endpoint,
			Payload:    payload,
			StatusCode: statusCode,
			At:         time.Now().UTC().Format(time.RFC3339),
		})
	}
}
