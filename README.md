# CodeMap

CodeMap is a multi-language repository visualizer built with Go, TypeScript, and React. It detects bounded projects inside a repo, generates one graph per project, and ships a lightweight viewer that can be attached to pull requests in GitHub Actions.

## What v1 does

- Detects projects from common manifests like `package.json`, `go.mod`, `Package.swift`, `pyproject.toml`, `pom.xml`, and `Cargo.toml`
- Produces one graph per detected project instead of collapsing the whole repo into one map
- Builds a language-agnostic graph with `project`, `directory`, `file`, `module`, and `symbol` nodes
- Adds shallow `contains`, `depends_on`, and cross-project edges across many languages
- Highlights changed files from a pull request so developers can inspect the touched area first

## CLI

```bash
go run ./cmd/codemap analyze --repo . --out ./codemap-out
```

Important flags:

- `--include` limits analysis to comma-separated path prefixes
- `--exclude` skips comma-separated path prefixes
- `--config` points to a JSON file with `projectRoots` overrides
- `--changed-files` accepts a comma-separated list of repo-relative paths
- `--max-files` limits file count per project to keep CI safe

The command writes:

- `manifest.json`
- `graphs/<project-id>.json`
- `viewer/index.html`
- `summary.json`
- `summary.md`

## Viewer

The generated viewer lives in `codemap-out/viewer/index.html`.

The checked-in `web/src` folder mirrors the viewer logic in TypeScript for future iteration. The current emitted bundle is a dependency-light static viewer embedded by the Go binary so GitHub Actions can produce an artifact without a separate frontend build step.

## GitHub Actions

Use the composite action from this repo:

```yaml
- uses: ./
  with:
    output-dir: codemap-out
```

The action:

- detects changed files in pull requests
- runs the Go analyzer
- uploads the generated artifact
- posts or updates a PR comment with the summary

## Roadmap

- Replace the heuristic parser with deeper Tree-sitter-backed parsers per language
- Add richer reference and call edges
- Add optional architecture rules and CI enforcement
