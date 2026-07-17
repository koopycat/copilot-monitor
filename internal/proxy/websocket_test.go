package proxy

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"copilot-monitoring/internal/log"
	"copilot-monitoring/internal/policy"
	"copilot-monitoring/internal/store"
)

func TestWSFrameRoundTripPreservesMaskingFragmentationAndRSV(t *testing.T) {
	want := wsFrame{
		fin:     false,
		rsv:     0x40,
		opcode:  wsTextFrame,
		masked:  true,
		maskKey: [4]byte{1, 2, 3, 4},
		payload: []byte("fragment"),
	}

	var encoded bytes.Buffer
	if err := writeWSFrame(&encoded, want); err != nil {
		t.Fatal(err)
	}
	got, err := readWSFrame(bytes.NewReader(encoded.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if got.fin != want.fin || got.rsv != want.rsv || got.opcode != want.opcode || got.masked != want.masked || got.maskKey != want.maskKey || !bytes.Equal(got.payload, want.payload) {
		t.Fatalf("frame = %#v, want %#v", got, want)
	}

	var relayed bytes.Buffer
	if err := writeWSFrame(&relayed, got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(relayed.Bytes(), encoded.Bytes()) {
		t.Fatalf("relay changed frame bytes:\n got %x\nwant %x", relayed.Bytes(), encoded.Bytes())
	}
}

func TestWebsocketUpstreamTarget(t *testing.T) {
	for _, tt := range []struct {
		upstream   string
		address    string
		serverName string
	}{
		{upstream: "api.githubcopilot.com", address: "api.githubcopilot.com:443", serverName: "api.githubcopilot.com"},
		{upstream: "localhost:8443", address: "localhost:8443", serverName: "localhost"},
	} {
		address, serverName := websocketUpstreamTarget(tt.upstream)
		if address != tt.address || serverName != tt.serverName {
			t.Fatalf("websocketUpstreamTarget(%q) = (%q, %q), want (%q, %q)", tt.upstream, address, serverName, tt.address, tt.serverName)
		}
	}
}

func TestWSClientInspectorBlocksDisallowedModelBeforeForwarding(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	h := NewHandlerWithStore(log.Disabled(), st, "")
	h.SetUpstream("api.githubcopilot.com")
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7733/responses", nil)
	inspector := &wsClientInspector{
		h:               h,
		idBase:          42,
		r:               req,
		activePolicy:    &policy.Policy{Mode: policy.Blocklist, Models: []string{"gpt-4o"}},
		policyAvailable: true,
	}

	input := encodeWSFrames(t, wsFrame{
		fin:     true,
		opcode:  wsTextFrame,
		masked:  true,
		maskKey: [4]byte{1, 2, 3, 4},
		payload: []byte(`{"type":"response.create","response":{"model":"gpt-4o"}}`),
	})
	var upstream bytes.Buffer
	result := inspector.copyInspected(&upstream, bytes.NewReader(input))
	if !result.Blocked {
		t.Fatal("blocked model should stop the relay")
	}
	if upstream.Len() != 0 {
		t.Fatalf("blocked message reached upstream: %x", upstream.Bytes())
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var status int
	var stream bool
	var model string
	if err := db.QueryRowContext(context.Background(), "SELECT status, stream, model FROM requests").Scan(&status, &stream, &model); err != nil {
		t.Fatal(err)
	}
	if status != http.StatusForbidden || !stream || model != "gpt-4o" {
		t.Fatalf("blocked record = status %d stream %t model %q", status, stream, model)
	}

	var client bytes.Buffer
	if err := writeWSModelBlockedClose(&client); err != nil {
		t.Fatal(err)
	}
	closeFrame, err := readWSFrame(bytes.NewReader(client.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if closeFrame.opcode != wsCloseFrame || !closeFrame.fin || closeFrame.masked {
		t.Fatalf("close frame = %#v", closeFrame)
	}
	if code := binary.BigEndian.Uint16(closeFrame.payload[:2]); code != wsPolicyViolationCode {
		t.Fatalf("close code = %d, want %d", code, wsPolicyViolationCode)
	}
	if reason := string(closeFrame.payload[2:]); reason != wsModelBlockedReason {
		t.Fatalf("close reason = %q, want %q", reason, wsModelBlockedReason)
	}
}

func TestWSClientInspectorPreservesAllowedFragmentedFrames(t *testing.T) {
	h := NewHandler(log.Disabled())
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7733/responses", nil)
	inspector := &wsClientInspector{
		h:               h,
		idBase:          7,
		r:               req,
		activePolicy:    &policy.Policy{Mode: policy.Allowlist, Models: []string{"gpt-4o"}},
		policyAvailable: true,
	}
	frames := []wsFrame{
		{fin: false, opcode: wsTextFrame, masked: true, maskKey: [4]byte{1, 1, 1, 1}, payload: []byte(`{"response":{"model":"gpt-`)},
		{fin: true, opcode: wsPingFrame, masked: true, maskKey: [4]byte{2, 2, 2, 2}, payload: []byte("ping")},
		{fin: true, opcode: wsContFrame, masked: true, maskKey: [4]byte{3, 3, 3, 3}, payload: []byte(`4o"}}`)},
	}
	input := encodeWSFrames(t, frames...)
	var upstream bytes.Buffer
	result := inspector.copyInspected(&upstream, bytes.NewReader(input))
	if result.Blocked || result.Stopped {
		t.Fatalf("allowed relay result = %#v", result)
	}
	if !bytes.Equal(upstream.Bytes(), input) {
		t.Fatalf("allowed relay changed frame sequence:\n got %x\nwant %x", upstream.Bytes(), input)
	}
}

func TestWSClientInspectorFailsOpenWithoutModel(t *testing.T) {
	h := NewHandler(log.Disabled())
	inspector := &wsClientInspector{
		h:               h,
		idBase:          9,
		r:               httptest.NewRequest(http.MethodGet, "/responses", nil),
		activePolicy:    &policy.Policy{Mode: policy.Allowlist, Models: []string{"gpt-4o"}},
		policyAvailable: true,
	}
	input := encodeWSFrames(t, wsFrame{
		fin:     true,
		opcode:  wsTextFrame,
		masked:  true,
		maskKey: [4]byte{5, 5, 5, 5},
		payload: []byte(`{"type":"response.create","response":{}}`),
	})
	var upstream bytes.Buffer
	result := inspector.copyInspected(&upstream, bytes.NewReader(input))
	if result.Blocked || result.Stopped || !bytes.Equal(upstream.Bytes(), input) {
		t.Fatalf("empty-model relay result = %#v, output = %x", result, upstream.Bytes())
	}
}

func encodeWSFrames(t *testing.T, frames ...wsFrame) []byte {
	t.Helper()
	var encoded bytes.Buffer
	for _, frame := range frames {
		if err := writeWSFrame(&encoded, frame); err != nil {
			t.Fatal(err)
		}
	}
	return encoded.Bytes()
}
