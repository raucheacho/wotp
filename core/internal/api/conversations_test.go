package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/wotp/core/internal/store"
	"github.com/wotp/core/internal/whatsapp"
)

func inboundEvent(phone, text string) whatsapp.Event {
	return inboundEventAt(phone, text, time.Now().UTC())
}

func inboundEventAt(phone, text string, at time.Time) whatsapp.Event {
	return whatsapp.Event{
		Type:  whatsapp.EventMessageReceived,
		Phone: phone,
		At:    at,
		Data:  map[string]interface{}{"text": text, "pushName": "Client Test"},
	}
}

// TestRouteInboundMessage_AlwaysForwardsWithStateInPayload is a regression
// test for the takeover redesign: wotp no longer decides whether to forward
// message.received based on conversation state — it always forwards, and
// includes conversation_state in the event's Data so the receiving app's
// own bot logic can decide whether to act.
func TestRouteInboundMessage_AlwaysForwardsWithStateInPayload(t *testing.T) {
	env := newTestEnv(t)
	rt := env.server.runtime()

	enriched, err := routeInboundMessage(context.Background(), rt.Store, rt.MediaDir, inboundEvent("212600000000", "hello"))
	if err != nil {
		t.Fatalf("routeInboundMessage (1st): %v", err)
	}
	data, ok := enriched.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected enriched.Data to be a map, got %T", enriched.Data)
	}
	if data["conversation_state"] != store.ConversationStateBot {
		t.Fatalf("conversation_state (1st) = %v, want %q", data["conversation_state"], store.ConversationStateBot)
	}

	conv, err := rt.Store.GetOrCreateConversation(context.Background(), "212600000000")
	if err != nil {
		t.Fatalf("GetOrCreateConversation: %v", err)
	}
	if err := rt.Store.SetConversationState(context.Background(), conv.ID, store.ConversationStateHuman, "agent", "manual takeover"); err != nil {
		t.Fatalf("SetConversationState: %v", err)
	}

	enriched, err = routeInboundMessage(context.Background(), rt.Store, rt.MediaDir, inboundEvent("212600000000", "still there?"))
	if err != nil {
		t.Fatalf("routeInboundMessage (2nd): %v", err)
	}
	data, ok = enriched.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected enriched.Data to be a map, got %T", enriched.Data)
	}
	// Still forwarded (the caller decides what to do with it) — only the
	// state included in the payload changed.
	if data["conversation_state"] != store.ConversationStateHuman {
		t.Fatalf("conversation_state (2nd) = %v, want %q", data["conversation_state"], store.ConversationStateHuman)
	}

	// Both messages must still be persisted regardless of conversation
	// state — a human opening the conversation needs the full history.
	msgs, err := rt.Store.ListInboundMessagesByPhone(context.Background(), "212600000000", 10)
	if err != nil {
		t.Fatalf("ListInboundMessagesByPhone: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected both inbound messages to be persisted, got %d", len(msgs))
	}
}

func TestConversationsAPI_ListGetTakeoverResume(t *testing.T) {
	env := newTestEnv(t)
	rt := env.server.runtime()
	anonKey := env.anonKey

	// Empty at first.
	rec := env.do(t, http.MethodGet, "/v1/conversations", anonKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list conversations: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var listed []store.Conversation
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("expected no conversations yet, got %d", len(listed))
	}

	// Simulate an inbound message creating the conversation.
	if _, err := routeInboundMessage(context.Background(), rt.Store, rt.MediaDir, inboundEvent("212600000000", "hi")); err != nil {
		t.Fatalf("routeInboundMessage: %v", err)
	}

	rec = env.do(t, http.MethodGet, "/v1/conversations", anonKey, nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 conversation after an inbound message, got %d", len(listed))
	}
	conv := listed[0]
	if conv.State != store.ConversationStateBot {
		t.Fatalf("State = %q, want default %q", conv.State, store.ConversationStateBot)
	}

	// GET single conversation.
	rec = env.do(t, http.MethodGet, "/v1/conversations/"+conv.ID, anonKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get conversation: status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// GET unknown conversation -> 404.
	rec = env.do(t, http.MethodGet, "/v1/conversations/not-a-real-id", anonKey, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get unknown conversation: status = %d, want 404", rec.Code)
	}

	// Takeover.
	rec = env.do(t, http.MethodPost, "/v1/conversations/"+conv.ID+"/takeover", anonKey, ConversationStateChangeRequest{
		Actor: "agent-1", Reason: "customer escalation",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("takeover: status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = env.do(t, http.MethodGet, "/v1/conversations/"+conv.ID, anonKey, nil)
	var afterTakeover store.Conversation
	if err := json.Unmarshal(rec.Body.Bytes(), &afterTakeover); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if afterTakeover.State != store.ConversationStateHuman {
		t.Fatalf("State after takeover = %q, want %q", afterTakeover.State, store.ConversationStateHuman)
	}

	changes, err := rt.Store.ListConversationStateChanges(context.Background(), conv.ID)
	if err != nil {
		t.Fatalf("ListConversationStateChanges: %v", err)
	}
	if len(changes) != 1 || changes[0].Actor != "agent-1" || changes[0].Reason != "customer escalation" {
		t.Fatalf("unexpected audit trail after takeover: %+v", changes)
	}

	// Resume.
	rec = env.do(t, http.MethodPost, "/v1/conversations/"+conv.ID+"/resume", anonKey, ConversationStateChangeRequest{
		Actor: "agent-1", Reason: "resolved",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("resume: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = env.do(t, http.MethodGet, "/v1/conversations/"+conv.ID, anonKey, nil)
	var afterResume store.Conversation
	if err := json.Unmarshal(rec.Body.Bytes(), &afterResume); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if afterResume.State != store.ConversationStateBot {
		t.Fatalf("State after resume = %q, want %q", afterResume.State, store.ConversationStateBot)
	}
}

func TestConversationsAPI_MessagesMergeInboundAndOutbound(t *testing.T) {
	env := newTestEnv(t)
	rt := env.server.runtime()
	anonKey := env.anonKey

	base := time.Now().UTC()
	inboundAt := base
	outboundAt := base.Add(1 * time.Minute) // well-separated: avoids any ambiguity in merge ordering

	if _, err := routeInboundMessage(context.Background(), rt.Store, rt.MediaDir, inboundEventAt("212600000000", "where is my order?", inboundAt)); err != nil {
		t.Fatalf("routeInboundMessage: %v", err)
	}
	if err := rt.Store.SaveGenericMessage(context.Background(), &store.GenericMessage{
		ID: "wamid-out-1", Phone: "+212600000000", MessageType: "text", Content: "it's on its way!",
		Status: "sent", CreatedAt: outboundAt, UpdatedAt: outboundAt,
	}); err != nil {
		t.Fatalf("SaveGenericMessage: %v", err)
	}

	convs, err := rt.Store.ListConversations(context.Background())
	if err != nil || len(convs) != 1 {
		t.Fatalf("ListConversations: %v (%d)", err, len(convs))
	}
	conv := convs[0]

	rec := env.do(t, http.MethodGet, "/v1/conversations/"+conv.ID+"/messages", anonKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get messages: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var msgs []conversationMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &msgs); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (1 inbound + 1 outbound), got %d: %+v", len(msgs), msgs)
	}
	if msgs[0].Direction != "inbound" || msgs[0].Content != "where is my order?" {
		t.Fatalf("msgs[0] = %+v, want the inbound message first (chronological)", msgs[0])
	}
	if msgs[1].Direction != "outbound" || msgs[1].Content != "it's on its way!" {
		t.Fatalf("msgs[1] = %+v, want the outbound reply second", msgs[1])
	}
}

// TestConversationsAPI_MessagesExposeInboundMediaKind is a regression test:
// InboundMessage.MediaKind/MediaMimeType were added to the schema and
// written by routeInboundMessage, but GET .../messages' merge loop never
// read them back onto conversationMessage — a media message showed up
// indistinguishable from an empty text message.
func TestConversationsAPI_MessagesExposeInboundMediaKind(t *testing.T) {
	env := newTestEnv(t)
	rt := env.server.runtime()
	anonKey := env.anonKey

	conv, err := rt.Store.GetOrCreateConversation(context.Background(), "212600000000")
	if err != nil {
		t.Fatalf("GetOrCreateConversation: %v", err)
	}
	if err := rt.Store.InsertInboundMessage(context.Background(), &store.InboundMessage{
		ConversationID: conv.ID,
		Phone:          "212600000000",
		Content:        "look at this",
		MessageID:      "wamid.IMG1",
		MediaKind:      "image",
		MediaMimeType:  "image/jpeg",
		CreatedAt:      time.Now().UTC(),
	}); err != nil {
		t.Fatalf("InsertInboundMessage: %v", err)
	}

	rec := env.do(t, http.MethodGet, "/v1/conversations/"+conv.ID+"/messages", anonKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get messages: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var msgs []conversationMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &msgs); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d: %+v", len(msgs), msgs)
	}
	if msgs[0].Kind != "image" {
		t.Fatalf("msgs[0].Kind = %q, want %q", msgs[0].Kind, "image")
	}
	if msgs[0].MediaMimeType != "image/jpeg" {
		t.Fatalf("msgs[0].MediaMimeType = %q, want %q", msgs[0].MediaMimeType, "image/jpeg")
	}
}

// TestConversationsAPI_OTPSendShowsUpInMessages is a regression test for
// folding OTP into the conversation model: POST /v1/otp/send must link its
// OTP request to the conversation for that phone number, so it shows up in
// GET .../messages with Kind "otp" and its status — but never the code.
func TestConversationsAPI_OTPSendShowsUpInMessages(t *testing.T) {
	env := newTestEnv(t)
	anonKey := env.anonKey

	rec := env.do(t, http.MethodPost, "/v1/otp/send", anonKey, OTPSendRequest{Phone: "+212600000000"})
	if rec.Code != http.StatusOK {
		t.Fatalf("otp send: status = %d, body = %s", rec.Code, rec.Body.String())
	}

	convs, err := env.server.runtime().Store.ListConversations(context.Background())
	if err != nil || len(convs) != 1 {
		t.Fatalf("ListConversations: %v (%d)", err, len(convs))
	}
	conv := convs[0]
	if conv.Phone != "212600000000" {
		t.Fatalf("conversation phone = %q, want the normalized phone the OTP was sent to", conv.Phone)
	}

	rec = env.do(t, http.MethodGet, "/v1/conversations/"+conv.ID+"/messages", anonKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get messages: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var msgs []conversationMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &msgs); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message (the OTP send), got %d: %+v", len(msgs), msgs)
	}
	if msgs[0].Direction != "outbound" || msgs[0].Kind != "otp" {
		t.Fatalf("msgs[0] = %+v, want an outbound otp-kind entry", msgs[0])
	}
	if msgs[0].Status != string(store.StatusPending) {
		t.Fatalf("msgs[0].Status = %q, want %q (no number paired in this test, so the send itself hasn't resolved)", msgs[0].Status, store.StatusPending)
	}
	if strings.Contains(rec.Body.String(), "code") {
		t.Fatalf("response body must never mention the OTP code: %s", rec.Body.String())
	}
}
