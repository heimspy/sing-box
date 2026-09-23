package proxy

import (
	"bytes"
	"context"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gobwas/ws"
	"github.com/sagernet/sing-box/service/heimspyinspector/internal/ipc"
)

func TestWebSocketWriterSerializesMessages(t *testing.T) {
	var output bytes.Buffer
	writer := &websocketWriter{target: &output}
	var workers sync.WaitGroup
	for i := range 40 {
		workers.Go(func() {
			data := bytes.Repeat([]byte{byte(i)}, 64)
			err := writer.message(data, true, []ws.Header{
				{Masked: true, Mask: [4]byte{1, 2, 3, 4}, Length: 32},
				{Masked: true, Mask: [4]byte{5, 6, 7, 8}, Length: 32},
			})
			if err != nil {
				t.Errorf("write: %v", err)
			}
		})
	}
	workers.Wait()
	seen := make(map[byte]bool)
	for range 40 {
		var payload []byte
		for part := range 2 {
			header, err := ws.ReadHeader(&output)
			if err != nil {
				t.Fatal(err)
			}
			if !header.Masked || header.Fin != (part == 1) {
				t.Fatalf("bad header: %+v", header)
			}
			if part == 0 && header.OpCode != ws.OpBinary || part == 1 && header.OpCode != ws.OpContinuation {
				t.Fatalf("interleaved frames: %+v", header)
			}
			data := make([]byte, header.Length)
			if _, err := io.ReadFull(&output, data); err != nil {
				t.Fatal(err)
			}
			ws.Cipher(data, header.Mask, 0)
			payload = append(payload, data...)
		}
		if !bytes.Equal(payload, bytes.Repeat(payload[:1], 64)) {
			t.Fatal("interleaved payload")
		}
		seen[payload[0]] = true
	}
	if len(seen) != 40 || output.Len() != 0 {
		t.Fatal("lost or extra messages")
	}
}

func TestWebSocketWriterEmptyMessage(t *testing.T) {
	var output bytes.Buffer
	writer := &websocketWriter{target: &output}
	if err := writer.message(nil, false, []ws.Header{{Fin: true, Masked: true}}); err != nil {
		t.Fatal(err)
	}
	header, err := ws.ReadHeader(&output)
	if err != nil {
		t.Fatal(err)
	}
	if !header.Fin || !header.Masked || header.Length != 0 || header.OpCode != ws.OpText {
		t.Fatalf("bad empty message: %+v", header)
	}
}

func TestWebSocketResendCancellationAndBusy(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	var output bytes.Buffer
	r := &runtime{peer: ipc.NewPeer(&output, cancel)}
	sessionCtx, session := r.session(context.Background())
	defer session.cancel()
	upstream, remote := net.Pipe()
	defer remote.Close()
	body := newWebSocketBody(sessionCtx, upstream, session)
	defer body.Close()
	// No peer consumes the write: cancellation must unblock it and release the worker.
	r.sendWebSocket(ctx, ipc.Message{ID: "one", Session: session.id, Data: ipc.Bytes("hello")})
	r.sendWebSocket(ctx, ipc.Message{ID: "two", Session: session.id, Data: ipc.Bytes("hello")})
	done := make(chan struct{})
	go func() { session.workers.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("resend did not unblock")
	}
	results := make(map[string]string)
	for output.Len() > 0 {
		msg, err := ipc.ReadMessage(&output)
		if err != nil {
			t.Fatal(err)
		}
		if msg.Type == "websocket-send-result" {
			results[msg.ID] = msg.Error
		}
	}
	if results["one"] == "" {
		t.Fatal("stalled send reported success")
	}
	if !strings.Contains(results["two"], "already in progress") {
		t.Fatalf("busy reply: %q", results["two"])
	}
	if !body.closing.Load() || session.resending.Load() {
		t.Fatal("resend resources not released")
	}
}
