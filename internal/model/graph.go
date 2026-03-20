package model

type NodeType string

const (
	NodeProject   NodeType = "project"
	NodeDirectory NodeType = "directory"
	NodeFile      NodeType = "file"
	NodeModule    NodeType = "module"
	NodeSymbol    NodeType = "symbol"
)

type EdgeType string

const (
	EdgeContains     EdgeType = "contains"
	EdgeDependsOn    EdgeType = "depends_on"
	EdgeReferences   EdgeType = "references"
	EdgeCrossProject EdgeType = "cross_project"
)

type Node struct {
	ID        string            `json:"id"`
	Label     string            `json:"label"`
	Type      NodeType          `json:"type"`
	Language  string            `json:"language,omitempty"`
	Path      string            `json:"path,omitempty"`
	ProjectID string            `json:"projectId,omitempty"`
	Changed   bool              `json:"changed,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type Edge struct {
	ID           string            `json:"id"`
	Source       string            `json:"source"`
	Target       string            `json:"target"`
	Type         EdgeType          `json:"type"`
	ProjectID    string            `json:"projectId,omitempty"`
	CrossProject bool              `json:"crossProject,omitempty"`
	Changed      bool              `json:"changed,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type Project struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Root         string   `json:"root"`
	LanguageHint string   `json:"languageHint,omitempty"`
	Manifest     string   `json:"manifest,omitempty"`
	Tags         []string `json:"tags,omitempty"`
}

type Graph struct {
	Project Project `json:"project"`
	Nodes   []Node  `json:"nodes"`
	Edges   []Edge  `json:"edges"`
}

type Manifest struct {
	RepoRoot     string         `json:"repoRoot"`
	GeneratedAt  string         `json:"generatedAt"`
	ProjectCount int            `json:"projectCount"`
	GraphCount   int            `json:"graphCount"`
	Projects     []ManifestItem `json:"projects"`
	ChangedFiles []string       `json:"changedFiles,omitempty"`
}

type ManifestItem struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Root         string `json:"root"`
	GraphPath    string `json:"graphPath"`
	NodeCount    int    `json:"nodeCount"`
	EdgeCount    int    `json:"edgeCount"`
	ChangedNodes int    `json:"changedNodes"`
}

type Summary struct {
	ProjectCount int      `json:"projectCount"`
	GraphCount   int      `json:"graphCount"`
	ArtifactPath string   `json:"artifactPath"`
	ProjectNames []string `json:"projectNames"`
}

type Result struct {
	Manifest Manifest `json:"manifest"`
	Graphs   []Graph  `json:"graphs"`
	Summary  Summary  `json:"summary"`
}
