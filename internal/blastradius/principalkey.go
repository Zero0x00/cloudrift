package blastradius

import (
	"encoding/base64"
	"strings"
)

const principalKeySep = string('\x1e')

// EncodePrincipalID builds a URL-safe id for principal-root blast radius routes.
// Format: arn<sep>type<sep>account_id
func EncodePrincipalID(arn, pType, accountID string) string {
	s := strings.TrimSpace(arn) + principalKeySep + strings.TrimSpace(pType) + principalKeySep + strings.TrimSpace(accountID)
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

// DecodePrincipalID inverts EncodePrincipalID; returns ok=false on invalid payload.
func DecodePrincipalID(id string) (arn, pType, accountID string, ok bool) {
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(id))
	if err != nil {
		return "", "", "", false
	}
	parts := strings.SplitN(string(b), principalKeySep, 3)
	if len(parts) != 3 {
		return "", "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2]), true
}
