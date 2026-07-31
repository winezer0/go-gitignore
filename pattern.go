package gitignore

import (
	"fmt"
	"regexp"
	"strings"
)

func globExpression(pattern string, anchored bool) (string, error) {
	var expression strings.Builder
	if anchored {
		expression.WriteByte('^')
	} else {
		expression.WriteString(`(?:^|.*/)`)
	}
	for index := 0; index < len(pattern); index++ {
		char := pattern[index]
		switch char {
		case '*':
			index = writeStar(&expression, pattern, index)
		case '?':
			expression.WriteString(`[^/]`)
		case '[':
			end, class, err := characterClass(pattern, index)
			if err != nil {
				return "", err
			}
			expression.WriteString(class)
			index = end
		case '\\':
			if index+1 >= len(pattern) {
				expression.WriteString(`\\`)
				continue
			}
			index++
			expression.WriteString(regexp.QuoteMeta(string(pattern[index])))
		default:
			expression.WriteString(regexp.QuoteMeta(string(char)))
		}
	}
	expression.WriteByte('$')
	return expression.String(), nil
}

func writeStar(expression *strings.Builder, pattern string, index int) int {
	end := index + 1
	for end < len(pattern) && pattern[end] == '*' {
		end++
	}
	if end-index != 2 || !isGlobStar(pattern, index, end) {
		expression.WriteString(`[^/]*`)
		return end - 1
	}
	if end < len(pattern) && pattern[end] == '/' {
		expression.WriteString(`(?:[^/]+/)*`)
		return end
	}
	expression.WriteString(`.*`)
	return end - 1
}

func isGlobStar(pattern string, start, end int) bool {
	leftBoundary := start == 0 || pattern[start-1] == '/'
	rightBoundary := end == len(pattern) || pattern[end] == '/'
	return leftBoundary && rightBoundary
}

func characterClass(pattern string, start int) (int, string, error) {
	end := characterClassEnd(pattern, start)
	if end >= len(pattern) {
		return 0, "", fmt.Errorf("unterminated character class in %q", pattern)
	}
	content := pattern[start+1 : end]
	if strings.HasPrefix(content, "!") {
		content = "^" + content[1:]
	}
	content = strings.ReplaceAll(content, `\`, `\\`)
	return end, "[" + content + "]", nil
}

func characterClassEnd(pattern string, start int) int {
	index := start + 1
	if index < len(pattern) && (pattern[index] == '!' || pattern[index] == '^') {
		index++
	}
	if index < len(pattern) && pattern[index] == ']' {
		index++
	}
	for index < len(pattern) {
		if pattern[index] == '\\' && index+1 < len(pattern) {
			index += 2
			continue
		}
		if isPOSIXClassStart(pattern, index) {
			index = posixClassEnd(pattern, index)
			continue
		}
		if pattern[index] == ']' {
			return index
		}
		index++
	}
	return len(pattern)
}

func isPOSIXClassStart(pattern string, index int) bool {
	return index+1 < len(pattern) && pattern[index] == '[' && pattern[index+1] == ':'
}

func posixClassEnd(pattern string, start int) int {
	for index := start + 2; index+1 < len(pattern); index++ {
		if pattern[index] == ':' && pattern[index+1] == ']' {
			return index + 2
		}
	}
	return start + 1
}
