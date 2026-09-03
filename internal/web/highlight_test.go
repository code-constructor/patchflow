package web

import (
	"strings"
	"testing"
)

// TestHighlightLineDropsErrorTokens ensures context-free JSON fragments render
// without error styling for separators the line-based lexer cannot resolve.
func TestHighlightLineDropsErrorTokens(t *testing.T) {
	output := string(highlightLine("schema.json", `      "type": { "const": "diagram" },`))
	if strings.Contains(output, "background-color:#e3d2d2") || strings.Contains(output, "color:#a61717") {
		t.Fatalf("expected no error styling, got %s", output)
	}
	if !strings.Contains(output, "diagram") {
		t.Fatalf("expected highlighted source, got %s", output)
	}
}
