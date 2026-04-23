package blastradius

import (
	"encoding/base64"
	"strings"
)

const entityKeySep = string('\x1e')

// EncodeExternalEntityID builds a URL-safe id for the triple used by
// internal/api/handlers external entity aggregation: principal × type × external account.
func EncodeExternalEntityID(externalPrincipal, principalType, externalAccountID string) string {
	s := externalPrincipal + entityKeySep + strings.TrimSpace(principalType) + entityKeySep + externalAccountID
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

// DecodeExternalEntityID inverts [EncodeExternalEntityID]; returns false if invalid.
func DecodeExternalEntityID(id string) (ep, pt, ea string, ok bool) {
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(id))
	if err != nil {
		return "", "", "", false
	}
	parts := strings.SplitN(string(b), entityKeySep, 3)
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}
