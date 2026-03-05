package events

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type EventType string

const (
	EventStoryComplete    EventType = "story_complete"
	EventStoryFailed      EventType = "story_failed"
	EventPattern          EventType = "pattern"
	EventStuck            EventType = "stuck"
	EventJudgeResult      EventType = "judge_result"
	EventContextExhausted EventType = "context_exhausted"
)

type Event struct {
	Type      EventType         `json:"type"`
	Timestamp time.Time         `json:"timestamp"`
	StoryID   string            `json:"story_id,omitempty"`
	Summary   string            `json:"summary"`
	Files     []string          `json:"files,omitempty"`
	Patterns  []string          `json:"patterns,omitempty"`
	Errors    []string          `json:"errors,omitempty"`
	Meta      map[string]string `json:"meta,omitempty"`
}

const eventsFileName = "events.jsonl"

func eventsPath(projectDir string) string {
	return filepath.Join(projectDir, ".ralph", eventsFileName)
}

// Append writes an event to the events.jsonl file.
func Append(projectDir string, event Event) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}

	path := eventsPath(projectDir)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening events file: %w", err)
	}
	defer f.Close()

	_, err = fmt.Fprintln(f, string(data))
	return err
}

// Load reads all events from events.jsonl.
func Load(projectDir string) ([]Event, error) {
	path := eventsPath(projectDir)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue // skip malformed lines
		}
		events = append(events, ev)
	}
	return events, scanner.Err()
}

// Filter specifies criteria for querying events.
type Filter struct {
	Type    EventType
	StoryID string
	Limit   int
}

// Query returns events matching the filter.
func Query(projectDir string, filter Filter) ([]Event, error) {
	all, err := Load(projectDir)
	if err != nil {
		return nil, err
	}

	var result []Event
	for _, ev := range all {
		if filter.Type != "" && ev.Type != filter.Type {
			continue
		}
		if filter.StoryID != "" && ev.StoryID != filter.StoryID {
			continue
		}
		result = append(result, ev)
	}

	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[len(result)-filter.Limit:]
	}
	return result, nil
}

// FormatContextSection renders relevant events as a markdown section for prompt injection.
func FormatContextSection(evts []Event, currentStoryID string) string {
	if len(evts) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Context from Previous Iterations\n\n")

	// Always include pattern events
	var patterns []Event
	var storyEvents []Event
	var recentCompleted []Event

	for _, ev := range evts {
		switch ev.Type {
		case EventPattern:
			patterns = append(patterns, ev)
		case EventStuck, EventStoryFailed, EventContextExhausted:
			if ev.StoryID == currentStoryID {
				storyEvents = append(storyEvents, ev)
			}
		case EventStoryComplete:
			recentCompleted = append(recentCompleted, ev)
		}
	}

	if len(patterns) > 0 {
		sb.WriteString("### Discovered Patterns\n")
		for _, ev := range patterns {
			for _, p := range ev.Patterns {
				sb.WriteString("- " + p + "\n")
			}
		}
		sb.WriteString("\n")
	}

	if len(storyEvents) > 0 {
		sb.WriteString("### Previous Issues with This Story\n")
		for _, ev := range storyEvents {
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", ev.Type, ev.Summary))
			for _, e := range ev.Errors {
				sb.WriteString(fmt.Sprintf("  - Error: %s\n", e))
			}
		}
		sb.WriteString("\n")
	}

	// Show last 3 completed stories
	if len(recentCompleted) > 3 {
		recentCompleted = recentCompleted[len(recentCompleted)-3:]
	}
	if len(recentCompleted) > 0 {
		sb.WriteString("### Recently Completed Stories\n")
		for _, ev := range recentCompleted {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", ev.StoryID, ev.Summary))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
