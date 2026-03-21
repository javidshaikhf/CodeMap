package analyze

import (
	"path/filepath"
	"testing"

	"github.com/example/codemap/internal/config"
)

func TestRunBuildsGraphsForMonorepo(t *testing.T) {
	repo := filepath.Join("..", "..", "testdata", "monorepo")
	result, err := Run(Options{
		RepoRoot:     repo,
		OutDir:       filepath.Join(repo, "codemap-out"),
		ChangedFiles: []string{"frontend/src/App.tsx"},
		MaxFiles:     200,
		Config:       config.Config{},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Manifest.ProjectCount != 2 {
		t.Fatalf("expected 2 projects, got %d", result.Manifest.ProjectCount)
	}
	if len(result.Graphs) != 2 {
		t.Fatalf("expected 2 graphs, got %d", len(result.Graphs))
	}
	if result.Manifest.Projects[1].ChangedNodes == 0 {
		t.Fatalf("expected changed nodes to be highlighted")
	}
	for _, graph := range result.Graphs {
		for _, node := range graph.Nodes {
			if node.Path == "frontend/package.json" || node.Path == "backend/go.mod" {
				t.Fatalf("expected manifest files to be excluded from script graph, found %s", node.Path)
			}
			if node.Type == "directory" || node.Type == "symbol" {
				t.Fatalf("expected only script/module nodes, found %s", node.Type)
			}
		}
	}
}

func TestRunSupportsFallbackSingleProject(t *testing.T) {
	repo := filepath.Join("..", "..", "testdata", "swift-app")
	result, err := Run(Options{
		RepoRoot: repo,
		OutDir:   filepath.Join(repo, "codemap-out"),
		MaxFiles: 200,
		Config:   config.Config{},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Manifest.ProjectCount != 1 {
		t.Fatalf("expected 1 project, got %d", result.Manifest.ProjectCount)
	}
	if len(result.Graphs[0].Nodes) == 0 || len(result.Graphs[0].Edges) == 0 {
		t.Fatalf("expected graph nodes and edges to be generated")
	}
}
