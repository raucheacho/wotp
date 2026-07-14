package webhooks

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/wotp/core/internal/config"
	"github.com/wotp/core/internal/whatsapp"
)

type WebhookPayload struct {
	Event     string `json:"event"`
	Timestamp int64  `json:"timestamp"`
	Data      any    `json:"data"`
}

type Service struct {
	cfg        config.WebhooksConfig
	httpClient *http.Client
}

func NewService(cfg config.WebhooksConfig) *Service {
	return &Service{
		cfg: cfg,
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

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}
