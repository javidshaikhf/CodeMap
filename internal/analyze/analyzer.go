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
		Nodes: []model.Node{{
			ID:    project.ID,
			Label: project.Name,
			Type:  model.NodeProject,
			Path:  project.Root,
		}},
	}

	nodes := map[string]model.Node{
		project.ID: graph.Nodes[0],
	}
	edges := map[string]model.Edge{}
	filesSeen := 0

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

		filesSeen++
		if opts.MaxFiles > 0 && filesSeen > opts.MaxFiles {
			return fmt.Errorf("project %s exceeds max-files limit (%d)", project.Name, opts.MaxFiles)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		fileNode := model.Node{
			ID:        nodeID(project.ID, "file", relFromRepo),
			Label:     filepath.Base(path),
			Type:      model.NodeFile,
			Language:  languageFromPath(path),
			Path:      relFromRepo,
			ProjectID: project.ID,
			Changed:   changedSet[relFromRepo],
		}
		nodes[fileNode.ID] = fileNode
		if filepath.ToSlash(filepath.Dir(relFromProject)) == "." {
			edges[edgeID(project.ID, project.ID, fileNode.ID, model.EdgeContains)] = model.Edge{
				ID:        edgeID(project.ID, project.ID, fileNode.ID, model.EdgeContains),
				Source:    project.ID,
				Target:    fileNode.ID,
				Type:      model.EdgeContains,
				ProjectID: project.ID,
				Changed:   fileNode.Changed,
			}
		}

		addDirectoryNodes(project, relFromProject, relFromRepo, changedSet, nodes, edges)

		parsed := parser.parse(project, relFromRepo, content)
		if parsed.language != "" {
			fileNode.Language = parsed.language
			nodes[fileNode.ID] = fileNode
		}

		for _, imp := range parsed.imports {
			moduleID := nodeID(project.ID, "module", imp)
			nodes[moduleID] = model.Node{
				ID:        moduleID,
				Label:     imp,
				Type:      model.NodeModule,
				Language:  parsed.language,
				Path:      imp,
				ProjectID: project.ID,
				Changed:   fileNode.Changed,
			}
			edges[edgeID(project.ID, fileNode.ID, moduleID, model.EdgeDependsOn)] = model.Edge{
				ID:        edgeID(project.ID, fileNode.ID, moduleID, model.EdgeDependsOn),
				Source:    fileNode.ID,
				Target:    moduleID,
				Type:      model.EdgeDependsOn,
				ProjectID: project.ID,
				Changed:   fileNode.Changed,
			}
			if crossProject := findCrossProject(imp, project, allProjects); crossProject != nil {
				edges[edgeID(project.ID, fileNode.ID, crossProject.ID, model.EdgeCrossProject)] = model.Edge{
					ID:           edgeID(project.ID, fileNode.ID, crossProject.ID, model.EdgeCrossProject),
					Source:       fileNode.ID,
					Target:       crossProject.ID,
					Type:         model.EdgeCrossProject,
					ProjectID:    project.ID,
					CrossProject: true,
					Changed:      fileNode.Changed,
					Metadata: map[string]string{
						"import": imp,
					},
				}
			}
		}

		for _, symbol := range parsed.symbols {
			symbolID := nodeID(project.ID, "symbol", relFromRepo+":"+symbol)
			nodes[symbolID] = model.Node{
				ID:        symbolID,
				Label:     symbol,
				Type:      model.NodeSymbol,
				Language:  parsed.language,
				Path:      relFromRepo,
				ProjectID: project.ID,
				Changed:   fileNode.Changed,
			}
			edges[edgeID(project.ID, fileNode.ID, symbolID, model.EdgeContains)] = model.Edge{
				ID:        edgeID(project.ID, fileNode.ID, symbolID, model.EdgeContains),
				Source:    fileNode.ID,
				Target:    symbolID,
				Type:      model.EdgeContains,
				ProjectID: project.ID,
				Changed:   fileNode.Changed,
			}
		}

		return nil
	})
	if err != nil {
		return model.Graph{}, err
	}

	graph.Nodes = mapValues(nodes)
	graph.Edges = mapEdgeValues(edges)
	sort.Slice(graph.Nodes, func(i, j int) bool { return graph.Nodes[i].ID < graph.Nodes[j].ID })
	sort.Slice(graph.Edges, func(i, j int) bool { return graph.Edges[i].ID < graph.Edges[j].ID })
	return graph, nil
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
