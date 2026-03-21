package analyze

import (
	"path/filepath"
	"regexp"
	"strings"
)

type languageSpec struct {
	name         string
	imports      []*regexp.Regexp
	symbols      []*regexp.Regexp
	commentStart []string
}

var languageByExt = map[string]languageSpec{
	".go": {
		name: "go",
		imports: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*import\s+"([^"]+)"`),
			regexp.MustCompile(`(?m)^\s*import\s+\(([^)]+)\)`),
		},
		symbols: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*type\s+([A-Z_a-z][\w]*)\s+(?:struct|interface)`),
			regexp.MustCompile(`(?m)^\s*func\s+(?:\([^)]+\)\s*)?([A-Z_a-z][\w]*)\s*\(`),
		},
		commentStart: []string{"//"},
	},
	".ts":  tsSpec(),
	".tsx": tsSpec(),
	".js":  tsSpec(),
	".jsx": tsSpec(),
	".swift": {
		name: "swift",
		imports: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*import\s+([A-Z_a-z][\w\.]*)`),
		},
		symbols: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*(?:class|struct|protocol|enum)\s+([A-Z_a-z][\w]*)`),
			regexp.MustCompile(`(?m)^\s*func\s+([A-Z_a-z][\w]*)\s*\(`),
		},
		commentStart: []string{"//"},
	},
	".py": {
		name: "python",
		imports: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*import\s+([A-Z_a-z][\w\.]*)`),
			regexp.MustCompile(`(?m)^\s*from\s+([A-Z_a-z][\w\.]*)\s+import\s+`),
		},
		symbols: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*class\s+([A-Z_a-z][\w]*)`),
			regexp.MustCompile(`(?m)^\s*def\s+([A-Z_a-z][\w]*)\s*\(`),
		},
		commentStart: []string{"#"},
	},
	".java":  jvmSpec("java"),
	".kt":    jvmSpec("kotlin"),
	".scala": jvmSpec("scala"),
	".cs": {
		name: "csharp",
		imports: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*using\s+([A-Z_a-z][\w\.]*)\s*;`),
		},
		symbols: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*(?:public|private|protected|internal)?\s*(?:sealed\s+|abstract\s+)?(?:class|interface|record|struct|enum)\s+([A-Z_a-z][\w]*)`),
			regexp.MustCompile(`(?m)^\s*(?:public|private|protected|internal)?\s*(?:static\s+)?[\w<>\[\], ?]+\s+([A-Z_a-z][\w]*)\s*\(`),
		},
		commentStart: []string{"//"},
	},
	".rs": {
		name: "rust",
		imports: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*use\s+([^;]+);`),
			regexp.MustCompile(`(?m)^\s*mod\s+([A-Z_a-z][\w]*)`),
		},
		symbols: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*(?:struct|enum|trait)\s+([A-Z_a-z][\w]*)`),
			regexp.MustCompile(`(?m)^\s*fn\s+([A-Z_a-z][\w]*)\s*\(`),
		},
		commentStart: []string{"//"},
	},
	".rb": {
		name: "ruby",
		imports: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*require(?:_relative)?\s+["']([^"']+)["']`),
		},
		symbols: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*class\s+([A-Z_a-z][\w:]*)`),
			regexp.MustCompile(`(?m)^\s*module\s+([A-Z_a-z][\w:]*)`),
			regexp.MustCompile(`(?m)^\s*def\s+([A-Z_a-z][\w!?=]*)`),
		},
		commentStart: []string{"#"},
	},
	".php": {
		name: "php",
		imports: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*(?:require|require_once|include|include_once)\s*\(?["']([^"']+)["']`),
			regexp.MustCompile(`(?m)^\s*use\s+([A-Z_a-z\\][\w\\]+)`),
		},
		symbols: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*(?:final\s+|abstract\s+)?class\s+([A-Z_a-z][\w]*)`),
			regexp.MustCompile(`(?m)^\s*interface\s+([A-Z_a-z][\w]*)`),
			regexp.MustCompile(`(?m)^\s*function\s+([A-Z_a-z][\w]*)\s*\(`),
		},
		commentStart: []string{"//", "#"},
	},
	".dart": {
		name: "dart",
		imports: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*import\s+['"]([^'"]+)['"]`),
			regexp.MustCompile(`(?m)^\s*export\s+['"]([^'"]+)['"]`),
		},
		symbols: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*(?:abstract\s+)?class\s+([A-Z_a-z][\w]*)`),
			regexp.MustCompile(`(?m)^\s*(?:mixin|enum|extension)\s+([A-Z_a-z][\w]*)`),
			regexp.MustCompile(`(?m)^\s*(?:[A-Z_a-z_<>\?\[\]]+\s+)?([A-Z_a-z][\w]*)\s*\(`),
		},
		commentStart: []string{"//"},
	},
	".c":   cSpec("c"),
	".h":   cSpec("c"),
	".cpp": cSpec("cpp"),
	".cc":  cSpec("cpp"),
	".cxx": cSpec("cpp"),
	".hpp": cSpec("cpp"),
	".hh":  cSpec("cpp"),
	".m":   cSpec("objective-c"),
	".mm":  cSpec("objective-cpp"),
	".sh": {
		name: "shell",
		imports: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*\.\s+([^\s]+)`),
			regexp.MustCompile(`(?m)^\s*source\s+([^\s]+)`),
		},
		symbols: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*([A-Z_a-z][\w]*)\s*\(\)\s*\{`),
		},
		commentStart: []string{"#"},
	},
}

func tsSpec() languageSpec {
	return languageSpec{
		name: "typescript",
		imports: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*import(?:.+from\s+)?["']([^"']+)["']`),
			regexp.MustCompile(`(?m)^\s*export\s+\*\s+from\s+["']([^"']+)["']`),
			regexp.MustCompile(`(?m)^\s*const\s+\w+\s*=\s*require\(["']([^"']+)["']\)`),
		},
		symbols: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*export\s+(?:default\s+)?class\s+([A-Z_a-z][\w]*)`),
			regexp.MustCompile(`(?m)^\s*(?:export\s+)?class\s+([A-Z_a-z][\w]*)`),
			regexp.MustCompile(`(?m)^\s*(?:export\s+)?interface\s+([A-Z_a-z][\w]*)`),
			regexp.MustCompile(`(?m)^\s*(?:export\s+)?function\s+([A-Z_a-z][\w]*)\s*\(`),
		},
		commentStart: []string{"//"},
	}
}

func jvmSpec(name string) languageSpec {
	return languageSpec{
		name: name,
		imports: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*import\s+([A-Z_a-z][\w\.]*)`),
		},
		symbols: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*(?:public\s+)?(?:class|interface|enum|record)\s+([A-Z_a-z][\w]*)`),
			regexp.MustCompile(`(?m)^\s*(?:public|private|protected)?\s*(?:static\s+)?[\w<>\[\], ?]+\s+([A-Z_a-z][\w]*)\s*\(`),
		},
		commentStart: []string{"//"},
	}
}

func cSpec(name string) languageSpec {
	return languageSpec{
		name: name,
		imports: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*#include\s+[<"]([^>"]+)[>"]`),
		},
		symbols: []*regexp.Regexp{
			regexp.MustCompile(`(?m)^\s*(?:class|struct|enum)\s+([A-Z_a-z][\w]*)`),
			regexp.MustCompile(`(?m)^\s*(?:static\s+)?[\w\*\s]+?\s+([A-Z_a-z][\w]*)\s*\([^;]*\)\s*\{`),
		},
		commentStart: []string{"//"},
	}
}

func detectLanguage(path string) (languageSpec, bool) {
	ext := strings.ToLower(filepath.Ext(path))
	spec, ok := languageByExt[ext]
	return spec, ok
}
