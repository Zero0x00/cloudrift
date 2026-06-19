// Package synth adds an optional LLM answer-synthesis layer over retrieval. The query
// path retrieves findings (vector/graph), then — if a synthesis provider is configured
// with an API key — asks an LLM to compose a grounded natural-language answer citing the
// retrieved findings. Without a key it degrades to a no-op (retrieval-only), preserving
// prior behavior.
//
// The Synthesizer interface is pluggable: "anthropic" (Claude) is the operational
// provider today; Gemini/Llama/etc. can be added as additional implementations.
package synth

import (
	"context"
	"strings"

	"github.com/Zero0x00/cloudrift/internal/config"
)

// ContextItem is one retrieved finding handed to the synthesizer as grounding.
type ContextItem struct {
	ID             string
	Title          string
	Severity       string
	Module         string
	Recommendation string
	Detail         string
}

// Result is the outcome of a synthesis attempt.
type Result struct {
	Answer string // synthesized answer text ("" when Used is false)
	Model  string // model that produced the answer
	Used   bool   // true when an LLM actually produced the answer
}

// Synthesizer turns a question + retrieved findings into a grounded answer.
type Synthesizer interface {
	// Available reports whether synthesis will actually run (provider configured + key present).
	Available() bool
	// Synthesize composes an answer grounded in items. Returns Used=false (no error) when
	// unavailable or when there is nothing to ground on.
	Synthesize(ctx context.Context, question string, items []ContextItem) (Result, error)
}

// noop is the retrieval-only fallback used when no provider/key is configured.
type noop struct{}

func (noop) Available() bool { return false }
func (noop) Synthesize(context.Context, string, []ContextItem) (Result, error) {
	return Result{Used: false}, nil
}

// New returns a Synthesizer for the config. It returns the no-op synthesizer (never nil)
// when the provider is unknown/disabled or the API key env var is unset — so callers can
// always call Synthesize and branch on Result.Used.
func New(cfg *config.Config) Synthesizer {
	if cfg == nil {
		return noop{}
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.Synthesis.Provider))
	switch provider {
	case "anthropic", "claude":
		key := apiKeyFromEnv(cfg.Synthesis.APIKeyEnv)
		if key == "" {
			return noop{}
		}
		return newAnthropic(cfg, key)
	default:
		return noop{}
	}
}
