package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short string", "hello", 10, "hello"},
		{"exactly at limit", "hello", 5, "hello"},
		{"over limit", "hello world", 8, "hello..."},
		{"maxLen 0", "hello", 0, ""},
		{"maxLen 1", "hello", 1, "h"},
		{"maxLen 2", "hello", 2, "he"},
		{"maxLen 3", "hello", 3, "hel"},
		{"maxLen 4", "hello", 4, "h..."},
		{"empty string", "", 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateStr(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateStr(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestEncodeProjectPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/Users/test/project", "-Users-test-project"},
		{"/home/user/my project", "-home-user-my-project"},
		{"simple", "simple"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := encodeProjectPath(tt.input)
			if got != tt.want {
				t.Errorf("encodeProjectPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatToolInput(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  string
	}{
		{"nil", nil, ""},
		{"command key", map[string]interface{}{"command": "ls -la"}, "ls -la"},
		{"file_path key", map[string]interface{}{"file_path": "/tmp/test.go"}, "/tmp/test.go"},
		{"pattern key", map[string]interface{}{"pattern": "*.go"}, "*.go"},
		{"unknown keys", map[string]interface{}{"foo": "bar"}, "foo"},
		{"non-map", "just a string", "just a string"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatToolInput(tt.input)
			if got != tt.want {
				t.Errorf("formatToolInput(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFindToolNameForResult(t *testing.T) {
	calls := []toolCallInfo{
		{Name: "Read", ToolUseID: "id-1"},
		{Name: "Write", ToolUseID: "id-2"},
		{Name: "Bash", ToolUseID: "id-3"},
	}
	tests := []struct {
		name      string
		toolUseID string
		want      string
	}{
		{"matching ID", "id-2", "Write"},
		{"empty ID", "", "unknown"},
		{"no match", "id-999", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findToolNameForResult(tt.toolUseID, calls)
			if got != tt.want {
				t.Errorf("findToolNameForResult(%q) = %q, want %q", tt.toolUseID, got, tt.want)
			}
		})
	}
}

func TestExtractTextContent(t *testing.T) {
	tests := []struct {
		name    string
		content interface{}
		want    string
	}{
		{"nil", nil, ""},
		{"string", "hello", "hello"},
		{"non-string", 42, "42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTextContent(tt.content)
			if got != tt.want {
				t.Errorf("extractTextContent(%v) = %q, want %q", tt.content, got, tt.want)
			}
		})
	}
}

func TestLocateSessionFile(t *testing.T) {
	// Not found case
	got := locateSessionFile("nonexistent-session-id", "/tmp/nonexistent")
	if got != "" {
		t.Errorf("locateSessionFile with nonexistent session should return empty, got %q", got)
	}

	// Empty session ID
	got = locateSessionFile("", "/tmp")
	if got != "" {
		t.Errorf("locateSessionFile with empty ID should return empty, got %q", got)
	}
}

func TestParseSessionJSONL(t *testing.T) {
	// Empty session ID returns ""
	got := parseSessionJSONL("", "/tmp")
	if got != "" {
		t.Errorf("parseSessionJSONL with empty session ID should return empty, got %q", got)
	}

	// Nonexistent session returns ""
	got = parseSessionJSONL("nonexistent-id", "/tmp/nonexistent")
	if got != "" {
		t.Errorf("parseSessionJSONL with nonexistent session should return empty, got %q", got)
	}
}

func TestFormatSessionContext(t *testing.T) {
	t.Run("empty inputs", func(t *testing.T) {
		got := formatSessionContext(nil, nil, nil, 8000)
		if got == "" {
			t.Error("formatSessionContext should return header even with empty inputs")
		}
	})

	t.Run("with tool calls", func(t *testing.T) {
		calls := []toolCallInfo{
			{Name: "Read", Input: "/tmp/test.go"},
			{Name: "Bash", Input: "go test ./..."},
		}
		got := formatSessionContext(calls, nil, nil, 8000)
		if got == "" {
			t.Error("formatSessionContext should return content with tool calls")
		}
	})

	t.Run("truncation at maxChars", func(t *testing.T) {
		calls := make([]toolCallInfo, 100)
		for i := range calls {
			calls[i] = toolCallInfo{Name: "Read", Input: "/very/long/path/that/takes/up/space/file.go"}
		}
		got := formatSessionContext(calls, nil, nil, 200)
		if len(got) > 200 {
			t.Errorf("formatSessionContext should truncate to maxChars, got len=%d", len(got))
		}
	})
}

func TestContentBlockUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{"text block", `{"type":"text","text":"hello"}`},
		{"tool_use block", `{"type":"tool_use","id":"123","name":"Read","input":{"file_path":"/tmp"}}`},
		{"tool_result with string content", `{"type":"tool_result","tool_use_id":"123","content":"result text"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cb contentBlock
			if err := cb.UnmarshalJSON([]byte(tt.json)); err != nil {
				t.Errorf("UnmarshalJSON(%s) error: %v", tt.json, err)
			}
		})
	}
}

func TestCheckPartialToolCall(t *testing.T) {
	t.Run("no file returns false", func(t *testing.T) {
		if CheckPartialToolCall("/nonexistent/path") {
			t.Error("expected false for nonexistent file")
		}
	})

	t.Run("complete tool call returns false", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.log")
		content := `{"type":"content_block_start","content_block":{"type":"tool_use","id":"1"}}
{"type":"content_block_stop"}
`
		os.WriteFile(path, []byte(content), 0o644)
		if CheckPartialToolCall(path) {
			t.Error("expected false for complete tool call")
		}
	})

	t.Run("partial tool call returns true", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.log")
		content := `{"type":"content_block_start","content_block":{"type":"tool_use","id":"1"}}
`
		os.WriteFile(path, []byte(content), 0o644)
		if !CheckPartialToolCall(path) {
			t.Error("expected true for partial tool call")
		}
	})

	t.Run("malformed JSON lines are skipped", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.log")
		content := `not json
{"type":"content_block_start","content_block":{"type":"tool_use","id":"1"}}
also not json
{"type":"content_block_stop"}
`
		os.WriteFile(path, []byte(content), 0o644)
		if CheckPartialToolCall(path) {
			t.Error("expected false for complete tool call with malformed lines")
		}
	})
}

func TestAppendActivityMarker(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "activity.log")

	// Creates file
	if err := AppendActivityMarker(path, "marker1"); err != nil {
		t.Fatalf("AppendActivityMarker create: %v", err)
	}

	// Appends to existing
	if err := AppendActivityMarker(path, "marker2"); err != nil {
		t.Fatalf("AppendActivityMarker append: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	if content != "marker1\nmarker2\n" {
		t.Errorf("unexpected content: %q", content)
	}
}
