package scanner

import "testing"

func TestCategorizeRecognizesDevServerAndAIAgent(t *testing.T) {
	cat, tool := Categorize("node", "node /app/node_modules/vite/bin/vite.js dev")
	if cat != CategoryDevServer || tool != "Vite" {
		t.Fatalf("expected Vite dev server, got %q / %q", cat, tool)
	}

	cat, tool = Categorize("node", "node /usr/local/bin/pi-coding-agent")
	if cat != CategoryAIAgent || tool != "Claude" {
		t.Fatalf("expected AI agent Claude, got %q / %q", cat, tool)
	}
}
