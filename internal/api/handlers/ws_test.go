package handlers

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func TestScanProgressWSHandshakeAndEvent(t *testing.T) {
	srv := httptest.NewServer(ScanProgressWS())
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	var msg map[string]any
	if err := wsjson.Read(context.Background(), conn, &msg); err != nil {
		t.Fatalf("read json: %v", err)
	}
	if msg["event_type"] != "progress" {
		t.Fatalf("unexpected event_type: %v", msg["event_type"])
	}
}
