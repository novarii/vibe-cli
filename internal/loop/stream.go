package loop

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// StreamEvent represents a Claude stream-json event
type StreamEvent struct {
	Type      string   `json:"type"`
	Subtype   string   `json:"subtype,omitempty"`
	Message   *Message `json:"message,omitempty"`
	SessionID string   `json:"session_id,omitempty"`
}

// Message represents a Claude message
type Message struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

// ContentBlock represents a content block in a message
type ContentBlock struct {
	Type string `json:"type"`
	// For text
	Text string `json:"text,omitempty"`
	// For tool_use
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
	// For tool_result
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
}

// StreamFormatter formats Claude stream-json output
type StreamFormatter struct {
	output          io.Writer
	capturedOutput  strings.Builder
	lastToolUse     string
	seenMessages    map[string]bool // Track message IDs to avoid duplicates
}

// NewStreamFormatter creates a new stream formatter
func NewStreamFormatter(output io.Writer) *StreamFormatter {
	return &StreamFormatter{
		output:       output,
		seenMessages: make(map[string]bool),
	}
}

// ProcessLine processes a single JSON line from the stream
func (f *StreamFormatter) ProcessLine(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	// Capture raw output for detection
	f.capturedOutput.WriteString(line)
	f.capturedOutput.WriteString("\n")

	var event StreamEvent
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		// Not JSON, print as-is
		fmt.Fprintln(f.output, line)
		return
	}

	f.formatEvent(&event)
}

// formatEvent formats a single stream event
func (f *StreamFormatter) formatEvent(event *StreamEvent) {
	switch event.Type {
	case "system":
		if event.Subtype == "init" {
			fmt.Fprintf(f.output, "\033[2m⚡ Session %s\033[0m\n", truncate(event.SessionID, 8))
		}

	case "assistant":
		if event.Message != nil {
			f.formatAssistantMessage(event.Message)
		}

	case "user":
		// User messages are tool results - show abbreviated
		if event.Message != nil {
			for _, block := range event.Message.Content {
				if block.Type == "tool_result" {
					f.formatToolResult(&block)
				}
			}
		}

	case "result":
		fmt.Fprintf(f.output, "\n\033[32m━━━ Done ━━━\033[0m\n")
	}
}

// formatAssistantMessage formats an assistant message
func (f *StreamFormatter) formatAssistantMessage(msg *Message) {
	for _, block := range msg.Content {
		switch block.Type {
		case "text":
			// Show text output
			text := strings.TrimSpace(block.Text)
			if text != "" {
				fmt.Fprintf(f.output, "\033[37m%s\033[0m\n", text)
			}

		case "tool_use":
			f.lastToolUse = block.Name
			fmt.Fprintf(f.output, "\033[33m🔧 %s\033[0m", block.Name)
			f.formatToolInput(block.Name, block.Input)
			fmt.Fprintln(f.output)

		case "thinking":
			// Show thinking (dimmed, abbreviated)
			if block.Text != "" {
				preview := truncate(block.Text, 100)
				fmt.Fprintf(f.output, "\033[2m💭 %s\033[0m\n", preview)
			}
		}
	}
}

// formatToolInput shows relevant tool input
func (f *StreamFormatter) formatToolInput(toolName string, input map[string]any) {
	if input == nil {
		return
	}

	switch toolName {
	case "Read":
		if path, ok := input["file_path"].(string); ok {
			fmt.Fprintf(f.output, " \033[2m%s\033[0m", path)
		}
	case "Write":
		if path, ok := input["file_path"].(string); ok {
			fmt.Fprintf(f.output, " \033[2m%s\033[0m", path)
		}
	case "Edit":
		if path, ok := input["file_path"].(string); ok {
			fmt.Fprintf(f.output, " \033[2m%s\033[0m", path)
		}
	case "Bash":
		if cmd, ok := input["command"].(string); ok {
			fmt.Fprintf(f.output, " \033[2m%s\033[0m", truncate(cmd, 50))
		}
	case "Glob":
		if pattern, ok := input["pattern"].(string); ok {
			fmt.Fprintf(f.output, " \033[2m%s\033[0m", pattern)
		}
	case "Grep":
		if pattern, ok := input["pattern"].(string); ok {
			fmt.Fprintf(f.output, " \033[2m%s\033[0m", pattern)
		}
	case "TodoWrite":
		fmt.Fprint(f.output, " \033[2mupdating todos\033[0m")
	case "Task":
		if desc, ok := input["description"].(string); ok {
			fmt.Fprintf(f.output, " \033[2m%s\033[0m", desc)
		}
	}
}

// formatToolResult shows abbreviated tool result
func (f *StreamFormatter) formatToolResult(block *ContentBlock) {
	// Just show a checkmark - results are usually verbose
	fmt.Fprintf(f.output, "\033[32m  ✓\033[0m\n")
}

// GetCapturedOutput returns all captured output for detection purposes
func (f *StreamFormatter) GetCapturedOutput() string {
	return f.capturedOutput.String()
}

// truncate shortens a string to max length
func truncate(s string, max int) string {
	// Remove newlines
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
