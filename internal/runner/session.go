package runner

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ohjann/ralphplusplus/internal/debuglog"
)

// parseSessionJSONL reads a Claude Code JSONL session file and extracts
// structured context: tool calls attempted, tool errors, and assistant reasoning.
// Returns a formatted string truncated to maxChars, suitable for prompt injection.
// If the session file doesn't exist or can't be parsed, returns "".
func parseSessionJSONL(sessionID string, projectDir string) string {
	if sessionID == "" {
		return ""
	}

	sessionPath := locateSessionFile(sessionID, projectDir)
	if sessionPath == "" {
		return ""
	}

	f, err := os.Open(sessionPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	var toolCalls []toolCallInfo
	var toolErrors []toolErrorInfo
	var reasoningBlocks []string

	scanner := bufio.NewScanner(f)
	// Increase buffer for potentially large lines
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var msg sessionMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}

		for _, block := range msg.Content {
			switch block.Type {
			case "tool_use":
				tc := toolCallInfo{
					Name:      block.Name,
					Input:     truncateStr(formatToolInput(block.Input), 200),
					ToolUseID: block.ID,
				}
				toolCalls = append(toolCalls, tc)

			case "tool_result":
				if block.IsError {
					te := toolErrorInfo{
						ToolName: findToolNameForResult(block.ToolUseID, toolCalls),
						Error:    truncateStr(extractTextContent(block.Content), 300),
					}
					toolErrors = append(toolErrors, te)
				}

			case "text":
				if msg.Role == "assistant" && block.Text != "" {
					reasoningBlocks = append(reasoningBlocks, block.Text)
				}
			}
		}
	}
	// Log scanner errors (I/O errors, buffer overflow) but don't fail —
	// partial results are still useful for session context.
	if err := scanner.Err(); err != nil {
		debuglog.Log("parseSessionJSONL: scanner error: %v", err)
	}

	if len(toolCalls) == 0 && len(toolErrors) == 0 && len(reasoningBlocks) == 0 {
		return ""
	}

	return formatSessionContext(toolCalls, toolErrors, reasoningBlocks, 8000)
}

// locateSessionFile finds the JSONL session file for the given session ID.
// Session files live at ~/.claude/projects/<encoded-cwd>/<session-id>.jsonl
// where encoded-cwd replaces non-alphanumeric chars with dashes.
func locateSessionFile(sessionID, projectDir string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return ""
	}

	encodedCwd := encodeProjectPath(absDir)
	sessionPath := filepath.Join(homeDir, ".claude", "projects", encodedCwd, sessionID+".jsonl")

	if _, err := os.Stat(sessionPath); err != nil {
		return ""
	}

	return sessionPath
}

// nonAlphanumeric matches any character that is not a letter or digit.
var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9]`)

// encodeProjectPath converts an absolute path to the Claude Code encoded format.
// Non-alphanumeric characters are replaced with dashes.
func encodeProjectPath(absPath string) string {
	return nonAlphanumeric.ReplaceAllString(absPath, "-")
}

// Session message types for parsing JSONL

type sessionMessage struct {
	Role    string         `json:"role"`
	Content []contentBlock `json:"content"`
}

type contentBlock struct {
	Type      string      `json:"type"`
	ID        string      `json:"id,omitempty"`        // tool_use blocks
	Name      string      `json:"name,omitempty"`      // tool_use blocks
	Input     interface{} `json:"input,omitempty"`      // tool_use blocks
	Text      string      `json:"text,omitempty"`       // text blocks
	ToolUseID string      `json:"tool_use_id,omitempty"` // tool_result blocks
	IsError   bool        `json:"is_error,omitempty"`   // tool_result blocks
	Content   interface{} `json:"-"` // tool_result blocks, handled by custom unmarshaler
}

func (cb *contentBlock) UnmarshalJSON(data []byte) error {
	// Use a type alias to avoid infinite recursion
	type Alias contentBlock
	aux := &struct {
		*Alias
		// Content can be string or array, handle both
		RawContent json.RawMessage `json:"content,omitempty"`
	}{
		Alias: (*Alias)(cb),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	if len(aux.RawContent) > 0 {
		// Try string first
		var s string
		if err := json.Unmarshal(aux.RawContent, &s); err == nil {
			cb.Content = s
			return nil
		}
		// Otherwise keep as raw
		cb.Content = string(aux.RawContent)
	}

	return nil
}

type toolCallInfo struct {
	Name      string
	Input     string
	ToolUseID string
}

type toolErrorInfo struct {
	ToolName string
	Error    string
}

// formatToolInput extracts a short summary from tool input.
func formatToolInput(input interface{}) string {
	if input == nil {
		return ""
	}

	m, ok := input.(map[string]interface{})
	if !ok {
		return fmt.Sprintf("%v", input)
	}

	// Try common fields for a short summary
	if cmd, ok := m["command"].(string); ok {
		return cmd
	}
	if fp, ok := m["file_path"].(string); ok {
		return fp
	}
	if pattern, ok := m["pattern"].(string); ok {
		return pattern
	}
	if prompt, ok := m["prompt"].(string); ok {
		return truncateStr(prompt, 100)
	}

	// Fall back to listing keys
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return strings.Join(keys, ", ")
}

// findToolNameForResult looks up the tool name for a tool_use_id.
func findToolNameForResult(toolUseID string, calls []toolCallInfo) string {
	if toolUseID == "" {
		return "unknown"
	}
	// Search backwards since results follow their calls
	for i := len(calls) - 1; i >= 0; i-- {
		if calls[i].ToolUseID == toolUseID {
			return calls[i].Name
		}
	}
	return "unknown"
}

// extractTextContent pulls text from content that might be a string or JSON array.
func extractTextContent(content interface{}) string {
	if content == nil {
		return ""
	}
	if s, ok := content.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", content)
}

// truncateStr truncates a string to maxLen characters, appending "..." if truncated.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 4 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// formatSessionContext builds the final formatted output from parsed session data.
func formatSessionContext(toolCalls []toolCallInfo, toolErrors []toolErrorInfo, reasoning []string, maxChars int) string {
	var b strings.Builder

	b.WriteString("\n\n### Previous Session Transcript Analysis\n")

	// Tool calls - show last N
	if len(toolCalls) > 0 {
		b.WriteString("\n## Tool Calls Attempted\n")
		start := 0
		if len(toolCalls) > 30 {
			start = len(toolCalls) - 30
		}
		for _, tc := range toolCalls[start:] {
			if tc.Input != "" {
				b.WriteString(fmt.Sprintf("- %s: %s\n", tc.Name, tc.Input))
			} else {
				b.WriteString(fmt.Sprintf("- %s\n", tc.Name))
			}

			if b.Len() > maxChars-1000 {
				b.WriteString("- ... (truncated)\n")
				break
			}
		}
	}

	// Errors
	if len(toolErrors) > 0 {
		b.WriteString("\n## Errors Encountered\n")
		for _, te := range toolErrors {
			b.WriteString(fmt.Sprintf("- %s: %s\n", te.ToolName, te.Error))
			if b.Len() > maxChars-500 {
				b.WriteString("- ... (truncated)\n")
				break
			}
		}
	}

	// Reasoning - last few blocks
	if len(reasoning) > 0 {
		b.WriteString("\n## Agent Reasoning\n")
		start := 0
		if len(reasoning) > 5 {
			start = len(reasoning) - 5
		}
		for _, r := range reasoning[start:] {
			truncated := truncateStr(r, 500)
			b.WriteString(truncated + "\n\n")
			if b.Len() > maxChars {
				break
			}
		}
	}

	result := b.String()
	if len(result) > maxChars {
		result = result[:maxChars-3] + "..."
	}
	return result
}
