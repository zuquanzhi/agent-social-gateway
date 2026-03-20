package security

import (
	"regexp"
	"strings"
)

type ContentFilter struct {
	patterns []*regexp.Regexp
}

func NewContentFilter() *ContentFilter {
	return &ContentFilter{
		patterns: []*regexp.Regexp{
			// API keys and tokens
			regexp.MustCompile(`(?i)(api[_-]?key|token|secret|password)\s*[:=]\s*["']?[a-zA-Z0-9_\-]{16,}["']?`),
			// AWS keys
			regexp.MustCompile(`(?i)AKIA[0-9A-Z]{16}`),
			// Generic secrets
			regexp.MustCompile(`(?i)(sk-[a-zA-Z0-9]{32,})`),
			// Private keys
			regexp.MustCompile(`-----BEGIN (RSA |EC )?PRIVATE KEY-----`),
		},
	}
}

func (cf *ContentFilter) AddPattern(pattern string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	cf.patterns = append(cf.patterns, re)
	return nil
}

func (cf *ContentFilter) ContainsSensitiveData(text string) bool {
	for _, p := range cf.patterns {
		if p.MatchString(text) {
			return true
		}
	}
	return false
}

func (cf *ContentFilter) Redact(text string) string {
	result := text
	for _, p := range cf.patterns {
		result = p.ReplaceAllStringFunc(result, func(match string) string {
			if len(match) <= 8 {
				return "[REDACTED]"
			}
			return match[:4] + strings.Repeat("*", len(match)-8) + match[len(match)-4:]
		})
	}
	return result
}

func (cf *ContentFilter) IsSafe(text string) (bool, string) {
	if cf.ContainsSensitiveData(text) {
		return false, "content contains potentially sensitive data"
	}
	return true, ""
}
