package output

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/example/codemap/internal/analyze"
	"github.com/example/codemap/internal/model"
)

//go:embed viewer/index.html
var viewerHTML string

//go:embed viewer/app.js
var viewerJS string

//go:embed viewer/styles.css
var viewerCSS string

func WriteAll(result model.Result, opts analyze.Options) error {
	outDir := opts.OutDir
	if !filepath.IsAbs(outDir) {
		outDir = filepath.Join(opts.RepoRoot, outDir)
	}

	graphDir := filepath.Join(outDir, "graphs")
	viewerDir := filepath.Join(outDir, "viewer")

	if err := os.MkdirAll(graphDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(viewerDir, 0o755); err != nil {
		return err
	}

	manifestPath := filepath.Join(outDir, "manifest.json")
	if err := writeJSON(manifestPath, result.Manifest); err != nil {
		return err
	}

	for _, graph := range result.Graphs {
		if err := writeJSON(filepath.Join(graphDir, graph.Project.ID+".json"), graph); err != nil {
			return err
		}
	}

	if err := os.WriteFile(filepath.Join(viewerDir, "index.html"), []byte(viewerHTML), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(viewerDir, "data.js"), []byte(viewerDataJS(result)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(viewerDir, "app.js"), []byte(viewerJS), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(viewerDir, "styles.css"), []byte(viewerCSS), 0o644); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(outDir, "summary.json"), mustJSON(result.Summary), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "summary.md"), []byte(markdownSummary(result)), 0o644); err != nil {
		return err
	}

	return nil
}

func viewerDataJS(result model.Result) string {
	type payload struct {
		Manifest model.Manifest         `json:"manifest"`
		Graphs   map[string]model.Graph `json:"graphs"`
	}

	graphs := make(map[string]model.Graph, len(result.Graphs))
	for _, graph := range result.Graphs {
		graphs[graph.Project.ID] = graph
	}

	data := payload{
		Manifest: result.Manifest,
		Graphs:   graphs,
	}

	raw, _ := json.Marshal(data)
	return "window.CODEMAP_DATA = " + string(raw) + ";\n"
}

func writeJSON(path string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return os.WriteFile(path, payload, 0o644)
}

func mustJSON(value any) []byte {
	payload, _ := json.MarshalIndent(value, "", "  ")
	return append(payload, '\n')
}

func markdownSummary(result model.Result) string {
	var builder strings.Builder
	builder.WriteString("# CodeMap Summary\n\n")
	builder.WriteString(fmt.Sprintf("- Projects detected: %d\n", result.Manifest.ProjectCount))
	builder.WriteString(fmt.Sprintf("- Graphs generated: %d\n", result.Manifest.GraphCount))
	if len(result.Manifest.ChangedFiles) > 0 {
		builder.WriteString(fmt.Sprintf("- Changed files highlighted: %d\n", len(result.Manifest.ChangedFiles)))
	}
	builder.WriteString("\n## Projects\n\n")
	for _, project := range result.Manifest.Projects {
		builder.WriteString(fmt.Sprintf("- `%s` at `%s` with %d nodes / %d edges\n", project.Name, project.Root, project.NodeCount, project.EdgeCount))
	}
	builder.WriteString("\nOpen `viewer/index.html` from the artifact to inspect the interactive graph.\n")
	return builder.String()
}
