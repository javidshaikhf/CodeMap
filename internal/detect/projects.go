package detect

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/example/codemap/internal/config"
	"github.com/example/codemap/internal/model"
)

var manifests = map[string]string{
	"package.json":        "javascript",
	"go.mod":              "go",
	"Package.swift":       "swift",
	"Podfile":             "swift",
	"Cartfile":            "swift",
	"Cartfile.resolved":   "swift",
	"pyproject.toml":      "python",
	"requirements.txt":    "python",
	"pom.xml":             "java",
	"build.gradle":        "jvm",
	"build.gradle.kts":    "jvm",
	"settings.gradle":     "jvm",
	"settings.gradle.kts": "jvm",
	"Cargo.toml":          "rust",
	"composer.json":       "php",
	"pubspec.yaml":        "dart",
	"Gemfile":             "ruby",
	"mix.exs":             "elixir",
	"project.clj":         "clojure",
	"CMakeLists.txt":      "cpp",
	"Makefile":            "native",
	"meson.build":         "native",
	"*.xcodeproj":         "swift",
	"*.xcworkspace":       "swift",
	"*.csproj":            "dotnet",
	"*.fsproj":            "dotnet",
	"*.vbproj":            "dotnet",
	"*.sln":               "dotnet",
}

func Discover(repoRoot string, cfg config.Config) ([]model.Project, error) {
	if len(cfg.ProjectRoots) > 0 {
		return discoverFromOverrides(repoRoot, cfg.ProjectRoots), nil
	}

	var projects []model.Project
	seen := map[string]bool{}

	err := filepath.WalkDir(repoRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if shouldSkipDir(repoRoot, path, d) {
			return filepath.SkipDir
		}

		if d.IsDir() && hasManifestDirName(d.Name()) {
			root := filepath.Dir(path)
			lang, _ := matchManifestName(d.Name())
			addProject(&projects, seen, repoRoot, root, d.Name(), lang)
			return filepath.SkipDir
		}

		if d.IsDir() {
			return nil
		}

		lang, ok := matchManifestName(d.Name())
		if !ok {
			return nil
		}

		addProject(&projects, seen, repoRoot, filepath.Dir(path), d.Name(), lang)
		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(projects) == 0 {
		projects = []model.Project{{
			ID:   "project-root",
			Name: filepath.Base(repoRoot),
			Root: ".",
			Tags: []string{"fallback"},
		}}
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Root < projects[j].Root
	})

	return collapseNested(projects), nil
}

func discoverFromOverrides(repoRoot string, roots []string) []model.Project {
	projects := make([]model.Project, 0, len(roots))
	for _, root := range roots {
		clean := filepath.Clean(root)
		name := filepath.Base(clean)
		if clean == "." {
			name = filepath.Base(repoRoot)
		}
		projects = append(projects, model.Project{
			ID:   projectID(clean),
			Name: name,
			Root: clean,
			Tags: []string{"config"},
		})
	}
	return projects
}

func collapseNested(projects []model.Project) []model.Project {
	filtered := make([]model.Project, 0, len(projects))
	for _, project := range projects {
		nested := false
		for _, existing := range filtered {
			if project.Root != existing.Root && strings.HasPrefix(project.Root+"/", existing.Root+"/") {
				nested = true
				break
			}
		}
		if !nested {
			filtered = append(filtered, project)
		}
	}
	return filtered
}

func addProject(projects *[]model.Project, seen map[string]bool, repoRoot, root, manifest, language string) {
	rel := "."
	if root != repoRoot {
		rel, _ = filepath.Rel(repoRoot, root)
	}
	rel = filepath.ToSlash(rel)
	if rel == "" {
		rel = "."
	}
	if seen[rel] {
		return
	}
	seen[rel] = true

	name := filepath.Base(root)
	if rel == "." {
		name = filepath.Base(repoRoot)
	}

	*projects = append(*projects, model.Project{
		ID:           projectID(rel),
		Name:         name,
		Root:         rel,
		Manifest:     manifest,
		LanguageHint: language,
	})
}

func projectID(root string) string {
	replacer := strings.NewReplacer("/", "-", ".", "root", "_", "-")
	id := replacer.Replace(root)
	id = strings.Trim(id, "-")
	if id == "" {
		return "project-root"
	}
	return "project-" + id
}

func shouldSkipDir(repoRoot, path string, d os.DirEntry) bool {
	if !d.IsDir() {
		return false
	}
	name := d.Name()
	if name == ".git" || name == "node_modules" || name == "vendor" || name == ".next" || name == "dist" || name == "build" || name == "codemap-out" {
		return true
	}
	rel, _ := filepath.Rel(repoRoot, path)
	return strings.HasPrefix(rel, ".git/")
}

func hasManifestDirName(name string) bool {
	return strings.HasSuffix(name, ".xcodeproj") || strings.HasSuffix(name, ".xcworkspace")
}

func matchManifestName(name string) (string, bool) {
	if lang, ok := manifests[name]; ok {
		return lang, true
	}

	switch {
	case strings.HasSuffix(name, ".csproj"):
		return manifests["*.csproj"], true
	case strings.HasSuffix(name, ".fsproj"):
		return manifests["*.fsproj"], true
	case strings.HasSuffix(name, ".vbproj"):
		return manifests["*.vbproj"], true
	case strings.HasSuffix(name, ".sln"):
		return manifests["*.sln"], true
	case strings.HasSuffix(name, ".xcodeproj"):
		return manifests["*.xcodeproj"], true
	case strings.HasSuffix(name, ".xcworkspace"):
		return manifests["*.xcworkspace"], true
	default:
		return "", false
	}
}
