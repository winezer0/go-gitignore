package goignore

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// IgnoreLevel 保存单个目录编译后的忽略规则。
type IgnoreLevel struct {
	DirPath  string
	RuleSets []*RuleSet
}

// IgnoreStack 管理随目录层级变化的忽略规则栈。
type IgnoreStack struct {
	Levels   []IgnoreLevel
	NoIgnore bool // If true, ignore files (.gitignore, etc.) are bypassed
	Hidden   bool // If true, search hidden files/directories
	MaxDepth int  // Maximum depth to search (0 means infinite)
	// IgnoreFileNames 按从低到高优先级列出每层目录加载的规则文件名。
	IgnoreFileNames []string
	rootPath        string
}

// NewIgnoreStack 使用忽略、隐藏和深度选项创建规则栈。
func NewIgnoreStack(noIgnore, hidden bool, maxDepth int) *IgnoreStack {
	return &IgnoreStack{
		Levels:          nil,
		NoIgnore:        noIgnore,
		Hidden:          hidden,
		MaxDepth:        maxDepth,
		IgnoreFileNames: defaultIgnoreFileNames(),
	}
}

// LoadBaseRules 从起始路径向上加载基础规则，最多检查 64 层目录。
func (s *IgnoreStack) LoadBaseRules(startPath string) error {
	abs, err := filepath.Abs(startPath)
	if err != nil {
		return err
	}
	s.rootPath = filepath.Clean(abs)
	if s.NoIgnore {
		return nil
	}
	dirs, gitRoot, err := findRuleParents(abs)
	if err != nil {
		return err
	}
	baseDir := abs
	if gitRoot != "" {
		baseDir = gitRoot
	}

	globalPath, err := getGlobalGitIgnorePath()
	if err != nil {
		return err
	}
	if globalPath != "" {
		global, err := loadOptionalRules(globalPath)
		if err != nil {
			return err
		}
		if global != nil {
			s.Levels = append(s.Levels, IgnoreLevel{DirPath: baseDir, RuleSets: []*RuleSet{global}})
		}
	}
	if gitRoot != "" {
		gitPath := filepath.Join(gitRoot, ".git")
		gitInfo, err := os.Stat(gitPath)
		if err != nil {
			return err
		}
		if gitInfo.IsDir() {
			exclude, err := loadOptionalRules(filepath.Join(gitPath, "info", "exclude"))
			if err != nil {
				return err
			}
			if exclude != nil {
				s.Levels = append(s.Levels, IgnoreLevel{DirPath: gitRoot, RuleSets: []*RuleSet{exclude}})
			}
		}
	}

	for i := len(dirs) - 1; i >= 0; i-- {
		level, err := loadIgnoreLevel(dirs[i], s.IgnoreFileNames)
		if err != nil {
			return err
		}
		if hasRules(level) {
			s.Levels = append(s.Levels, level)
		}
	}
	return nil
}

func findRuleParents(start string) ([]string, string, error) {
	var dirs []string
	current := start
	for {
		if current != start {
			dirs = append(dirs, current)
		}
		_, statErr := os.Stat(filepath.Join(current, ".git"))
		if statErr == nil {
			return dirs, current, nil
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return nil, "", statErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			return dirs, "", nil
		}
		current = parent
	}
}

func getGlobalGitIgnorePath() (string, error) {
	// Check if git config --global core.excludesfile is set
	cmd := exec.Command("git", "config", "--global", "core.excludesfile")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err == nil {
		path := strings.TrimSpace(out.String())
		if path != "" {
			if strings.HasPrefix(path, "~") {
				home, err := os.UserHomeDir()
				if err != nil {
					return "", err
				}
				path = filepath.Join(home, strings.TrimLeft(path[1:], `/\`))
			}
			return path, nil
		}
	}

	// Default fallbacks
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	p := filepath.Join(home, ".config", "git", "ignore")
	if _, err := os.Stat(p); err == nil {
		return p, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	p = filepath.Join(home, ".gitignore")
	if _, err := os.Stat(p); err == nil {
		return p, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	return "", nil
}

func loadOptionalRules(path string) (*RuleSet, error) {
	rules, err := CompileFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return rules, nil
}

// Clone 创建规则栈副本，用于隔离子目录遍历状态。
func (s *IgnoreStack) Clone() *IgnoreStack {
	levels := make([]IgnoreLevel, len(s.Levels))
	copy(levels, s.Levels)
	fileNames := append([]string(nil), s.IgnoreFileNames...)
	return &IgnoreStack{
		Levels:          levels,
		NoIgnore:        s.NoIgnore,
		Hidden:          s.Hidden,
		MaxDepth:        s.MaxDepth,
		IgnoreFileNames: fileNames,
		rootPath:        s.rootPath,
	}
}

// Push 加载目录内的忽略文件并压入规则栈。
func (s *IgnoreStack) Push(dirPath string) error {
	abs, err := filepath.Abs(dirPath)
	if err != nil {
		return err
	}
	abs = filepath.Clean(abs)
	if s.rootPath == "" {
		s.rootPath = abs
	}
	if s.NoIgnore {
		return nil
	}

	level, err := loadIgnoreLevel(abs, s.IgnoreFileNames)
	if err != nil {
		return err
	}
	s.Levels = append(s.Levels, level)
	return nil
}

// Pop 移除最深一层忽略规则。
func (s *IgnoreStack) Pop() {
	if len(s.Levels) > 0 {
		s.Levels = s.Levels[:len(s.Levels)-1]
	}
}

// IsIgnored 从最深层到根层判断路径是否应被忽略，并返回路径处理错误。
func (s *IgnoreStack) IsIgnored(path string, isDir bool) (bool, error) {
	ignored, _, err := s.MatchesPathHow(path, isDir)
	return ignored, err
}

// MatchesPathHow 判断路径是否忽略，并返回最终生效规则及路径处理错误。
func (s *IgnoreStack) MatchesPathHow(path string, isDir bool) (bool, *MatchDetail, error) {
	return s.matchesPathHow(path, isDir)
}
