package whatsapp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
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

func TestPool_SendLocationErrorsWithoutAPairedNumber(t *testing.T) {
	p := newTestPool(t, true, true)
	if _, err := p.SendLocation(context.Background(), "15550100", LocationSendOptions{Latitude: 1, Longitude: 2}); err == nil {
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

func fromMeMessageEvent(chat types.JID, text string) *events.Message {
	evt := messageEvent(chat, false, text)
	evt.Info.IsFromMe = true
	return evt
}

func protocolMessageEvent(chat types.JID) *events.Message {
	return &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{Chat: chat, Sender: chat},
			ID:            "MSG-PROTO",
			Timestamp:     time.Now(),
		},
		Message: &waE2E.Message{
			ProtocolMessage: &waE2E.ProtocolMessage{
				Type: waE2E.ProtocolMessage_HISTORY_SYNC_NOTIFICATION.Enum(),
			},
		},
	}
}

// lidMessageEvent simulates WhatsApp addressing the sender by LID (a
// privacy-preserving numeric ID) instead of their phone number — SenderAlt
// carries the real phone-number JID in that case, per whatsmeow's
// MessageSource doc comment.
func lidMessageEvent(lid, realPhoneJID types.JID, text string) *events.Message {
	return &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:      lid,
				Sender:    lid,
				SenderAlt: realPhoneJID,
			},
			ID:        "MSG-LID",
			PushName:  "Tester",
			Timestamp: time.Now(),
		},
		Message: &waE2E.Message{Conversation: proto.String(text)},
	}
}

func locationMessageEvent(chat types.JID, latitude, longitude float64, name string) *events.Message {
	loc := &waE2E.LocationMessage{
		DegreesLatitude:  proto.Float64(latitude),
		DegreesLongitude: proto.Float64(longitude),
	}
	if name != "" {
		loc.Name = proto.String(name)
	}
	return &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{Chat: chat, Sender: chat},
			ID:            "MSG-LOC",
			PushName:      "Tester",
			Timestamp:     time.Now(),
		},
		Message: &waE2E.Message{LocationMessage: loc},
	}
}

// TestPool_HandleEvent_ExtractsInboundLocation is a regression test: before
// this, an inbound LocationMessage fell through every text-extraction
// branch and produced an event with empty Data["text"] — the event still
// fired (so a conversation still got created), but the coordinates were
// silently dropped.
func TestPool_HandleEvent_ExtractsInboundLocation(t *testing.T) {
	p := newTestPool(t, true, true)
	d := fakeDevice("111", true)

	dmJID := types.NewJID("222", types.DefaultUserServer)
	p.handleEvent(d, locationMessageEvent(dmJID, 33.5731, -7.5898, "Casablanca"))

	select {
	case evt := <-p.events:
		data, ok := evt.Data.(map[string]interface{})
		if !ok {
			t.Fatalf("expected Data to be a map, got %T", evt.Data)
		}
		if data["text"] != "Casablanca" {
			t.Fatalf(`data["text"] = %v, want "Casablanca"`, data["text"])
		}
	default:
		t.Fatal("expected a message.received event, got none")
	}
}

func TestPool_HandleEvent_ExtractsInboundLocationWithoutName(t *testing.T) {
	p := newTestPool(t, true, true)
	d := fakeDevice("111", true)

	dmJID := types.NewJID("222", types.DefaultUserServer)
	p.handleEvent(d, locationMessageEvent(dmJID, 33.5731, -7.5898, ""))

	select {
	case evt := <-p.events:
		data, _ := evt.Data.(map[string]interface{})
		if data["text"] != "33.573100, -7.589800" {
			t.Fatalf(`data["text"] = %v, want the raw coordinates when no name is set`, data["text"])
		}
	default:
		t.Fatal("expected a message.received event, got none")
	}
}

func imageMessageEvent(chat types.JID, caption, mimeType string) *events.Message {
	return &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{Chat: chat, Sender: chat},
			ID:            "MSG-IMG",
			PushName:      "Tester",
			Timestamp:     time.Now(),
		},
		Message: &waE2E.Message{
			ImageMessage: &waE2E.ImageMessage{
				Caption:  proto.String(caption),
				Mimetype: proto.String(mimeType),
				// No DirectPath: this exercises the "right message type ->
				// right Data fields" boundary described in
				// extractInboundMedia's doc comment — Download needs a live
				// connection to actually fetch bytes, which a unit test
				// can't drive, so mediaBytes is expected to come back nil
				// here (fail-open), same as a real download failure would.
			},
		},
	}
}

// TestPool_HandleEvent_ExtractsInboundMedia is a regression test: before
// this, an inbound image/video/audio/document message fell through every
// text-extraction branch and produced an event with empty Data — the
// caption and the fact that media was even attached were both silently
// dropped.
func TestPool_HandleEvent_ExtractsInboundMedia(t *testing.T) {
	p := newTestPool(t, true, true)
	d := fakeDevice("111", true)

	dmJID := types.NewJID("222", types.DefaultUserServer)
	p.handleEvent(d, imageMessageEvent(dmJID, "look at this", "image/jpeg"))

	select {
	case evt := <-p.events:
		data, ok := evt.Data.(map[string]interface{})
		if !ok {
			t.Fatalf("expected Data to be a map, got %T", evt.Data)
		}
		if data["text"] != "look at this" {
			t.Fatalf(`data["text"] = %v, want the caption`, data["text"])
		}
		if data["mediaKind"] != "image" {
			t.Fatalf(`data["mediaKind"] = %v, want "image"`, data["mediaKind"])
		}
		if data["mediaMimeType"] != "image/jpeg" {
			t.Fatalf(`data["mediaMimeType"] = %v, want "image/jpeg"`, data["mediaMimeType"])
		}
		if _, present := data["mediaBytes"]; present {
			t.Fatalf(`data["mediaBytes"] = %v, want absent (no live connection to download from)`, data["mediaBytes"])
		}
	default:
		t.Fatal("expected a message.received event, got none")
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

// TestPool_HandleEvent_DropsFromMeMessages is a regression test: WhatsApp
// multi-device syncs every sent message (including ones sent from the
// phone itself, or in a "Message Yourself" self-chat) to every linked
// device as an *events.Message with IsFromMe=true. This must never be
// treated as an inbound customer message — see the IsFromMe check in
// handleEvent.
func TestPool_HandleEvent_DropsFromMeMessages(t *testing.T) {
	p := newTestPool(t, true, true)
	d := fakeDevice("111", true)

	dmJID := types.NewJID("222", types.DefaultUserServer)
	p.handleEvent(d, fromMeMessageEvent(dmJID, "this is an echo of my own message"))

	select {
	case evt := <-p.events:
		t.Fatalf("expected an IsFromMe message to be dropped, got event %+v", evt)
	default:
	}
}

// TestPool_HandleEvent_DropsProtocolMessages is a regression test: WhatsApp
// sends a burst of protocol-level messages (history sync notifications,
// app-state key shares, ...) right after a device links, wrapped in the
// same *events.Message type as real user messages. These must never turn
// into an EventMessageReceived (empty-content noise flooding webhooks and
// conversations) — see the ProtocolMessage check in handleEvent.
func TestPool_HandleEvent_DropsProtocolMessages(t *testing.T) {
	p := newTestPool(t, true, true)
	d := fakeDevice("111", true)

	dmJID := types.NewJID("222", types.DefaultUserServer)
	p.handleEvent(d, protocolMessageEvent(dmJID))

	select {
	case evt := <-p.events:
		t.Fatalf("expected a protocol message to be dropped, got event %+v", evt)
	default:
	}
}

// TestPool_HandleEvent_PrefersSenderAltWhenAddressedByLID is a regression
// test: when WhatsApp addresses the sender by LID (server "lid") instead of
// their real phone number, Event.Phone must be the real phone number
// (SenderAlt), not the LID's numeric ID — otherwise a reply lands under an
// unrelated "phone number" that doesn't match anything (OTP sends, outbound
// messages, or a human operator's expectations).
func TestPool_HandleEvent_PrefersSenderAltWhenAddressedByLID(t *testing.T) {
	p := newTestPool(t, true, true)
	d := fakeDevice("111", true)

	lid := types.NewJID("153876514742433", types.HiddenUserServer)
	realPhone := types.NewJID("212600000000", types.DefaultUserServer)
	p.handleEvent(d, lidMessageEvent(lid, realPhone, "salut"))

	select {
	case evt := <-p.events:
		if evt.Phone != "212600000000" {
			t.Fatalf("evt.Phone = %q, want the real phone number 212600000000 (from SenderAlt), not the LID", evt.Phone)
		}
	default:
		t.Fatal("expected a message.received event")
	}
}

// TestFetchMediaData_RejectsNonSuccessStatus is a regression test: a URL
// that returns an error page (403, 404, expired link, ...) must fail the
// send loudly instead of silently uploading the error page's bytes as if
// they were the real image/video/audio/document — a send that returns a
// message id and looks successful while the recipient gets nothing usable.
func TestFetchMediaData_RejectsNonSuccessStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("<html>access denied</html>"))
	}))
	defer srv.Close()

	if _, err := fetchMediaData(context.Background(), srv.URL, ""); err == nil {
		t.Fatal("expected an error for a non-2xx media URL response")
	}
}

func TestFetchMediaData_AcceptsSuccessStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fake-image-bytes"))
	}))
	defer srv.Close()

	data, err := fetchMediaData(context.Background(), srv.URL, "")
	if err != nil {
		t.Fatalf("fetchMediaData: %v", err)
	}
	if string(data) != "fake-image-bytes" {
		t.Fatalf("data = %q, want fake-image-bytes", data)
	}
}

// TestMediaKindHandlers_RoutesToCorrectMessageType is a regression test for
// media type parity with Cloud (see CloudClient.SendMedia): each MediaKind
// must upload with the matching whatsmeow.MediaType and build the matching
// waE2E.Message field — no live device/network needed, this is pure
// routing logic shared by Pool.SendMedia and MeowClient.SendMedia.
func TestMediaKindHandlers_RoutesToCorrectMessageType(t *testing.T) {
	fakeUpload := whatsmeow.UploadResponse{
		URL: "https://example.com/blob", DirectPath: "/blob",
		MediaKey: []byte("key"), FileEncSHA256: []byte("enc"), FileSHA256: []byte("sha"),
	}

	cases := []struct {
		kind         MediaKind
		wantUpload   whatsmeow.MediaType
		checkMessage func(t *testing.T, msg *waE2E.Message)
	}{
		{MediaKindImage, whatsmeow.MediaImage, func(t *testing.T, msg *waE2E.Message) {
			if msg.ImageMessage == nil {
				t.Fatal("expected ImageMessage to be set")
			}
			if msg.ImageMessage.GetCaption() != "cap" {
				t.Fatalf("caption = %q, want cap", msg.ImageMessage.GetCaption())
			}
		}},
		{MediaKindVideo, whatsmeow.MediaVideo, func(t *testing.T, msg *waE2E.Message) {
			if msg.VideoMessage == nil {
				t.Fatal("expected VideoMessage to be set")
			}
			if msg.VideoMessage.GetCaption() != "cap" {
				t.Fatalf("caption = %q, want cap", msg.VideoMessage.GetCaption())
			}
		}},
		{MediaKindAudio, whatsmeow.MediaAudio, func(t *testing.T, msg *waE2E.Message) {
			if msg.AudioMessage == nil {
				t.Fatal("expected AudioMessage to be set")
			}
			if msg.AudioMessage.GetURL() != fakeUpload.URL {
				t.Fatalf("audio URL = %q, want %q", msg.AudioMessage.GetURL(), fakeUpload.URL)
			}
		}},
		{MediaKindDocument, whatsmeow.MediaDocument, func(t *testing.T, msg *waE2E.Message) {
			if msg.DocumentMessage == nil {
				t.Fatal("expected DocumentMessage to be set")
			}
			if msg.DocumentMessage.GetFileName() != "report.pdf" {
				t.Fatalf("filename = %q, want report.pdf", msg.DocumentMessage.GetFileName())
			}
			if msg.DocumentMessage.GetCaption() != "cap" {
				t.Fatalf("caption = %q, want cap", msg.DocumentMessage.GetCaption())
			}
		}},
		{"", whatsmeow.MediaImage, func(t *testing.T, msg *waE2E.Message) {
			if msg.ImageMessage == nil {
				t.Fatal("expected an empty/unrecognized kind to default to ImageMessage")
			}
		}},
	}

	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			mediaType, build := mediaKindHandlers(tc.kind)
			if mediaType != tc.wantUpload {
				t.Fatalf("upload MediaType = %v, want %v", mediaType, tc.wantUpload)
			}
			msg := build(fakeUpload, "application/octet-stream", 1234, "cap", "report.pdf")
			tc.checkMessage(t, msg)
		})
	}
}
