package dashboard

import "embed"

// Dist is the production Vite build embedded into the cloudrift binary.
// The embed path is relative to this file (dashboard/dist).
//
//go:embed dist
var Dist embed.FS
