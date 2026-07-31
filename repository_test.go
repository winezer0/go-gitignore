package gitignore

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestRepositoryMatcherHierarchy(t *testing.T) {
	root := t.TempDir()
	writeRepositoryFiles(t, root, map[string]string{
		".gitignore":             "*.tmp\nblocked/\n",
		"child/.ignore":          "!keep.tmp\n*.cache\n",
		"child/.rgignore":        "keep.tmp\n",
		"blocked/.gitignore":     "!keep.txt\n",
		"nested/deep/.gitignore": "*.log\n",
	})
	matcher := newTestRepositoryMatcher(t, root, RepositoryConfig{
		IgnoreFileNames: []string{".gitignore", ".ignore", ".rgignore"},
	})
	tests := []struct {
		name       string
		path       string
		isDir      bool
		want       bool
		wantSource string
	}{
		{name: "root rule", path: "file.tmp", want: true, wantSource: ".gitignore"},
		{name: "rgignore overrides ignore", path: "child/keep.tmp", want: true, wantSource: ".rgignore"},
		{name: "nested ignore", path: "nested/deep/app.log", want: true, wantSource: ".gitignore"},
		{name: "parent exclusion blocks child negation", path: "blocked/keep.txt", want: true, wantSource: ".gitignore"},
		{name: "ordinary path", path: "child/main.go"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ignored, detail, err := matcher.MatchesPathHow(filepath.FromSlash(test.path), test.isDir)
			if err != nil {
				t.Fatalf("MatchesPathHow() error = %v", err)
			}
			if ignored != test.want {
				t.Fatalf("MatchesPathHow() ignored = %t, want %t", ignored, test.want)
			}
			if test.wantSource == "" {
				if detail != nil {
					t.Fatalf("MatchesPathHow() detail = %#v, want nil", detail)
				}
				return
			}
			if detail == nil || filepath.Base(detail.SourcePath) != test.wantSource {
				t.Fatalf("MatchesPathHow() detail = %#v", detail)
			}
		})
	}
}

func TestRepositoryMatcherSources(t *testing.T) {
	root := t.TempDir()
	writeRepositoryFiles(t, root, map[string]string{
		".gitignore":        "git.txt\npriority.txt\n",
		".ignore":           "ignore.txt\n!priority.txt\n",
		".rgignore":         "rg.txt\nkeep.txt\n",
		".customignore":     "custom-name.txt\n",
		"custom.ignore":     "!keep.txt\ncustom.txt\n",
		".git/info/exclude": "excluded.txt\n",
	})
	tests := []struct {
		name   string
		config RepositoryConfig
		path   string
		want   bool
	}{
		{name: "gitignore enabled", config: RepositoryConfig{IgnoreFileNames: []string{".gitignore"}}, path: "git.txt", want: true},
		{name: "ignore disabled", config: RepositoryConfig{IgnoreFileNames: []string{".gitignore"}}, path: "ignore.txt"},
		{name: "ignore enabled", config: RepositoryConfig{IgnoreFileNames: []string{".ignore"}}, path: "ignore.txt", want: true},
		{name: "rgignore enabled", config: RepositoryConfig{IgnoreFileNames: []string{".rgignore"}}, path: "rg.txt", want: true},
		{name: "custom file name", config: RepositoryConfig{IgnoreFileNames: []string{".customignore"}}, path: "custom-name.txt", want: true},
		{name: "later file has priority", config: RepositoryConfig{IgnoreFileNames: []string{".gitignore", ".ignore"}}, path: "priority.txt"},
		{name: "git exclude enabled", config: RepositoryConfig{UseGitExclude: true}, path: "excluded.txt", want: true},
		{name: "additional file", config: RepositoryConfig{AdditionalIgnoreFiles: []string{"custom.ignore"}}, path: "custom.txt", want: true},
		{name: "additional file has priority", config: RepositoryConfig{IgnoreFileNames: []string{".rgignore"}, AdditionalIgnoreFiles: []string{"custom.ignore"}}, path: "keep.txt"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			matcher := newTestRepositoryMatcher(t, root, test.config)
			got, err := matcher.Matches(filepath.FromSlash(test.path), false)
			if err != nil {
				t.Fatalf("Matches() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("Matches() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestRepositoryMatcherPaths(t *testing.T) {
	root := t.TempDir()
	writeRepositoryFiles(t, root, map[string]string{".gitignore": "*.log\n"})
	matcher := newTestRepositoryMatcher(t, root, RepositoryConfig{IgnoreFileNames: []string{".gitignore"}})
	absMatch := filepath.Join(root, "app.log")
	outside := filepath.Join(filepath.Dir(root), filepath.Base(root)+"-outside", "app.log")
	tests := []struct {
		name    string
		path    string
		want    bool
		wantErr bool
	}{
		{name: "relative path", path: "app.log", want: true},
		{name: "absolute path", path: absMatch, want: true},
		{name: "parent traversal", path: filepath.Join("..", "app.log"), wantErr: true},
		{name: "sibling prefix", path: outside, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := matcher.Matches(test.path, false)
			if (err != nil) != test.wantErr {
				t.Fatalf("Matches() error = %v, wantErr %t", err, test.wantErr)
			}
			if got != test.want {
				t.Fatalf("Matches() = %t, want %t", got, test.want)
			}
		})
	}
	if matcher.Root() != filepath.Clean(root) {
		t.Fatalf("Root() = %q, want %q", matcher.Root(), filepath.Clean(root))
	}
}

func TestRepositoryMatcherConcurrentReads(t *testing.T) {
	root := t.TempDir()
	writeRepositoryFiles(t, root, map[string]string{".gitignore": "*.tmp\n"})
	matcher := newTestRepositoryMatcher(t, root, RepositoryConfig{IgnoreFileNames: []string{".gitignore"}})
	var group sync.WaitGroup
	errors := make(chan string, 32)
	for index := 0; index < 32; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			ignored, err := matcher.Matches("cache.tmp", false)
			if err != nil || !ignored {
				errors <- "concurrent match failed"
			}
		}()
	}
	group.Wait()
	close(errors)
	for message := range errors {
		t.Fatal(message)
	}
}

func TestRepositoryMatcherErrorsAndDefaults(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file.txt")
	if err := os.WriteFile(file, []byte("data"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	tests := []struct {
		name string
		root string
	}{
		{name: "empty root"},
		{name: "missing root", root: filepath.Join(root, "missing")},
		{name: "file root", root: file},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewRepositoryMatcher(test.root); err == nil {
				t.Fatal("NewRepositoryMatcher() error = nil, want error")
			}
		})
	}
	if _, err := NewRepositoryMatcherWithConfig(root, RepositoryConfig{
		AdditionalIgnoreFiles: []string{"missing.ignore"},
	}); err == nil {
		t.Fatal("NewRepositoryMatcherWithConfig() error = nil, want error")
	}
	config := DefaultRepositoryConfig()
	if !reflect.DeepEqual(config.IgnoreFileNames, []string{".gitignore", ".ignore", ".rgignore"}) ||
		!config.UseGitExclude || !config.UseGlobalGitIgnore {
		t.Fatalf("DefaultRepositoryConfig() = %#v", config)
	}
	var matcher *RepositoryMatcher
	if _, _, err := matcher.MatchesPathHow("file", false); err == nil {
		t.Fatal("nil MatchesPathHow() error = nil, want error")
	}
	if matcher.Root() != "" {
		t.Fatalf("nil Root() = %q, want empty", matcher.Root())
	}
}

func TestIgnoreStackCustomFileNames(t *testing.T) {
	root := t.TempDir()
	writeRepositoryFiles(t, root, map[string]string{
		".gitignore":    "git.txt\n",
		".customignore": "custom.txt\n",
	})
	stack := NewIgnoreStack(false, true, 0)
	stack.IgnoreFileNames = []string{".customignore"}
	if err := stack.Push(root); err != nil {
		t.Fatalf("Push() error = %v", err)
	}
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "custom loaded", path: "custom.txt", want: true},
		{name: "default disabled", path: "git.txt"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := stack.IsIgnored(filepath.Join(root, test.path), false)
			if err != nil {
				t.Fatalf("IsIgnored() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("IsIgnored() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestRepositoryMatcherRejectsInvalidFileNames(t *testing.T) {
	root := t.TempDir()
	tests := []string{"", ".", "..", filepath.Join("config", "ignore")}
	for _, name := range tests {
		t.Run(strings.ReplaceAll(name, string(filepath.Separator), "_"), func(t *testing.T) {
			_, err := NewRepositoryMatcherWithConfig(root, RepositoryConfig{IgnoreFileNames: []string{name}})
			if err == nil {
				t.Fatalf("NewRepositoryMatcherWithConfig(%q) error = nil", name)
			}
		})
	}
}

func TestIgnoreStackRejectsOutsidePath(t *testing.T) {
	root := t.TempDir()
	stack := NewIgnoreStack(false, true, 0)
	if err := stack.Push(root); err != nil {
		t.Fatalf("Push() error = %v", err)
	}
	outside := filepath.Join(filepath.Dir(root), "outside.txt")
	if _, _, err := stack.MatchesPathHow(outside, false); err == nil || !strings.Contains(err.Error(), "outside ignore root") {
		t.Fatalf("MatchesPathHow() error = %v", err)
	}
}

func newTestRepositoryMatcher(t *testing.T, root string, config RepositoryConfig) *RepositoryMatcher {
	t.Helper()
	matcher, err := NewRepositoryMatcherWithConfig(root, config)
	if err != nil {
		t.Fatalf("NewRepositoryMatcherWithConfig() error = %v", err)
	}
	return matcher
}

func writeRepositoryFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
	}
}
