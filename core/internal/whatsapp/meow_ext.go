package whatsapp

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// SendLocation sends a location pin to the given phone number via
// WhatsApp. opts.Name/Address are optional.
func (m *MeowClient) SendLocation(ctx context.Context, phone string, opts LocationSendOptions) (*SendResult, error) {
	if !m.IsConnected() {
		return nil, fmt.Errorf("whatsapp: not connected")
	}
	jid := toJID(phone)

	loc := &waE2E.LocationMessage{
		DegreesLatitude:  proto.Float64(opts.Latitude),
		DegreesLongitude: proto.Float64(opts.Longitude),
	}
	if opts.Name != "" {
		loc.Name = proto.String(opts.Name)
	}
	if opts.Address != "" {
		loc.Address = proto.String(opts.Address)
	}

	resp, err := m.client.SendMessage(ctx, jid, &waE2E.Message{LocationMessage: loc})
	if err != nil {
		m.emitEvent(Event{Type: EventMessageFailed, Phone: phone, Error: err.Error(), At: time.Now().UTC()})
		return nil, fmt.Errorf("whatsapp: send location: %w", err)
	}

	m.emitEvent(Event{Type: EventMessageSent, Phone: phone, MessageID: resp.ID, At: time.Now().UTC()})
	return &SendResult{MessageID: resp.ID}, nil
}

// SendMedia sends a media message to the given phone number via WhatsApp.
// Kind selects the upload type and waE2E.Message field — see
// mediaKindHandlers in pool.go, shared with Pool.SendMedia.
func (m *MeowClient) SendMedia(ctx context.Context, phone string, opts MediaSendOptions) (*SendResult, error) {
	if !m.IsConnected() {
		return nil, fmt.Errorf("whatsapp: not connected")
	}

	// Shared with Pool.SendMedia — includes the response status-code check
	// that makes a gated/expired/error-page URL fail loudly here instead of
	// silently uploading the error page as if it were the file.
	data, err := fetchMediaData(ctx, opts.URL, opts.Base64Data)
	if err != nil {
		return nil, err
	}

	contentType := http.DetectContentType(data)
	jid := toJID(phone)

	if m.simulateTyping {
		_ = m.client.SendChatPresence(ctx, jid, types.ChatPresenceComposing, types.ChatPresenceMediaText)
		time.Sleep(2 * time.Second)
		_ = m.client.SendChatPresence(ctx, jid, types.ChatPresencePaused, types.ChatPresenceMediaText)
	}

	mediaType, buildMessage := mediaKindHandlers(opts.Kind)
	resp, err := m.client.Upload(ctx, data, mediaType)
	if err != nil {
		return nil, fmt.Errorf("whatsapp: upload media: %w", err)
	}
	msg := buildMessage(resp, contentType, uint64(len(data)), opts.Caption, opts.Filename)

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
