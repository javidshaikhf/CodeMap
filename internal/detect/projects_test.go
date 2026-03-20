package detect

import (
	"path/filepath"
	"testing"

	"github.com/example/codemap/internal/config"
)

func TestDiscoverMonorepoProjects(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "monorepo")
	projects, err := Discover(root, config.Config{})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}
	if projects[0].Root != "backend" || projects[1].Root != "frontend" {
		t.Fatalf("unexpected project roots: %+v", projects)
	}
}

func TestDiscoverFallbackRoot(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "single")
	projects, err := Discover(root, config.Config{})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].Root != "." {
		t.Fatalf("expected fallback project at root, got %s", projects[0].Root)
	}
}
