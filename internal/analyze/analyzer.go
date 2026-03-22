package analyze

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/example/codemap/internal/config"
	"github.com/example/codemap/internal/detect"
	"github.com/example/codemap/internal/model"
)

type Options struct {
	RepoRoot     string
	OutDir       string
	Include      []string
	Exclude      []string
	ChangedFiles []string
	MaxFiles     int
	Config       config.Config
}

type parser interface {
	parse(project model.Project, relPath string, content []byte) fileParseResult
}

type heuristicParser struct{}

type fileParseResult struct {
	language string
	imports  []string
	symbols  []string
	refs     []string
}

type analyzedFile struct {
	projectRel string
	repoRel    string
	language   string
	imports    []string
	symbols    []string
	refs       []string
	changed    bool
}

func Run(opts Options) (model.Result, error) {
	projects, err := detect.Discover(opts.RepoRoot, opts.Config)
	if err != nil {
		return model.Result{}, err
	}

	changedSet := toSet(opts.ChangedFiles)
	parser := heuristicParser{}

	graphs := make([]model.Graph, 0, len(projects))
	manifestItems := make([]model.ManifestItem, 0, len(projects))
	projectNames := make([]string, 0, len(projects))

	for _, project := range projects {
		graph, err := buildProjectGraph(opts, project, changedSet, parser, projects)
		if err != nil {
			return model.Result{}, err
		}
		graphs = append(graphs, graph)
		projectNames = append(projectNames, project.Name)

		changedNodes := 0
		for _, node := range graph.Nodes {
			if node.Changed {
				changedNodes++
			}
		}

		manifestItems = append(manifestItems, model.ManifestItem{
			ID:           project.ID,
			Name:         project.Name,
			Root:         project.Root,
			GraphPath:    filepath.ToSlash(filepath.Join("graphs", project.ID+".json")),
			NodeCount:    len(graph.Nodes),
			EdgeCount:    len(graph.Edges),
			ChangedNodes: changedNodes,
		})
	}

	manifest := model.Manifest{
		RepoRoot:     opts.RepoRoot,
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		ProjectCount: len(projects),
		GraphCount:   len(graphs),
		Projects:     manifestItems,
		ChangedFiles: normalizePaths(opts.ChangedFiles),
	}

	return model.Result{
		Manifest: manifest,
		Graphs:   graphs,
		Summary: model.Summary{
			ProjectCount: len(projects),
			GraphCount:   len(graphs),
			ArtifactPath: opts.OutDir,
			ProjectNames: projectNames,
		},
	}, nil
}

func buildProjectGraph(opts Options, project model.Project, changedSet map[string]bool, parser parser, allProjects []model.Project) (model.Graph, error) {
	rootAbs := opts.RepoRoot
	if project.Root != "." {
		rootAbs = filepath.Join(opts.RepoRoot, filepath.FromSlash(project.Root))
	}

	graph := model.Graph{
		Project: project,
	}
	files, err := collectProjectFiles(opts, project, rootAbs, changedSet, parser)
	if err != nil {
		return model.Graph{}, err
	}

	nodes := map[string]model.Node{}
	edges := map[string]model.Edge{}

	filesByRepoPath := make(map[string]analyzedFile, len(files))
	for _, file := range files {
		filesByRepoPath[file.repoRel] = file
		nodes[nodeID(project.ID, "file", file.repoRel)] = model.Node{
			ID:        nodeID(project.ID, "file", file.repoRel),
			Label:     filepath.Base(file.repoRel),
			Type:      model.NodeFile,
			Language:  file.language,
			Path:      file.repoRel,
			ProjectID: project.ID,
			Changed:   file.changed,
		}
	}

	declaredBySymbol := make(map[string][]analyzedFile)
	for _, file := range files {
		for _, symbol := range file.symbols {
			declaredBySymbol[symbol] = append(declaredBySymbol[symbol], file)
		}
	}

	for _, file := range files {
		fileNodeID := nodeID(project.ID, "file", file.repoRel)
		for _, imp := range file.imports {
			if target, ok := resolveInternalImport(file, imp, project, filesByRepoPath); ok {
				targetNodeID := nodeID(project.ID, "file", target.repoRel)
				edges[edgeID(project.ID, fileNodeID, targetNodeID, model.EdgeDependsOn)] = model.Edge{
					ID:        edgeID(project.ID, fileNodeID, targetNodeID, model.EdgeDependsOn),
					Source:    fileNodeID,
					Target:    targetNodeID,
					Type:      model.EdgeDependsOn,
					ProjectID: project.ID,
					Changed:   file.changed || target.changed,
					Metadata: map[string]string{
						"import": imp,
						"kind":   "script",
					},
				}
				continue
			}

			if crossProject := findCrossProject(imp, project, allProjects); crossProject != nil {
				edges[edgeID(project.ID, fileNodeID, crossProject.ID, model.EdgeCrossProject)] = model.Edge{
					ID:           edgeID(project.ID, fileNodeID, crossProject.ID, model.EdgeCrossProject),
					Source:       fileNodeID,
					Target:       crossProject.ID,
					Type:         model.EdgeCrossProject,
					ProjectID:    project.ID,
					CrossProject: true,
					Changed:      file.changed,
					Metadata: map[string]string{
						"import": imp,
					},
				}
			}
		}

		for _, ref := range file.refs {
			targets := declaredBySymbol[ref]
			if len(targets) != 1 {
				continue
			}
			target := targets[0]
			if target.repoRel == file.repoRel {
				continue
			}
			targetNodeID := nodeID(project.ID, "file", target.repoRel)
			id := edgeID(project.ID, fileNodeID, targetNodeID, model.EdgeReferences)
			if _, exists := edges[id]; exists {
				continue
			}
			edges[id] = model.Edge{
				ID:        id,
				Source:    fileNodeID,
				Target:    targetNodeID,
				Type:      model.EdgeReferences,
				ProjectID: project.ID,
				Changed:   file.changed || target.changed,
				Metadata: map[string]string{
					"symbol": ref,
					"kind":   "symbol",
				},
			}
		}
	}

	pruneDisconnectedNodes(nodes, edges)
	graph.Nodes = mapValues(nodes)
	graph.Edges = mapEdgeValues(edges)
	sort.Slice(graph.Nodes, func(i, j int) bool { return graph.Nodes[i].ID < graph.Nodes[j].ID })
	sort.Slice(graph.Edges, func(i, j int) bool { return graph.Edges[i].ID < graph.Edges[j].ID })
	return graph, nil
}

func collectProjectFiles(opts Options, project model.Project, rootAbs string, changedSet map[string]bool, parser parser) ([]analyzedFile, error) {
	filesSeen := 0
	var files []analyzedFile

	err := filepath.WalkDir(rootAbs, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if shouldSkipProjectPath(rootAbs, path, d) {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}

		relFromProject, err := filepath.Rel(rootAbs, path)
		if err != nil {
			return err
		}
		relFromProject = filepath.ToSlash(relFromProject)
		relFromRepo := relFromProject
		if project.Root != "." {
			relFromRepo = filepath.ToSlash(filepath.Join(project.Root, relFromProject))
		}

		if !shouldInclude(relFromRepo, opts.Include, opts.Exclude, opts.Config.ExcludeGlobs) {
			return nil
		}
		if isTestLikePath(relFromRepo) {
			return nil
		}

		if _, ok := detectLanguage(relFromRepo); !ok {
			return nil
		}

		filesSeen++
		if opts.MaxFiles > 0 && filesSeen > opts.MaxFiles {
			return fmt.Errorf("project %s exceeds max-files limit (%d)", project.Name, opts.MaxFiles)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		parsed := parser.parse(project, relFromRepo, content)
		if parsed.language == "" {
			return nil
		}

		files = append(files, analyzedFile{
			projectRel: relFromProject,
			repoRel:    relFromRepo,
			language:   parsed.language,
			imports:    parsed.imports,
			symbols:    parsed.symbols,
			refs:       parsed.refs,
			changed:    changedSet[relFromRepo],
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

func (heuristicParser) parse(project model.Project, relPath string, content []byte) fileParseResult {
	spec, ok := detectLanguage(relPath)
	if !ok {
		return fileParseResult{}
	}

	text := string(content)
	imports := parseImports(spec, text)
	symbols := parseSymbols(spec, text)
	return fileParseResult{
		language: spec.name,
		imports:  dedupe(imports),
		symbols:  dedupe(symbols),
		refs:     dedupe(parseRefs(text)),
	}
}

func parseImports(spec languageSpec, text string) []string {
	var imports []string
	for _, pattern := range spec.imports {
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			value := strings.TrimSpace(match[1])
			if value == "" {
				continue
			}
			if strings.Contains(value, "\n") {
				imports = append(imports, splitBlockImports(value)...)
				continue
			}
			imports = append(imports, normalizeImport(value)...)
		}
	}
	return imports
}

func parseSymbols(spec languageSpec, text string) []string {
	var symbols []string
	for _, pattern := range spec.symbols {
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			symbols = append(symbols, strings.TrimSpace(match[1]))
		}
	}
	return symbols
}

func parseRefs(text string) []string {
	identifierPattern := regexp.MustCompile(`\b[A-Z_a-z][A-Z_a-z0-9_]*\b`)
	return identifierPattern.FindAllString(text, -1)
}

func splitBlockImports(block string) []string {
	lines := strings.Split(block, "\n")
	var imports []string
	lineImport := regexp.MustCompile(`"([^"]+)"`)
	for _, line := range lines {
		match := lineImport.FindStringSubmatch(line)
		if len(match) == 2 {
			imports = append(imports, match[1])
		}
	}
	return imports
}

func normalizeImport(value string) []string {
	value = strings.Trim(value, "{} ")
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' '
	})
	var out []string
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			out = append(out, strings.TrimSpace(part))
		}
	}
	if len(out) == 0 {
		return []string{value}
	}
	return out
}

func addDirectoryNodes(project model.Project, projectRelPath, repoRelPath string, changedSet map[string]bool, nodes map[string]model.Node, edges map[string]model.Edge) {
	dir := filepath.ToSlash(filepath.Dir(projectRelPath))
	if dir == "." {
		return
	}

	segments := strings.Split(dir, "/")
	currentPath := ""
	parentID := project.ID
	for _, segment := range segments {
		if currentPath == "" {
			currentPath = segment
		} else {
			currentPath += "/" + segment
		}
		repoPath := currentPath
		if project.Root != "." {
			repoPath = filepath.ToSlash(filepath.Join(project.Root, currentPath))
		}
		dirID := nodeID(project.ID, "dir", repoPath)
		nodes[dirID] = model.Node{
			ID:        dirID,
			Label:     segment,
			Type:      model.NodeDirectory,
			Path:      repoPath,
			ProjectID: project.ID,
			Changed:   changedSet[repoPath] || changedSet[repoRelPath],
		}
		edges[edgeID(project.ID, parentID, dirID, model.EdgeContains)] = model.Edge{
			ID:        edgeID(project.ID, parentID, dirID, model.EdgeContains),
			Source:    parentID,
			Target:    dirID,
			Type:      model.EdgeContains,
			ProjectID: project.ID,
			Changed:   changedSet[repoPath],
		}
		parentID = dirID
	}

	fileNodeID := nodeID(project.ID, "file", repoRelPath)
	edges[edgeID(project.ID, parentID, fileNodeID, model.EdgeContains)] = model.Edge{
		ID:        edgeID(project.ID, parentID, fileNodeID, model.EdgeContains),
		Source:    parentID,
		Target:    fileNodeID,
		Type:      model.EdgeContains,
		ProjectID: project.ID,
		Changed:   changedSet[repoRelPath],
	}
}

func shouldSkipProjectPath(rootAbs, path string, d os.DirEntry) bool {
	if !d.IsDir() {
		return false
	}
	name := d.Name()
	if name == ".git" || name == "node_modules" || name == "vendor" || name == "dist" || name == "build" || name == ".next" || name == "codemap-out" {
		return true
	}
	rel, _ := filepath.Rel(rootAbs, path)
	return strings.HasPrefix(rel, ".git/")
}

func shouldInclude(path string, include, exclude, configExcludes []string) bool {
	path = filepath.ToSlash(path)
	if len(include) > 0 {
		matched := false
		for _, prefix := range include {
			prefix = filepath.ToSlash(strings.TrimSpace(prefix))
			if prefix != "" && (path == prefix || strings.HasPrefix(path, prefix+"/")) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	for _, prefix := range append(exclude, configExcludes...) {
		prefix = filepath.ToSlash(strings.TrimSpace(prefix))
		if prefix != "" && (path == prefix || strings.HasPrefix(path, prefix+"/")) {
			return false
		}
	}

	return true
}

func isTestLikePath(path string) bool {
	path = filepath.ToSlash(strings.ToLower(path))
	base := filepath.Base(path)

	if strings.Contains(path, "/test/") || strings.Contains(path, "/tests/") || strings.Contains(path, "/__tests__/") {
		return true
	}

	switch {
	case strings.HasSuffix(base, "_test.go"):
		return true
	case strings.HasSuffix(base, "tests.swift"), strings.HasSuffix(base, "test.swift"):
		return true
	case strings.HasSuffix(base, "tests.java"), strings.HasSuffix(base, "test.java"):
		return true
	case strings.HasSuffix(base, "tests.kt"), strings.HasSuffix(base, "test.kt"):
		return true
	case strings.HasSuffix(base, "tests.cs"), strings.HasSuffix(base, "test.cs"):
		return true
	case strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py"):
		return true
	case strings.HasSuffix(base, "_test.py"):
		return true
	case strings.HasSuffix(base, ".test.ts"), strings.HasSuffix(base, ".spec.ts"), strings.HasSuffix(base, ".test.tsx"), strings.HasSuffix(base, ".spec.tsx"):
		return true
	case strings.HasSuffix(base, ".test.js"), strings.HasSuffix(base, ".spec.js"), strings.HasSuffix(base, ".test.jsx"), strings.HasSuffix(base, ".spec.jsx"):
		return true
	case strings.HasSuffix(base, "test.php"), strings.HasSuffix(base, "tests.php"):
		return true
	default:
		return false
	}
}

func findCrossProject(importValue string, current model.Project, projects []model.Project) *model.Project {
	normalized := filepath.ToSlash(importValue)
	for _, project := range projects {
		if project.ID == current.ID || project.Root == "." {
			continue
		}
		base := filepath.Base(project.Root)
		if normalized == project.Root || strings.Contains(normalized, project.Root) || strings.Contains(normalized, base) {
			projectCopy := project
			return &projectCopy
		}
	}
	return nil
}

func toSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[filepath.ToSlash(value)] = true
	}
	return set
}

func normalizePaths(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, filepath.ToSlash(value))
		}
	}
	sort.Strings(out)
	return out
}

func dedupe(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func resolveInternalImport(source analyzedFile, importValue string, project model.Project, filesByRepoPath map[string]analyzedFile) (analyzedFile, bool) {
	if importValue == "" {
		return analyzedFile{}, false
	}

	baseRepoDir := filepath.ToSlash(filepath.Dir(source.repoRel))
	baseCandidate := filepath.ToSlash(filepath.Clean(filepath.Join(baseRepoDir, importValue)))
	for _, candidate := range importCandidates(baseCandidate) {
		if target, ok := filesByRepoPath[candidate]; ok {
			return target, true
		}
	}

	if project.Root != "." {
		projectBase := filepath.ToSlash(filepath.Clean(filepath.Join(project.Root, importValue)))
		for _, candidate := range importCandidates(projectBase) {
			if target, ok := filesByRepoPath[candidate]; ok {
				return target, true
			}
		}
	}

	return analyzedFile{}, false
}

func importCandidates(base string) []string {
	base = filepath.ToSlash(base)
	candidates := []string{base}
	if ext := filepath.Ext(base); ext != "" {
		return dedupe(candidates)
	}

	for ext := range languageByExt {
		candidates = append(candidates, base+ext)
		candidates = append(candidates, filepath.ToSlash(filepath.Join(base, "index"+ext)))
		candidates = append(candidates, filepath.ToSlash(filepath.Join(base, "__init__"+ext)))
	}
	return dedupe(candidates)
}

func pruneDisconnectedNodes(nodes map[string]model.Node, edges map[string]model.Edge) {
	if len(edges) == 0 {
		return
	}

	connected := map[string]bool{}
	for _, edge := range edges {
		connected[edge.Source] = true
		connected[edge.Target] = true
	}

	for id, node := range nodes {
		if node.Changed || connected[id] {
			continue
		}
		delete(nodes, id)
	}
}

func mapValues(values map[string]model.Node) []model.Node {
	out := make([]model.Node, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

func mapEdgeValues(values map[string]model.Edge) []model.Edge {
	out := make([]model.Edge, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

func nodeID(projectID, kind, value string) string {
	return fmt.Sprintf("%s:%s:%s", projectID, kind, hash(value))
}

func edgeID(projectID, source, target string, typ model.EdgeType) string {
	return fmt.Sprintf("%s:%s:%s:%s", projectID, hash(source), hash(target), typ)
}

func hash(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:8])
}

func languageFromPath(path string) string {
	if spec, ok := detectLanguage(path); ok {
		return spec.name
	}
	return ""
}
