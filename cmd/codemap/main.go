package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/example/codemap/internal/analyze"
	"github.com/example/codemap/internal/config"
	"github.com/example/codemap/internal/output"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "analyze":
		if err := runAnalyze(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "codemap: %v\n", err)
			os.Exit(1)
		}
	case "version":
		fmt.Println("codemap dev")
	default:
		usage()
		os.Exit(1)
	}
}

func runAnalyze(args []string) error {
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	repo := fs.String("repo", ".", "repository root to scan")
	outDir := fs.String("out", "./codemap-out", "directory to write graphs and viewer assets")
	include := fs.String("include", "", "comma-separated relative path prefixes to include")
	exclude := fs.String("exclude", "", "comma-separated relative path prefixes to exclude")
	configPath := fs.String("config", "", "optional JSON config file for detector overrides")
	changedFiles := fs.String("changed-files", "", "comma-separated changed file paths")
	maxFiles := fs.Int("max-files", 2000, "maximum files per detected project")

	if err := fs.Parse(args); err != nil {
		return err
	}

	absRepo, err := filepath.Abs(*repo)
	if err != nil {
		return err
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	opts := analyze.Options{
		RepoRoot:     absRepo,
		OutDir:       *outDir,
		Include:      splitCSV(*include),
		Exclude:      splitCSV(*exclude),
		ChangedFiles: splitCSV(*changedFiles),
		MaxFiles:     *maxFiles,
		Config:       cfg,
	}

	result, err := analyze.Run(opts)
	if err != nil {
		return err
	}

	return output.WriteAll(result, opts)
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func usage() {
	fmt.Println("usage:")
	fmt.Println("  codemap analyze --repo . --out ./codemap-out")
	fmt.Println("  codemap version")
}
