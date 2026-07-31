package gitignore

import (
	"os"
	"path/filepath"
)

func loadRepositoryBase(root string, config RepositoryConfig) ([]IgnoreLevel, error) {
	dirs, gitRoot, err := findRuleParents(root)
	if err != nil {
		return nil, err
	}
	baseDir := root
	if gitRoot != "" {
		baseDir = gitRoot
	}
	levels, err := loadExternalBaseRules(baseDir, gitRoot, config)
	if err != nil {
		return nil, err
	}
	for index := len(dirs) - 1; index >= 0; index-- {
		level, loadErr := loadIgnoreLevel(dirs[index], config.IgnoreFileNames)
		if loadErr != nil {
			return nil, loadErr
		}
		if hasRules(level) {
			levels = append(levels, level)
		}
	}
	return levels, nil
}

func loadExternalBaseRules(baseDir, gitRoot string, config RepositoryConfig) ([]IgnoreLevel, error) {
	var levels []IgnoreLevel
	if config.UseGlobalGitIgnore {
		globalPath, err := getGlobalGitIgnorePath()
		if err != nil {
			return nil, err
		}
		levels, err = appendRuleFile(levels, baseDir, globalPath)
		if err != nil {
			return nil, err
		}
	}
	if config.UseGitExclude && gitRoot != "" {
		excludePath, err := gitExcludePath(gitRoot)
		if err != nil {
			return nil, err
		}
		levels, err = appendRuleFile(levels, gitRoot, excludePath)
		if err != nil {
			return nil, err
		}
	}
	return levels, nil
}

func gitExcludePath(gitRoot string) (string, error) {
	gitPath := filepath.Join(gitRoot, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", nil
	}
	return filepath.Join(gitPath, "info", "exclude"), nil
}

func appendRuleFile(levels []IgnoreLevel, dir, path string) ([]IgnoreLevel, error) {
	if path == "" {
		return levels, nil
	}
	rules, err := loadOptionalRules(path)
	if err != nil {
		return nil, err
	}
	if rules != nil {
		levels = append(levels, IgnoreLevel{DirPath: dir, RuleSets: []*RuleSet{rules}})
	}
	return levels, nil
}

func loadAdditionalFiles(root string, paths []string) ([]IgnoreLevel, error) {
	levels := make([]IgnoreLevel, 0, len(paths))
	for _, path := range paths {
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		rules, err := CompileFile(path)
		if err != nil {
			return nil, err
		}
		levels = append(levels, IgnoreLevel{DirPath: root, RuleSets: []*RuleSet{rules}})
	}
	return levels, nil
}

func ancestorDirectories(root, target string) []string {
	rel, inside, err := relativeWithin(root, target)
	if err != nil || !inside {
		return nil
	}
	dirs := []string{root}
	if rel == "." {
		return dirs
	}
	current := root
	for _, part := range pathParts(rel) {
		current = filepath.Join(current, filepath.FromSlash(part))
		dirs = append(dirs, current)
	}
	return dirs
}
