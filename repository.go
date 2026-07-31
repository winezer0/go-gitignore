package goignore

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// RepositoryConfig 配置仓库匹配器加载的规则来源；各字段为入参选项，零值表示不加载对应来源。
type RepositoryConfig struct {
	Hidden                bool
	IgnoreFileNames       []string
	UseGitExclude         bool
	UseGlobalGitIgnore    bool
	AdditionalIgnoreFiles []string
}

// RepositoryMatcher 保存仓库规则的不可变快照，可供多个 goroutine 并发读取。
type RepositoryMatcher struct {
	rootPath  string
	hidden    bool
	base      []IgnoreLevel
	levels    map[string]IgnoreLevel
	overrides []IgnoreLevel
}

// DefaultRepositoryConfig 返回启用 ripgrep 常用规则来源的默认配置。
// 返回值可由调用方复制后修改，不影响其他匹配器。
func DefaultRepositoryConfig() RepositoryConfig {
	return RepositoryConfig{
		IgnoreFileNames:    defaultIgnoreFileNames(),
		UseGitExclude:      true,
		UseGlobalGitIgnore: true,
	}
}

// NewRepositoryMatcher 使用默认配置创建仓库规则快照。
// root 为规则根目录，返回只读匹配器或初始化错误。
func NewRepositoryMatcher(root string) (*RepositoryMatcher, error) {
	return NewRepositoryMatcherWithConfig(root, DefaultRepositoryConfig())
}

// NewRepositoryMatcherWithConfig 使用指定来源配置创建仓库规则快照。
// root 为规则根目录，config 控制来源；返回只读匹配器或初始化错误。
func NewRepositoryMatcherWithConfig(root string, config RepositoryConfig) (*RepositoryMatcher, error) {
	absRoot, err := repositoryRoot(root)
	if err != nil {
		return nil, err
	}
	base, err := loadRepositoryBase(absRoot, config)
	if err != nil {
		return nil, err
	}
	levels, err := discoverRepositoryLevels(absRoot, config.IgnoreFileNames)
	if err != nil {
		return nil, err
	}
	overrides, err := loadAdditionalFiles(absRoot, config.AdditionalIgnoreFiles)
	if err != nil {
		return nil, err
	}
	return &RepositoryMatcher{
		rootPath: absRoot, hidden: config.Hidden, base: base,
		levels: levels, overrides: overrides,
	}, nil
}

// Matches 判断路径是否被仓库规则忽略。
// path 可为根目录内的绝对或相对路径，isDir 指示目录；返回结果和路径错误。
func (m *RepositoryMatcher) Matches(path string, isDir bool) (bool, error) {
	ignored, _, err := m.MatchesPathHow(path, isDir)
	return ignored, err
}

// MatchesPathHow 判断路径并返回最终生效规则。
// path 可为根目录内的绝对或相对路径，isDir 指示目录；返回结果、诊断和错误。
func (m *RepositoryMatcher) MatchesPathHow(path string, isDir bool) (bool, *MatchDetail, error) {
	if m == nil {
		return false, nil, fmt.Errorf("repository matcher is nil")
	}
	absPath, _, err := resolveWithinRoot(m.rootPath, path)
	if err != nil {
		return false, nil, err
	}
	stack := m.stackForPath(absPath)
	return stack.MatchesPathHow(absPath, isDir)
}

// Root 返回匹配器使用的绝对规则根目录。
func (m *RepositoryMatcher) Root() string {
	if m == nil {
		return ""
	}
	return m.rootPath
}

func (m *RepositoryMatcher) stackForPath(path string) *IgnoreStack {
	levels := make([]IgnoreLevel, 0, len(m.base)+len(m.overrides)+8)
	levels = append(levels, m.base...)
	for _, dir := range ancestorDirectories(m.rootPath, filepath.Dir(path)) {
		if level, ok := m.levels[dir]; ok {
			levels = append(levels, level)
		}
	}
	levels = append(levels, m.overrides...)
	return &IgnoreStack{Levels: levels, Hidden: m.hidden, rootPath: m.rootPath}
}

func repositoryRoot(root string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("repository root is empty")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("repository root %q is not a directory", root)
	}
	return filepath.Clean(absRoot), nil
}

func discoverRepositoryLevels(root string, fileNames []string) (map[string]IgnoreLevel, error) {
	levels := make(map[string]IgnoreLevel)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		if path != root && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		level, err := loadIgnoreLevel(path, fileNames)
		if err != nil {
			return err
		}
		if hasRules(level) {
			levels[filepath.Clean(path)] = level
		}
		return nil
	})
	return levels, err
}

func loadIgnoreLevel(dir string, fileNames []string) (IgnoreLevel, error) {
	level := IgnoreLevel{DirPath: filepath.Clean(dir)}
	for _, name := range fileNames {
		if err := validateIgnoreFileName(name); err != nil {
			return level, err
		}
		rules, err := loadOptionalRules(filepath.Join(dir, name))
		if err != nil {
			return level, err
		}
		if rules != nil {
			level.RuleSets = append(level.RuleSets, rules)
		}
	}
	return level, nil
}

func defaultIgnoreFileNames() []string {
	return []string{".gitignore", ".ignore", ".rgignore"}
}

func validateIgnoreFileName(name string) error {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
		return fmt.Errorf("invalid ignore file name %q", name)
	}
	return nil
}

func hasRules(level IgnoreLevel) bool {
	return len(level.RuleSets) > 0
}
