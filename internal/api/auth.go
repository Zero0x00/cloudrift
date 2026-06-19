package api

import (
	"crypto/subtle"
	"net/http"
)

// APITokenEnv is the env var that, when set, gates the entire server (API + static UI) behind
// HTTP Basic auth. Browser-native: the browser prompts and same-origin fetch() inherits the
// credentials, so the SPA needs no changes. The token is the password (username is ignored).
const APITokenEnv = "CLOUDRIFT_API_TOKEN"

// basicAuthGate requires HTTP Basic auth whose password equals token (constant-time compared).
func basicAuthGate(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, pass, ok := r.BasicAuth()
			if !ok || subtle.ConstantTimeCompare([]byte(pass), []byte(token)) != 1 {
				w.Header().Set("WWW-Authenticate", `Basic realm="cloudrift", charset="UTF-8"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
