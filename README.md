# CodeMap

<p align="center">
  <img src="https://img.shields.io/badge/Multi--Language-Architecture%20Visualizer-0f766e?style=for-the-badge" alt="CodeMap" />
  <img src="https://img.shields.io/badge/PR-First-c2410c?style=for-the-badge" alt="PR First" />
  <img src="https://img.shields.io/badge/GitHub-Actions-1f2937?style=for-the-badge&logo=githubactions&logoColor=white" alt="GitHub Actions" />
</p>

<p align="center">
  See project boundaries, dependencies, and architecture drift directly in pull requests.
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Works%20Across-Go%20%C2%B7%20TS%20%C2%B7%20Swift%20%C2%B7%20Python%20%C2%B7%20Java%20%C2%B7%20Kotlin%20%C2%B7%20Rust%20%C2%B7%20C%23%20%C2%B7%20PHP%20%C2%B7%20Dart%20%C2%B7%20C%2FC%2B%2B-334155?style=flat-square" alt="Broad stack support" />
</p>

## Demo

[![Demo Preview](./docs/demo-poster.png)](./docs/demo.mp4)

<p align="center">
  <a href="./docs/demo.mp4"><img src="https://img.shields.io/badge/Watch-Demo-111827?style=for-the-badge&logo=video&logoColor=white" alt="Watch Demo" /></a>
</p>


## Tech Stack

<p align="center">
  <img src="https://skillicons.dev/icons?i=go,ts,react,github" alt="Tech Stack" />
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Analyzer-Go-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go" />
  <img src="https://img.shields.io/badge/Tooling-TypeScript-3178C6?style=flat-square&logo=typescript&logoColor=white" alt="TypeScript" />
  <img src="https://img.shields.io/badge/Viewer-Web_UI-61DAFB?style=flat-square&logo=react&logoColor=0b0f19" alt="React" />
  <img src="https://img.shields.io/badge/CI-GitHub_Actions-2088FF?style=flat-square&logo=githubactions&logoColor=white" alt="GitHub Actions" />
</p>

## Overview

CodeMap is a multi-language architecture visualizer for pull requests.

It scans a repository, detects bounded projects across many tech stacks, and generates interactive dependency maps so developers can understand what connects to what before merging code.

Built with Go, TypeScript, and a lightweight browser-based viewer, CodeMap is designed to run inside GitHub Actions and publish a PR-friendly artifact.

It is intentionally focused on script and source-code connections, not every file in the repository. Assets like images and other non-code files are ignored because they do not help explain execution or dependency flow.

## What It Does

- Detects separate projects inside a repo from common manifests like `package.json`, `go.mod`, `Package.swift`, `Podfile`, `pyproject.toml`, `pom.xml`, `Cargo.toml`, `composer.json`, `pubspec.yaml`, `.csproj`, `.sln`, and more
- Generates one graph per detected project instead of forcing the whole repository into a single unreadable map
- Focuses on analyzable source files and scripts rather than assets, screenshots, images, or general repo files
- Produces a language-agnostic dependency graph centered on script files only
- Hides framework and package noise like `Foundation`, `React`, or other external modules that do not help explain repo-internal flow
- Excludes test files by default so runtime/app flow stays clearer
- Adds shallow relationship edges such as script-to-script `depends_on` and cross-project links
- Highlights PR-touched files so reviewers can inspect the changed neighborhood first
- Ships an interactive viewer with pan, zoom, drag, node selection, neighbor highlighting, and an inspector panel

## Why This Exists

Large repositories hide architecture drift regardless of stack.

Developers often merge code without seeing:

- what module is now depending on what
- whether one module, service, app, or package is reaching into another layer it should not
- whether a new change crosses a boundary it should not
- how a single PR changes the dependency shape of a project

CodeMap makes those relationships visible directly in the PR workflow.

## Quick Start

From the CodeMap repo root:

```bash
go run ./cmd/codemap analyze --repo . --out ./codemap-out
```

For another repository, point `--repo` at that project, regardless of whether it is web, mobile, backend, native, or a mixed monorepo:

```bash
go run ./cmd/codemap analyze --repo /path/to/your/repo --out ./codemap-out
```

If `--repo` is an absolute path and `--out` is relative, the output is written inside the scanned repo.

Open the generated viewer:

- `codemap-out/viewer/index.html`

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

Generated output:

- `manifest.json`
- `graphs/<project-id>.json`
- `viewer/index.html`
- `summary.json`
- `summary.md`

## Viewer

The generated viewer is offline-friendly and artifact-friendly.

It currently supports:

- multiple project graphs
- script-focused dependency graphs instead of whole-repo file inventories
- pan and zoom
- dragging nodes
- selecting nodes
- highlighting connected nodes and lines
- inspector details for the selected node
- changed-node filtering

The checked-in `web/src` folder mirrors the viewer direction in TypeScript for future iteration. The emitted viewer bundle used by the CLI is embedded by the Go binary so GitHub Actions can produce a self-contained artifact without an additional frontend build step.

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

## Current Scope

CodeMap is intentionally broad-first for v1:

- support many languages and stacks with graceful degradation
- prioritize useful dependency maps over deep semantic perfection
- keep the output PR-friendly and easy to open

That means some stacks currently get shallower graphs than others, but the architecture map is still useful for spotting project boundaries and dependency drift across web, mobile, backend, native, and mixed-language repositories.

## Roadmap

- Replace the heuristic parser with deeper Tree-sitter-backed parsers per language
- Add richer reference and call edges
- Add architecture rules and optional CI enforcement
- Improve layout and graph navigation for large repos
- Persist custom node positions in the viewer

## Connect

<p align="center">
  <a href="https://www.linkedin.com/in/javidshaikh/">
    <img src="https://img.shields.io/badge/LinkedIn-Javid%20Shaikh-0A66C2?style=for-the-badge&logo=linkedin&logoColor=white" alt="LinkedIn" />
  </a>
  <a href="https://x.com/ByJavidShaikh">
    <img src="https://img.shields.io/badge/X-@ByJavidShaikh-111827?style=for-the-badge&logo=x&logoColor=white" alt="X" />
  </a>
</p>
