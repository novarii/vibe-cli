package loop

import (
	"regexp"
	"strings"
)

// PromiseDetector detects completion promises in output
type PromiseDetector struct {
	pattern *regexp.Regexp
	promise string
}

// NewPromiseDetector creates a new promise detector
func NewPromiseDetector(promise string) *PromiseDetector {
	if promise == "" {
		return nil
	}

	// Pattern: <promise>VALUE</promise>
	pattern := regexp.MustCompile(`<promise>([^<]*)</promise>`)

	return &PromiseDetector{
		pattern: pattern,
		promise: promise,
	}
}

// Detect checks if the completion promise is found in the output
func (d *PromiseDetector) Detect(output string) bool {
	if d == nil {
		return false
	}

	matches := d.pattern.FindAllStringSubmatch(output, -1)
	for _, match := range matches {
		if len(match) >= 2 {
			value := strings.TrimSpace(match[1])
			if value == d.promise {
				return true
			}
		}
	}

	return false
}

// ExtractPromise extracts the promise value from output if present
func (d *PromiseDetector) ExtractPromise(output string) string {
	if d == nil {
		return ""
	}

	matches := d.pattern.FindAllStringSubmatch(output, -1)
	for _, match := range matches {
		if len(match) >= 2 {
			return strings.TrimSpace(match[1])
		}
	}

	return ""
}

// PRDetector detects GitHub PR URLs in output
type PRDetector struct {
	pattern *regexp.Regexp
}

// NewPRDetector creates a new PR detector
func NewPRDetector() *PRDetector {
	// Matches: https://github.com/owner/repo/pull/123
	pattern := regexp.MustCompile(`https://github\.com/[^/\s]+/[^/\s]+/pull/\d+`)
	return &PRDetector{pattern: pattern}
}

// Detect checks if a PR URL is found in the output
func (d *PRDetector) Detect(output string) bool {
	if d == nil {
		return false
	}
	return d.pattern.MatchString(output)
}

// ExtractURL extracts the PR URL from output if present
func (d *PRDetector) ExtractURL(output string) string {
	if d == nil {
		return ""
	}
	return d.pattern.FindString(output)
}
