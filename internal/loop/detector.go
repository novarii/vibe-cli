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
