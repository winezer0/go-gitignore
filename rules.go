package gitignore

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// MatchDetail 描述最终命中的 ignore 规则及其来源。
type MatchDetail struct {
	Ignored    bool
	SourcePath string
	Line       int
	Pattern    string
	Negated    bool
}

// Rule 表示一条已编译的 Git ignore 规则。
type Rule struct {
	SourcePath string
	Line       int
	Pattern    string
	Negated    bool
	Directory  bool
	regexp     *regexp.Regexp
}

// RuleSet 保存单个 ignore 来源中按顺序编译的规则。
type RuleSet struct {
	rules []Rule
}

// CompileLines 编译内存中的 Git ignore 规则；sourcePath 用于诊断来源。
func CompileLines(sourcePath string, lines []string) (*RuleSet, error) {
	rules := make([]Rule, 0, len(lines))
	for index, line := range lines {
		if index == 0 {
			line = strings.TrimPrefix(line, "\ufeff")
		}
		rule, ok, err := compileRule(sourcePath, index+1, line)
		if err != nil {
			return nil, err
		}
		if ok {
			rules = append(rules, rule)
		}
	}
	return &RuleSet{rules: rules}, nil
}

// CompileFile 读取并编译 Git ignore 文件。
func CompileFile(path string) (*RuleSet, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	return CompileLines(path, lines)
}

// MatchesPathHow 判断相对路径是否忽略，并返回最后生效的规则。
func (s *RuleSet) MatchesPathHow(path string, isDir bool) (bool, *MatchDetail) {
	parts := pathParts(path)
	if len(parts) == 0 {
		return false, nil
	}
	for index := range parts {
		candidate := strings.Join(parts[:index+1], "/")
		candidateIsDir := index < len(parts)-1 || isDir
		ignored, detail := s.matchCandidate(candidate, candidateIsDir)
		if ignored && index < len(parts)-1 {
			return true, detail
		}
		if index == len(parts)-1 {
			return ignored, detail
		}
	}
	return false, nil
}

func (s *RuleSet) matchCandidate(path string, isDir bool) (bool, *MatchDetail) {
	ignored := false
	var detail *MatchDetail
	for index := range s.rules {
		rule := &s.rules[index]
		if rule.Directory && !isDir || !rule.regexp.MatchString(path) {
			continue
		}
		ignored = !rule.Negated
		detail = &MatchDetail{
			Ignored: ignored, SourcePath: rule.SourcePath, Line: rule.Line,
			Pattern: rule.Pattern, Negated: rule.Negated,
		}
	}
	return ignored, detail
}

func compileRule(source string, lineNumber int, original string) (Rule, bool, error) {
	line := strings.TrimSuffix(original, "\r")
	line = trimUnescapedTrailingSpaces(line)
	if line == "" || line[0] == '#' {
		return Rule{}, false, nil
	}
	negated := line[0] == '!'
	if negated {
		line = line[1:]
	}
	if line == "" {
		return Rule{}, false, nil
	}
	directory := strings.HasSuffix(line, "/")
	if directory {
		line = strings.TrimSuffix(line, "/")
	}
	anchored := strings.HasPrefix(line, "/") || strings.Contains(line, "/")
	line = strings.TrimPrefix(line, "/")
	expression, err := globExpression(line, anchored)
	if err != nil {
		return Rule{}, false, fmt.Errorf("%s:%d: %w", source, lineNumber, err)
	}
	compiled, err := regexp.Compile(expression)
	if err != nil {
		return Rule{}, false, fmt.Errorf("%s:%d: %w", source, lineNumber, err)
	}
	return Rule{
		SourcePath: source, Line: lineNumber, Pattern: original,
		Negated: negated, Directory: directory, regexp: compiled,
	}, true, nil
}

func pathParts(path string) []string {
	normalized := filepath.ToSlash(path)
	normalized = strings.Trim(normalized, "/")
	if normalized == "" || normalized == "." {
		return nil
	}
	parts := strings.Split(normalized, "/")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" && part != "." {
			result = append(result, part)
		}
	}
	return result
}

func trimUnescapedTrailingSpaces(value string) string {
	end := len(value)
	for end > 0 && value[end-1] == ' ' {
		backslashes := 0
		for index := end - 2; index >= 0 && value[index] == '\\'; index-- {
			backslashes++
		}
		if backslashes%2 == 1 {
			break
		}
		end--
	}
	return value[:end]
}
