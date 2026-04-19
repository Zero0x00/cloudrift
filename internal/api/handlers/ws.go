package handlers

import (
	"context"
	"net/http"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"

	"cloudrift/internal/api/schema"
)

func ScanProgressWS() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Restrict cross-origin WS handshakes to loopback dashboard hosts. The progress
		// stream carries no secrets, but wildcard origins are unnecessary attack surface.
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			OriginPatterns: []string{
				"http://localhost:*",
				"https://localhost:*",
				"http://127.0.0.1:*",
				"https://127.0.0.1:*",
			},
		})
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "ok")

		event := schema.ScanProgressEvent{
			EventType:         "progress",
			Stage:             "idle",
			Message:           "scan progress stream is connected",
			CompletedAccounts: 0,
			TotalAccounts:     0,
			Timestamp:         time.Now().UTC(),
		}
		_ = wsjson.Write(context.Background(), conn, event)
	}
}
