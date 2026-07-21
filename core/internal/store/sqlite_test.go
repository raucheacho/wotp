package store

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"
)

func newTestProjectStore(t *testing.T) *SQLiteProjectStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := NewSQLiteProjectStore(dbPath, logger)
	if err != nil {
		t.Fatalf("NewSQLiteProjectStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestNormalizePhone(t *testing.T) {
	cases := map[string]string{
		"+212 600-000000":   "212600000000",
		"212600000000":      "212600000000",
		"(212) 600 000 000": "212600000000",
	}
	for input, want := range cases {
		if got := NormalizePhone(input); got != want {
			t.Errorf("NormalizePhone(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestGetOrCreateConversation_IsIdempotentAndNormalizesPhone(t *testing.T) {
	ctx := context.Background()
	s := newTestProjectStore(t)

	c1, err := s.GetOrCreateConversation(ctx, "+212 600-000000")
	if err != nil {
		t.Fatalf("GetOrCreateConversation: %v", err)
	}
	if c1.Phone != "212600000000" {
		t.Fatalf("Phone = %q, want normalized 212600000000", c1.Phone)
	}
	if c1.State != ConversationStateBot {
		t.Fatalf("State = %q, want default %q", c1.State, ConversationStateBot)
	}

	// Same contact, different formatting — must resolve to the same row.
	c2, err := s.GetOrCreateConversation(ctx, "212600000000")
	if err != nil {
		t.Fatalf("GetOrCreateConversation (2nd): %v", err)
	}
	if c2.ID != c1.ID {
		t.Fatalf("expected the same conversation ID for the same phone in different formats, got %q vs %q", c1.ID, c2.ID)
	}

	all, err := s.ListConversations(ctx)
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected exactly 1 conversation despite 2 GetOrCreateConversation calls, got %d", len(all))
	}
}

func TestSetConversationState_UpdatesStateAndRecordsAudit(t *testing.T) {
	ctx := context.Background()
	s := newTestProjectStore(t)

	conv, err := s.GetOrCreateConversation(ctx, "212600000000")
	if err != nil {
		t.Fatalf("GetOrCreateConversation: %v", err)
	}

	if err := s.SetConversationState(ctx, conv.ID, ConversationStateHuman, "agent-42", "customer asked for a refund"); err != nil {
		t.Fatalf("SetConversationState: %v", err)
	}

	updated, err := s.GetConversationByID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetConversationByID: %v", err)
	}
	if updated.State != ConversationStateHuman {
		t.Fatalf("State = %q, want %q", updated.State, ConversationStateHuman)
	}
	if !updated.UpdatedAt.After(conv.UpdatedAt) && updated.UpdatedAt != conv.UpdatedAt {
		t.Fatalf("expected UpdatedAt to advance after a state change")
	}

	changes, err := s.ListConversationStateChanges(ctx, conv.ID)
	if err != nil {
		t.Fatalf("ListConversationStateChanges: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 audit row, got %d", len(changes))
	}
	if changes[0].FromState != ConversationStateBot || changes[0].ToState != ConversationStateHuman {
		t.Fatalf("unexpected transition recorded: %+v", changes[0])
	}
	if changes[0].Actor != "agent-42" || changes[0].Reason != "customer asked for a refund" {
		t.Fatalf("actor/reason not recorded correctly: %+v", changes[0])
	}

	// A second transition appends, it doesn't replace.
	if err := s.SetConversationState(ctx, conv.ID, ConversationStateBot, "agent-42", "resolved"); err != nil {
		t.Fatalf("SetConversationState (resume): %v", err)
	}
	changes, err = s.ListConversationStateChanges(ctx, conv.ID)
	if err != nil {
		t.Fatalf("ListConversationStateChanges (2nd): %v", err)
	}
	if len(changes) != 2 {
		t.Fatalf("expected 2 audit rows after a second transition, got %d", len(changes))
	}
	if changes[1].FromState != ConversationStateHuman || changes[1].ToState != ConversationStateBot {
		t.Fatalf("unexpected second transition: %+v", changes[1])
	}
}

func TestSetConversationState_ErrorsOnUnknownConversation(t *testing.T) {
	s := newTestProjectStore(t)
	if err := s.SetConversationState(context.Background(), "not-a-real-id", ConversationStateHuman, "", ""); err == nil {
		t.Fatal("expected an error for an unknown conversation id")
	}
}

func TestInboundMessages_PersistAndListChronologically(t *testing.T) {
	ctx := context.Background()
	s := newTestProjectStore(t)

	conv, err := s.GetOrCreateConversation(ctx, "212600000000")
	if err != nil {
		t.Fatalf("GetOrCreateConversation: %v", err)
	}

	base := time.Now().UTC()
	for i, text := range []string{"hi", "is my order coming?", "hello??"} {
		msg := &InboundMessage{
			ConversationID: conv.ID,
			Phone:          "212600000000",
			Content:        text,
			PushName:       "Client Test",
			MessageID:      "wamid-" + text,
			CreatedAt:      base.Add(time.Duration(i) * time.Minute),
		}
		if err := s.InsertInboundMessage(ctx, msg); err != nil {
			t.Fatalf("InsertInboundMessage(%q): %v", text, err)
		}
	}

	msgs, err := s.ListInboundMessagesByPhone(ctx, "+212 600-000000", 10)
	if err != nil {
		t.Fatalf("ListInboundMessagesByPhone: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 inbound messages, got %d", len(msgs))
	}
	want := []string{"hi", "is my order coming?", "hello??"}
	for i, w := range want {
		if msgs[i].Content != w {
			t.Fatalf("msgs[%d].Content = %q, want %q (chronological order)", i, msgs[i].Content, w)
		}
	}
}

func TestGetGenericMessagesByPhone_MatchesRegardlessOfFormatting(t *testing.T) {
	ctx := context.Background()
	s := newTestProjectStore(t)

	if err := s.SaveGenericMessage(ctx, &GenericMessage{
		ID: "m1", Phone: "+212600000000", MessageType: "text", Content: "your order shipped",
		Status: "sent", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("SaveGenericMessage: %v", err)
	}
	if err := s.SaveGenericMessage(ctx, &GenericMessage{
		ID: "m2", Phone: "999999999999", MessageType: "text", Content: "unrelated",
		Status: "sent", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("SaveGenericMessage (unrelated): %v", err)
	}

	msgs, err := s.GetGenericMessagesByPhone(ctx, "212600000000", 10)
	if err != nil {
		t.Fatalf("GetGenericMessagesByPhone: %v", err)
	}
	if len(msgs) != 1 || msgs[0].ID != "m1" {
		t.Fatalf("expected exactly the matching message, got %+v", msgs)
	}
}

func TestGetOTPRequestsByConversationID_FiltersAndOrdersChronologically(t *testing.T) {
	ctx := context.Background()
	s := newTestProjectStore(t)

	conv, err := s.GetOrCreateConversation(ctx, "212600000000")
	if err != nil {
		t.Fatalf("GetOrCreateConversation: %v", err)
	}
	otherConv, err := s.GetOrCreateConversation(ctx, "212699999999")
	if err != nil {
		t.Fatalf("GetOrCreateConversation (other): %v", err)
	}

	base := time.Now().UTC()
	mk := func(id, convID string, at time.Time) *OTPRequest {
		return &OTPRequest{
			ID: id, Token: "otp_tok_" + id, Phone: "212600000000", CodeHash: "hash",
			Status: StatusPending, ConversationID: convID,
			CreatedAt: at, ExpiresAt: at.Add(5 * time.Minute),
		}
	}
	if err := s.CreateOTPRequest(ctx, mk("o1", conv.ID, base)); err != nil {
		t.Fatalf("CreateOTPRequest o1: %v", err)
	}
	if err := s.CreateOTPRequest(ctx, mk("o2", conv.ID, base.Add(time.Minute))); err != nil {
		t.Fatalf("CreateOTPRequest o2: %v", err)
	}
	if err := s.CreateOTPRequest(ctx, mk("o3", otherConv.ID, base)); err != nil {
		t.Fatalf("CreateOTPRequest o3 (other conversation): %v", err)
	}

	got, err := s.GetOTPRequestsByConversationID(ctx, conv.ID, 10)
	if err != nil {
		t.Fatalf("GetOTPRequestsByConversationID: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 OTPs for this conversation, got %d: %+v", len(got), got)
	}
	if got[0].ID != "o1" || got[1].ID != "o2" {
		t.Fatalf("expected chronological order [o1, o2], got [%s, %s]", got[0].ID, got[1].ID)
	}
}
