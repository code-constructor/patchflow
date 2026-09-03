package review

import (
	"strings"
	"testing"

	"github.com/code-constructor/patchflow/internal/artifact"
)

// TestGeneratePlanUsesStableSemanticBlockIDs protects addressable baseline evidence.
func TestGeneratePlanUsesStableSemanticBlockIDs(t *testing.T) {
	files := []artifact.ChangedFile{
		{Path: "app/models/a_b.go", Status: "modified"},
		{Path: "app/models/a-b.go", Status: "modified"},
	}
	steps := GeneratePlan(files)
	if len(steps) != 1 || len(steps[0].Blocks) != 4 {
		t.Fatalf("unexpected baseline plan: %#v", steps)
	}
	if steps[0].ReviewQuestion == "" || len(steps[0].Attention) == 0 || steps[0].Blocks[3].Type != "takeaway" {
		t.Fatalf("baseline chapter lacks review guidance: %#v", steps[0])
	}
	first := steps[0].Blocks[1].ID
	second := steps[0].Blocks[2].ID
	if first == second || !strings.Contains(first, "app-models-a-b-go") || strings.Contains(first, "diff-1") {
		t.Fatalf("expected distinct semantic IDs, got %q and %q", first, second)
	}

	reversed := GeneratePlan([]artifact.ChangedFile{files[1], files[0]})
	if reversed[0].Blocks[1].ID != first || reversed[0].Blocks[2].ID != second {
		t.Fatalf("block IDs changed with input order: %#v", reversed[0].Blocks)
	}
}

// TestGeneratePlanCollapsesMechanicalEvidence keeps first-pass noise out of the reading path.
func TestGeneratePlanCollapsesMechanicalEvidence(t *testing.T) {
	steps := GeneratePlan([]artifact.ChangedFile{{Path: "vendor/library.min.js", Status: "modified"}})
	if len(steps) != 1 || steps[0].ID != "generated" || len(steps[0].Blocks) != 3 || !steps[0].Blocks[1].Collapsed || steps[0].Blocks[2].Type != "takeaway" {
		t.Fatalf("generated evidence should start collapsed: %#v", steps)
	}
}
