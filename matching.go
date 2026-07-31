package gitignore

import (
	"fmt"
	"path/filepath"
	"strings"
)

func (s *IgnoreStack) matchesPathHow(path string, isDir bool) (bool, *MatchDetail, error) {
	_, relPath, err := resolveWithinRoot(s.rootPath, path)
	if err != nil {
		return false, nil, err
	}
	parts := pathParts(relPath)
	if len(parts) == 0 {
		return false, nil, nil
	}
	current := s.rootPath
	for index, part := range parts {
		current = filepath.Join(current, filepath.FromSlash(part))
		candidateIsDir := index < len(parts)-1 || isDir
		ignored, detail, matchErr := s.matchCandidate(current, part, candidateIsDir)
		if matchErr != nil {
			return false, nil, matchErr
		}
		if ignored || index == len(parts)-1 {
			return ignored, detail, nil
		}
	}
	return false, nil, nil
}

func (s *IgnoreStack) matchCandidate(path, name string, isDir bool) (bool, *MatchDetail, error) {
	if !s.Hidden && strings.HasPrefix(name, ".") && name != "." && name != ".." {
		return true, &MatchDetail{Ignored: true, SourcePath: "<hidden>", Pattern: name}, nil
	}
	if s.NoIgnore {
		return false, nil, nil
	}
	for index := len(s.Levels) - 1; index >= 0; index-- {
		ignored, detail, err := matchLevelCandidate(s.Levels[index], path, isDir)
		if err != nil {
			return false, nil, err
		}
		if detail != nil {
			return ignored, detail, nil
		}
	}
	return false, nil, nil
}

func matchLevelCandidate(level IgnoreLevel, path string, isDir bool) (bool, *MatchDetail, error) {
	rel, inside, err := relativeWithin(level.DirPath, path)
	if err != nil || !inside {
		return false, nil, err
	}
	for index := len(level.RuleSets) - 1; index >= 0; index-- {
		rules := level.RuleSets[index]
		ignored, detail := rules.matchCandidate(rel, isDir)
		if detail != nil {
			return ignored, detail, nil
		}
	}
	return false, nil, nil
}

func resolveWithinRoot(root, path string) (string, string, error) {
	if root == "" {
		return "", "", fmt.Errorf("ignore root is not initialized")
	}
	absPath := path
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(root, absPath)
	}
	absPath, err := filepath.Abs(absPath)
	if err != nil {
		return "", "", err
	}
	rel, inside, err := relativeWithin(root, absPath)
	if err != nil {
		return "", "", err
	}
	if !inside {
		return "", "", fmt.Errorf("path %q is outside ignore root %q", path, root)
	}
	return filepath.Clean(absPath), rel, nil
}

func relativeWithin(root, path string) (string, bool, error) {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return "", false, err
	}
	rel = filepath.ToSlash(rel)
	inside := rel != ".." && !strings.HasPrefix(rel, "../")
	return rel, inside, nil
}
