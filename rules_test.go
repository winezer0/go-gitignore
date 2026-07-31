package gitignore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRuleSetGitSemantics(t *testing.T) {
	tests := []struct {
		name    string
		lines   []string
		path    string
		isDir   bool
		want    bool
		pattern string
	}{
		{name: "comment ignored", lines: []string{"# comment"}, path: "# comment"},
		{name: "UTF-8 BOM", lines: []string{"\ufeff*.cache"}, path: "data.cache", want: true, pattern: "*.cache"},
		{name: "escaped comment", lines: []string{`\#literal`}, path: "#literal", want: true, pattern: `\#literal`},
		{name: "escaped negation", lines: []string{`\!literal`}, path: "!literal", want: true, pattern: `\!literal`},
		{name: "trailing spaces ignored", lines: []string{"*.log   "}, path: "app.log", want: true, pattern: "*.log   "},
		{name: "escaped trailing space retained", lines: []string{`name\ `}, path: "name ", want: true, pattern: `name\ `},
		{name: "question wildcard", lines: []string{"temp?"}, path: "temp1", want: true, pattern: "temp?"},
		{name: "question does not cross slash", lines: []string{"temp?"}, path: "temp/1"},
		{name: "character class", lines: []string{"file[0-9].txt"}, path: "file7.txt", want: true, pattern: "file[0-9].txt"},
		{name: "negative class", lines: []string{"file[!0-9].txt"}, path: "filex.txt", want: true, pattern: "file[!0-9].txt"},
		{name: "POSIX digit class", lines: []string{"file[[:digit:]].txt"}, path: "file7.txt", want: true, pattern: "file[[:digit:]].txt"},
		{name: "POSIX digit class rejects letter", lines: []string{"file[[:digit:]].txt"}, path: "filex.txt"},
		{name: "class contains closing bracket", lines: []string{"file[]a].txt"}, path: "file].txt", want: true, pattern: "file[]a].txt"},
		{name: "root anchored", lines: []string{"/root.txt"}, path: "root.txt", want: true, pattern: "/root.txt"},
		{name: "root anchored rejects nested", lines: []string{"/root.txt"}, path: "nested/root.txt"},
		{name: "slash anchors pattern", lines: []string{"docs/*.md"}, path: "docs/readme.md", want: true, pattern: "docs/*.md"},
		{name: "slash pattern rejects deeper", lines: []string{"docs/*.md"}, path: "x/docs/readme.md"},
		{name: "leading double star", lines: []string{"**/cache"}, path: "a/b/cache", want: true, pattern: "**/cache"},
		{name: "middle double star zero dirs", lines: []string{"a/**/b"}, path: "a/b", want: true, pattern: "a/**/b"},
		{name: "middle double star many dirs", lines: []string{"a/**/b"}, path: "a/x/y/b", want: true, pattern: "a/**/b"},
		{name: "trailing double star contents", lines: []string{"abc/**"}, path: "abc/file", want: true, pattern: "abc/**"},
		{name: "trailing double star excludes directory itself", lines: []string{"abc/**"}, path: "abc", isDir: true},
		{name: "embedded double star stays in component", lines: []string{"ab**cd"}, path: "ab/x/cd"},
		{name: "embedded double star matches component", lines: []string{"ab**cd"}, path: "abXYZcd", want: true, pattern: "ab**cd"},
		{name: "triple star does not cross directory", lines: []string{"a/***/b"}, path: "a/x/y/b"},
		{name: "triple star matches one component", lines: []string{"a/***/b"}, path: "a/x/b", want: true, pattern: "a/***/b"},
		{name: "directory pattern matches directory", lines: []string{"build/"}, path: "build", isDir: true, want: true, pattern: "build/"},
		{name: "directory pattern rejects file", lines: []string{"build/"}, path: "build"},
		{name: "directory pattern matches descendant", lines: []string{"build/"}, path: "build/output.o", want: true, pattern: "build/"},
		{name: "negation re-includes file", lines: []string{"*.tmp", "!keep.tmp"}, path: "keep.tmp", pattern: "!keep.tmp"},
		{name: "parent exclusion blocks negation", lines: []string{"vendor/", "!vendor/keep.txt"}, path: "vendor/keep.txt", want: true, pattern: "vendor/"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rules, err := CompileLines("memory.ignore", test.lines)
			if err != nil {
				t.Fatalf("CompileLines() error = %v", err)
			}
			ignored, detail := rules.MatchesPathHow(test.path, test.isDir)
			if ignored != test.want {
				t.Fatalf("MatchesPathHow() ignored = %t, want %t", ignored, test.want)
			}
			gotPattern := ""
			if detail != nil {
				gotPattern = detail.Pattern
			}
			if gotPattern != test.pattern {
				t.Fatalf("MatchesPathHow() pattern = %q, want %q", gotPattern, test.pattern)
			}
		})
	}
}

func TestRuleDiagnostics(t *testing.T) {
	rules, err := CompileLines("memory.ignore", []string{"*.tmp", "!keep.tmp"})
	if err != nil {
		t.Fatalf("CompileLines() error = %v", err)
	}
	ignored, detail := rules.MatchesPathHow("keep.tmp", false)
	if ignored || detail == nil {
		t.Fatalf("MatchesPathHow() = %t, %#v", ignored, detail)
	}
	want := MatchDetail{Ignored: false, SourcePath: "memory.ignore", Line: 2, Pattern: "!keep.tmp", Negated: true}
	if *detail != want {
		t.Fatalf("MatchesPathHow() detail = %#v, want %#v", *detail, want)
	}
}

func TestCompileFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".gitignore")
	if err := os.WriteFile(path, []byte("# comment\r\n*.log\r\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	rules, err := CompileFile(path)
	if err != nil {
		t.Fatalf("CompileFile() error = %v", err)
	}
	ignored, detail := rules.MatchesPathHow("app.log", false)
	if !ignored || detail == nil || detail.SourcePath != path || detail.Line != 2 {
		t.Fatalf("MatchesPathHow() = %t, %#v", ignored, detail)
	}
}

func TestCompileErrors(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
	}{
		{name: "unterminated class", lines: []string{"file[abc"}},
		{name: "invalid class range", lines: []string{"file[z-a]"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := CompileLines("memory.ignore", test.lines); err == nil {
				t.Fatal("CompileLines() error = nil, want error")
			}
		})
	}
	if _, err := CompileFile(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("CompileFile() error = nil, want error")
	}
}
