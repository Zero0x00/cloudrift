package handlers

import (
	"net/http"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func ScanProgressWS(control *scanControlCenter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		ctx := r.Context()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				event := control.CurrentProgressEvent()
				if err := wsjson.Write(ctx, conn, event); err != nil {
					return
				}
			}
		}
	}
}
