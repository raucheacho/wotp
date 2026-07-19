package whatsapp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func newTestPool(t *testing.T, ignoreGroups, ignoreStatus bool) *Pool {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "session.db")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	p, err := NewPool(PoolConfig{
		DBPath:       dbPath,
		DeviceName:   "test",
		Logger:       logger,
		IgnoreGroups: ignoreGroups,
		IgnoreStatus: ignoreStatus,
	})
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(func() { p.container.Close() })
	return p
}

func fakeDevice(user string, connected bool) *device {
	d := &device{
		jid:   types.NewJID(user, types.DefaultUserServer),
		phone: user,
	}
	d.connected = connected
	return d
}

// NewPool opens a real sqlite-backed sqlstore.Container. Actually pairing a
// device requires a live connection to WhatsApp's servers, which isn't
// something a unit test can drive — that stays a manual verification step.
func TestPool_ContainerOpensCleanly(t *testing.T) {
	p := newTestPool(t, true, true)

	devices, err := p.container.GetAllDevices(t.Context())
	if err != nil {
		t.Fatalf("GetAllDevices on a fresh container: %v", err)
	}
	if len(devices) != 0 {
		t.Fatalf("expected 0 devices in a fresh container, got %d", len(devices))
	}
}

func TestPool_IsConnectedAndNumbers_NoDeviceYet(t *testing.T) {
	p := newTestPool(t, true, true)

	if p.IsConnected() {
		t.Fatal("a pool with no paired device should not report connected")
	}
	numbers := p.Numbers()
	if len(numbers) != 0 {
		t.Fatalf("Numbers() = %v, want an empty (non-nil) slice", numbers)
	}
	if numbers == nil {
		t.Fatal("Numbers() must return a non-nil empty slice, not nil, so it serializes to [] not null")
	}
}

func TestPool_IsConnectedAndNumbers_DisconnectedDevice(t *testing.T) {
	p := newTestPool(t, true, true)
	p.device = fakeDevice("111", false)

	if p.IsConnected() {
		t.Fatal("a disconnected device should not report the pool as connected")
	}
	numbers := p.Numbers()
	if len(numbers) != 1 || numbers[0].Connected {
		t.Fatalf("Numbers() = %+v, want one disconnected entry", numbers)
	}
}

func TestPool_IsConnectedAndNumbers_ConnectedDevice(t *testing.T) {
	p := newTestPool(t, true, true)
	p.device = fakeDevice("111", true)

	if !p.IsConnected() {
		t.Fatal("a connected device should report the pool as connected")
	}
	numbers := p.Numbers()
	if len(numbers) != 1 || numbers[0].Phone != "111" || !numbers[0].Connected {
		t.Fatalf("Numbers() = %+v, want one connected entry for 111", numbers)
	}
}

// TestPool_PairRefusesWhenAlreadyPaired is the regression test for the
// "one number per project" rule: Pair must reject a second attempt before
// it ever touches the network (a real device below is enough to trigger
// the check, since it happens first).
func TestPool_PairRefusesWhenAlreadyPaired(t *testing.T) {
	p := newTestPool(t, true, true)
	p.device = fakeDevice("111", true)

	if _, err := p.Pair(context.Background()); !errors.Is(err, ErrAlreadyPaired) {
		t.Fatalf("Pair error = %v, want ErrAlreadyPaired", err)
	}
}

// TestPool_SendMessageErrorsWithoutAPairedNumber covers the no-failover
// send path (there's nothing to fail over to with one device — a send
// either works or returns the error directly).
func TestPool_SendMessageErrorsWithoutAPairedNumber(t *testing.T) {
	p := newTestPool(t, true, true)
	if _, err := p.SendMessage(context.Background(), "15550100", "hi"); err == nil {
		t.Fatal("expected an error sending with no number paired")
	}
}

func TestPool_SendMessageErrorsWhenDeviceDisconnected(t *testing.T) {
	p := newTestPool(t, true, true)
	p.device = fakeDevice("111", false)
	if _, err := p.SendMessage(context.Background(), "15550100", "hi"); err == nil {
		t.Fatal("expected an error sending through a disconnected device")
	}
}

func messageEvent(chat types.JID, isGroup bool, text string) *events.Message {
	return &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:    chat,
				Sender:  chat,
				IsGroup: isGroup,
			},
			ID:        "MSG1",
			PushName:  "Tester",
			Timestamp: time.Now(),
		},
		Message: &waE2E.Message{Conversation: proto.String(text)},
	}
}

func TestPool_HandleEvent_IgnoresGroupsByDefault(t *testing.T) {
	p := newTestPool(t, true, true)
	d := fakeDevice("111", true)

	groupJID := types.NewJID("999", types.GroupServer)
	p.handleEvent(d, messageEvent(groupJID, true, "hello group"))

	select {
	case evt := <-p.events:
		t.Fatalf("expected group message to be dropped, got event %+v", evt)
	default:
	}
}

func TestPool_HandleEvent_IgnoresStatusByDefault(t *testing.T) {
	p := newTestPool(t, true, true)
	d := fakeDevice("111", true)

	p.handleEvent(d, messageEvent(types.StatusBroadcastJID, false, "my story"))

	select {
	case evt := <-p.events:
		t.Fatalf("expected status broadcast to be dropped, got event %+v", evt)
	default:
	}
}

func TestPool_HandleEvent_DirectMessagePassesThroughAndTagsFrom(t *testing.T) {
	p := newTestPool(t, true, true)
	d := fakeDevice("111", true)

	dmJID := types.NewJID("222", types.DefaultUserServer)
	p.handleEvent(d, messageEvent(dmJID, false, "hi there"))

	select {
	case evt := <-p.events:
		if evt.Type != EventMessageReceived {
			t.Fatalf("evt.Type = %q, want %q", evt.Type, EventMessageReceived)
		}
		if evt.From != d.jid.String() {
			t.Fatalf("evt.From = %q, want %q", evt.From, d.jid.String())
		}
		if evt.Phone != "222" {
			t.Fatalf("evt.Phone = %q, want 222", evt.Phone)
		}
	default:
		t.Fatal("expected a message.received event, got none")
	}
}

func TestPool_HandleEvent_GroupsAllowedWhenNotIgnored(t *testing.T) {
	p := newTestPool(t, false, false)
	d := fakeDevice("111", true)

	groupJID := types.NewJID("999", types.GroupServer)
	p.handleEvent(d, messageEvent(groupJID, true, "hello group"))

	select {
	case evt := <-p.events:
		if evt.Type != EventMessageReceived {
			t.Fatalf("evt.Type = %q, want %q", evt.Type, EventMessageReceived)
		}
	default:
		t.Fatal("expected the group message to pass through when IgnoreGroups is false")
	}
}
