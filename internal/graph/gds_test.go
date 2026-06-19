package graph

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// condExecer fails on the first statement whose cypher contains failOn (to simulate a
// missing GDS procedure); otherwise records calls and succeeds.
type condExecer struct {
	failOn string
	calls  []string
}

func (c *condExecer) Run(_ context.Context, cypher string, _ map[string]any) error {
	c.calls = append(c.calls, cypher)
	if c.failOn != "" && strings.Contains(cypher, c.failOn) {
		return errors.New("Neo.ClientError.Procedure.ProcedureNotFound: There is no procedure with the name `gds.graph.project` registered for this database instance.")
	}
	return nil
}

func TestRunGDS_Success(t *testing.T) {
	ex := &condExecer{}
	if err := RunGDS(context.Background(), ex, "20260101-120000"); err != nil {
		t.Fatalf("RunGDS: unexpected error: %v", err)
	}
	joined := strings.Join(ex.calls, "\n")
	for _, want := range []string{"gds.graph.project", "gds.louvain.write", "gds.pageRank.write", "gds.graph.drop"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected %s to be called; calls:\n%s", want, joined)
		}
	}
	// community + centrality are the written properties.
	if !strings.Contains(joined, "community") || !strings.Contains(joined, "centrality") {
		t.Errorf("expected community + centrality write properties; calls:\n%s", joined)
	}
}

func TestRunGDS_DegradesWhenGDSMissing(t *testing.T) {
	ex := &condExecer{failOn: "gds.graph.project"}
	err := RunGDS(context.Background(), ex, "scan-1")
	if err == nil {
		t.Fatal("expected an error when GDS project fails")
	}
	if !errors.Is(err, ErrGDSUnavailable) {
		t.Fatalf("expected ErrGDSUnavailable, got %v", err)
	}
}

func TestGDSGraphNameSanitizes(t *testing.T) {
	if got := gdsGraphName("2026-05-14T10:00:00Z"); strings.ContainsAny(got, "-:") {
		t.Errorf("graph name not sanitized: %s", got)
	}
	if got := gdsGraphName(""); got == "" || !strings.HasPrefix(got, "cloudrift_") {
		t.Errorf("empty scan id should still yield a valid name, got %q", got)
	}
}
