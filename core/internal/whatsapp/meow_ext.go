package whatsapp

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// SendMedia sends a media message to the given phone number via WhatsApp.
func (m *MeowClient) SendMedia(ctx context.Context, phone, url, base64Data, caption string) (*SendResult, error) {
	if !m.IsConnected() {
		return nil, fmt.Errorf("whatsapp: not connected")
	}

	var data []byte
	var err error

	if base64Data != "" {
		data, err = base64.StdEncoding.DecodeString(base64Data)
		if err != nil {
			return nil, fmt.Errorf("whatsapp: invalid base64 data: %w", err)
		}
	} else if url != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("whatsapp: invalid url: %w", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("whatsapp: download media: %w", err)
		}
		defer resp.Body.Close()
		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("whatsapp: read media: %w", err)
		}
	} else {
		return nil, fmt.Errorf("whatsapp: neither url nor base64Data provided")
	}

	contentType := http.DetectContentType(data)

	// Strip non-digit characters for WhatsApp JID
	var cleanPhone string
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			cleanPhone += string(r)
		}
	}
	jid := types.NewJID(cleanPhone, types.DefaultUserServer)

	if m.simulateTyping {
		_ = m.client.SendChatPresence(ctx, jid, types.ChatPresenceComposing, types.ChatPresenceMediaText)
		time.Sleep(2 * time.Second)
		_ = m.client.SendChatPresence(ctx, jid, types.ChatPresencePaused, types.ChatPresenceMediaText)
	}

	resp, err := m.client.Upload(ctx, data, whatsmeow.MediaImage)
	if err != nil {
		return nil, fmt.Errorf("whatsapp: upload media: %w", err)
	}

	msg := &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			Caption:       proto.String(caption),
			Mimetype:      proto.String(contentType),
			URL:           proto.String(resp.URL),
			DirectPath:    proto.String(resp.DirectPath),
			MediaKey:      resp.MediaKey,
			FileEncSHA256: resp.FileEncSHA256,
			FileSHA256:    resp.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(data))),
		},
	}

	sendResp, err := m.client.SendMessage(ctx, jid, msg)
	if err != nil {
		m.emitEvent(Event{
			Type:  EventMessageFailed,
			Phone: phone,
			Error: err.Error(),
			At:    time.Now().UTC(),
		})
		return nil, fmt.Errorf("whatsapp: send media message: %w", err)
	}

	msgID := sendResp.ID
	m.emitEvent(Event{
		Type:      EventMessageSent,
		Phone:     phone,
		MessageID: msgID,
		At:        time.Now().UTC(),
	})

	return &SendResult{MessageID: msgID}, nil
}

// GetChats returns a list of existing chats.
func (m *MeowClient) GetChats(ctx context.Context) ([]Chat, error) {
	if !m.IsConnected() {
		return nil, fmt.Errorf("whatsapp: not connected")
	}

	contacts, err := m.client.Store.Contacts.GetAllContacts(ctx)
	if err != nil {
		return nil, fmt.Errorf("whatsapp: get contacts: %w", err)
	}

	var chats []Chat
	for jid, contact := range contacts {
		chats = append(chats, Chat{
			JID:  jid.String(),
			Name: contact.FullName,
		})
	}
	return chats, nil
}
