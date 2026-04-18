package viewer_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ohjann/ralphplusplus/internal/history"
	"github.com/ohjann/ralphplusplus/internal/userdata"
	"github.com/ohjann/ralphplusplus/internal/viewer"
)

// seedIteration writes a manifest with a single story/iteration plus the
// prompt and jsonl files the iteration points at. Returns the file paths so
// tests can inspect them.
func seedIteration(t *testing.T, fp, runID, storyID string, iter int, role string) (promptPath, jsonlPath string) {
	t.Helper()

	reposDir, err := userdata.ReposDir()
	if err != nil {
		t.Fatalf("ReposDir: %v", err)
	}
	runDir := filepath.Join(reposDir, fp, "runs", runID)
	turnDir := filepath.Join(runDir, "turns", storyID)
	if err := os.MkdirAll(turnDir, 0o755); err != nil {
		t.Fatalf("mkdir turn dir: %v", err)
	}
	stem := role + "-iter-" + strconv.Itoa(iter)
	promptPath = filepath.Join(turnDir, stem+".prompt")
	jsonlPath = filepath.Join(turnDir, stem+".jsonl")

	// A minimal prompt + two-line stream: message_start → content_block_start
	// (text) → content_block_delta → content_block_stop → message_stop gives
	// us exactly one assistant turn (turn 1) plus the synthesised user turn
	// (turn 0).
	if err := os.WriteFile(promptPath, []byte("hello prompt"), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}
	lines := []string{
		`{"type":"stream_event","event":{"type":"message_start","message":{"role":"assistant"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi back"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_stop","index":0}}`,
		`{"type":"stream_event","event":{"type":"message_stop"}}`,
	}
	if err := os.WriteFile(jsonlPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write jsonl: %v", err)
	}

	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	m := history.Manifest{
		SchemaVersion: history.ManifestSchemaVersion,
		RunID:         runID,
		Kind:          history.KindDaemon,
		RepoFP:        fp,
		Status:        history.StatusComplete,
		StartTime:     now,
		Stories: []history.StoryRecord{{
			StoryID: storyID,
			Iterations: []history.IterationRecord{{
				Index:          iter,
				Role:           role,
				StartTime:      now,
				PromptFile:     promptPath,
				TranscriptFile: jsonlPath,
			}},
		}},
	}
	d, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(runDir, "manifest.json"), d, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return promptPath, jsonlPath
}


func TestHandleTranscript_StreamsNDJSONWithImmutableCache(t *testing.T) {
	t.Setenv("RALPH_DATA_DIR", t.TempDir())
	const fp = "feedfacecafe"
	const runID = "run-1-aaaaaa"
	seedIteration(t, fp, runID, "S1", 0, "implementer")

	_, h := newTestServer(t)
	rr := doGet(t, h, "/api/repos/"+fp+"/runs/"+runID+"/transcript/S1/0")
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/x-ndjson" {
		t.Errorf("content-type=%q want application/x-ndjson", ct)
	}
	if cc := rr.Header().Get("Cache-Control"); cc != "public, max-age=3600, immutable" {
		t.Errorf("cache-control=%q want public, max-age=3600, immutable", cc)
	}

	// One JSON object per line; each line parses; at least turn 0 (user) +
	// turn 1 (assistant). We additionally require a trailing newline so curl
	// terminates cleanly on EOF.
	body := rr.Body.Bytes()
	if len(body) == 0 || body[len(body)-1] != '\n' {
		t.Errorf("body does not end with newline: %q", body)
	}
	sc := bufio.NewScanner(bytes.NewReader(body))
	count := 0
	for sc.Scan() {
		line := sc.Bytes()
		var m map[string]any
		if err := json.Unmarshal(line, &m); err != nil {
			t.Fatalf("line %d not JSON: %v — %q", count, err, line)
		}
		count++
	}
	if sc.Err() != nil {
		t.Fatalf("scan: %v", sc.Err())
	}
	if count < 2 {
		t.Errorf("emitted %d turns, want >=2", count)
	}
}

func TestHandleTranscript_FollowSetsNoStore(t *testing.T) {
	// follow=true against a terminal manifest drains the existing jsonl and
	// then closes via the followShouldClose path (Status=complete). We
	// shorten the idle deadline so the drain+terminate loop finishes inside
	// the test harness rather than blocking for the production 30 s.
	t.Setenv("RALPH_DATA_DIR", t.TempDir())
	const fp = "feedfacecafe"
	const runID = "run-1-aaaaaa"
	seedIteration(t, fp, runID, "S1", 0, "implementer")

	restore := viewer.SetFollowIdleDeadline(50 * time.Millisecond)
	defer restore()

	srv, h := newTestServer(t)
	_ = srv
	ts := httptest.NewServer(h)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/repos/"+fp+"/runs/"+runID+"/transcript/S1/0?follow=true", nil)
	req.Header.Set("X-Ralph-Token", "tok-abc")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("cache-control=%q want no-store", cc)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/x-ndjson" {
		t.Errorf("content-type=%q want application/x-ndjson", ct)
	}
	if _, err := io.ReadAll(resp.Body); err != nil {
		t.Fatalf("read: %v", err)
	}
}

func TestHandlePrompt_ReturnsTextPlain(t *testing.T) {
	t.Setenv("RALPH_DATA_DIR", t.TempDir())
	const fp = "feedfacecafe"
	const runID = "run-1-aaaaaa"
	seedIteration(t, fp, runID, "S1", 0, "implementer")

	_, h := newTestServer(t)
	rr := doGet(t, h, "/api/repos/"+fp+"/runs/"+runID+"/prompt/S1/0")
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("content-type=%q want text/plain; charset=utf-8", ct)
	}
	if got := rr.Body.String(); got != "hello prompt" {
		t.Errorf("body=%q want %q", got, "hello prompt")
	}
}

func TestHandleTranscript_RejectsTraversal(t *testing.T) {
	t.Setenv("RALPH_DATA_DIR", t.TempDir())
	_, h := newTestServer(t)

	for _, path := range []string{
		"/api/repos/fp/runs/rid/transcript/..%2Fescape/0",
		"/api/repos/fp/runs/rid/transcript/..%2F..%2Fetc/0",
	} {
		rr := doGet(t, h, path)
		if rr.Code == http.StatusOK {
			t.Errorf("path=%q returned 200; traversal not blocked", path)
		}
	}
}

func TestHandleTranscript_RejectsManifestWithEscapingPath(t *testing.T) {
	t.Setenv("RALPH_DATA_DIR", t.TempDir())
	const fp = "feedfacecafe"
	const runID = "run-1-aaaaaa"

	reposDir, _ := userdata.ReposDir()
	runDir := filepath.Join(reposDir, fp, "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	escape := filepath.Join(reposDir, fp, "runs", runID, "..", "secret.prompt")
	if err := os.WriteFile(filepath.Join(reposDir, fp, "runs", "secret.prompt"), []byte("SECRET"), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	m := history.Manifest{
		RunID: runID,
		Stories: []history.StoryRecord{{
			StoryID: "S1",
			Iterations: []history.IterationRecord{{
				Index:          0,
				Role:           "implementer",
				PromptFile:     escape,
				TranscriptFile: escape,
			}},
		}},
	}
	d, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(runDir, "manifest.json"), d, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	_, h := newTestServer(t)
	rr := doGet(t, h, "/api/repos/"+fp+"/runs/"+runID+"/prompt/S1/0")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status=%d want 400 for escaping path; body=%q", rr.Code, rr.Body.String())
	}
}

func TestHandleTranscript_404WhenIterMissing(t *testing.T) {
	t.Setenv("RALPH_DATA_DIR", t.TempDir())
	const fp = "feedfacecafe"
	const runID = "run-1-aaaaaa"
	seedIteration(t, fp, runID, "S1", 0, "implementer")

	_, h := newTestServer(t)
	rr := doGet(t, h, "/api/repos/"+fp+"/runs/"+runID+"/transcript/S1/99")
	if rr.Code != http.StatusNotFound {
		t.Errorf("status=%d want 404", rr.Code)
	}
}

func TestHandleTranscript_StreamsIncrementally(t *testing.T) {
	// Use the handler against a real listener so we can observe the flusher
	// emitting each Turn as it is produced. The bufio-backed parser yields
	// synchronously so all turns land before the body is closed; the test
	// verifies the complete response is well-formed NDJSON.
	t.Setenv("RALPH_DATA_DIR", t.TempDir())
	const fp = "feedfacecafe"
	const runID = "run-1-aaaaaa"
	seedIteration(t, fp, runID, "S1", 0, "implementer")

	srv, h := newTestServer(t)
	_ = srv
	ts := httptest.NewServer(h)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/repos/"+fp+"/runs/"+runID+"/transcript/S1/0", nil)
	req.Header.Set("X-Ralph-Token", "tok-abc")
	req.Host = "127.0.0.1"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.HasSuffix(data, []byte("\n")) {
		t.Errorf("body does not terminate with newline: %q", data)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) < 2 {
		t.Errorf("got %d lines, want >=2: %q", len(lines), data)
	}
	for i, line := range lines {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("line %d not JSON: %v — %q", i, err, line)
		}
	}
}

// turnLines returns a stream-json block that, when written to the jsonl,
// produces one assistant Turn with the given text. Used by the tail tests to
// build incremental fixtures.
func turnLines(text string) string {
	return strings.Join([]string{
		`{"type":"stream_event","event":{"type":"message_start","message":{"role":"assistant"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":` + strconv.Quote(text) + `}}}`,
		`{"type":"stream_event","event":{"type":"content_block_stop","index":0}}`,
		`{"type":"stream_event","event":{"type":"message_stop"}}`,
	}, "\n") + "\n"
}

// seedLiveIteration is seedIteration with Status=running and an empty jsonl
// so tests can append stream-json lines while the follow handler is active.
func seedLiveIteration(t *testing.T, fp, runID, storyID string) (promptPath, jsonlPath string) {
	t.Helper()
	reposDir, err := userdata.ReposDir()
	if err != nil {
		t.Fatalf("ReposDir: %v", err)
	}
	runDir := filepath.Join(reposDir, fp, "runs", runID)
	turnDir := filepath.Join(runDir, "turns", storyID)
	if err := os.MkdirAll(turnDir, 0o755); err != nil {
		t.Fatalf("mkdir turn dir: %v", err)
	}
	promptPath = filepath.Join(turnDir, "implementer-iter-0.prompt")
	jsonlPath = filepath.Join(turnDir, "implementer-iter-0.jsonl")
	if err := os.WriteFile(promptPath, []byte("hello prompt"), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}
	if err := os.WriteFile(jsonlPath, nil, 0o644); err != nil {
		t.Fatalf("write jsonl: %v", err)
	}
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	m := history.Manifest{
		SchemaVersion: history.ManifestSchemaVersion,
		RunID:         runID,
		Kind:          history.KindDaemon,
		RepoFP:        fp,
		Status:        history.StatusRunning,
		StartTime:     now,
		Stories: []history.StoryRecord{{
			StoryID: storyID,
			Iterations: []history.IterationRecord{{
				Index:          0,
				Role:           "implementer",
				StartTime:      now,
				PromptFile:     promptPath,
				TranscriptFile: jsonlPath,
			}},
		}},
	}
	d, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(runDir, "manifest.json"), d, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return promptPath, jsonlPath
}

// setManifestStatus rewrites just the Status field without touching the
// stories layout so the handler's followShouldClose check can see the
// terminal transition.
func setManifestStatus(t *testing.T, fp, runID, status string) {
	t.Helper()
	reposDir, err := userdata.ReposDir()
	if err != nil {
		t.Fatalf("ReposDir: %v", err)
	}
	path := filepath.Join(reposDir, fp, "runs", runID, "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m history.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	m.Status = status
	out, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

// TestHandleTranscript_FollowTailsIncrementalWrites covers the core AC: new
// Turns must surface within 200 ms of the jsonl flush that produced them.
// The fsnotify watcher is what makes this a push rather than a poll — if
// the handler ever degrades to polling, this test will still pass but
// latency assertions keep regressions honest.
func TestHandleTranscript_FollowTailsIncrementalWrites(t *testing.T) {
	t.Setenv("RALPH_DATA_DIR", t.TempDir())
	const fp = "feedfacecafe"
	const runID = "run-live-111"
	_, jsonlPath := seedLiveIteration(t, fp, runID, "S1")

	// Short idle so the final close happens promptly after we flip the
	// manifest to complete.
	restore := viewer.SetFollowIdleDeadline(100 * time.Millisecond)
	defer restore()

	_, h := newTestServer(t)
	ts := httptest.NewServer(h)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/repos/"+fp+"/runs/"+runID+"/transcript/S1/0?follow=true", nil)
	req.Header.Set("X-Ralph-Token", "tok-abc")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}

	type turnMsg struct {
		Index int    `json:"index"`
		Role  string `json:"role"`
	}
	type ev struct {
		turn turnMsg
		at   time.Time
	}
	events := make(chan ev, 16)
	readErr := make(chan error, 1)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for sc.Scan() {
			var t turnMsg
			if err := json.Unmarshal(sc.Bytes(), &t); err != nil {
				readErr <- err
				return
			}
			events <- ev{turn: t, at: time.Now()}
		}
		readErr <- sc.Err()
	}()

	// Turn 0 (synthesised from the prompt) should arrive immediately.
	select {
	case e := <-events:
		if e.turn.Index != 0 || e.turn.Role != "user" {
			t.Fatalf("first turn = %+v, want index=0 role=user", e.turn)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for turn 0")
	}

	f, err := os.OpenFile(jsonlPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open append: %v", err)
	}
	defer f.Close()

	for i, text := range []string{"alpha", "bravo", "charlie"} {
		flushAt := time.Now()
		if _, err := f.WriteString(turnLines(text)); err != nil {
			t.Fatalf("write turn %d: %v", i+1, err)
		}
		if err := f.Sync(); err != nil {
			t.Fatalf("sync: %v", err)
		}
		select {
		case e := <-events:
			if got, want := e.turn.Index, i+1; got != want {
				t.Errorf("turn %d: got index=%d want %d", i+1, got, want)
			}
			if got := e.at.Sub(flushAt); got > 200*time.Millisecond {
				t.Errorf("turn %d: latency=%s, want <=200ms", i+1, got)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for turn %d", i+1)
		}
	}

	// Flip the manifest to complete; after the next idle tick the handler
	// should close cleanly, which unblocks the reader goroutine.
	setManifestStatus(t, fp, runID, history.StatusComplete)
	select {
	case err := <-readErr:
		if err != nil {
			t.Fatalf("reader err: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not close after manifest flipped to complete")
	}
}

// TestHandleTranscript_FollowReplaysFromTurnZeroStatelessly covers the
// reconnect half of the story. Two follow requests back-to-back must both
// include Turn 0 (the synthesised prompt) and the full index sequence — the
// server is stateless, so clients de-dupe by Turn.Index.
func TestHandleTranscript_FollowReplaysFromTurnZeroStatelessly(t *testing.T) {
	t.Setenv("RALPH_DATA_DIR", t.TempDir())
	const fp = "feedfacecafe"
	const runID = "run-replay-1"
	_, jsonlPath := seedLiveIteration(t, fp, runID, "S1")

	// Pre-seed two complete turns and mark the run terminal; each request
	// drains + closes via followShouldClose.
	if err := os.WriteFile(jsonlPath, []byte(turnLines("first")+turnLines("second")), 0o644); err != nil {
		t.Fatalf("write jsonl: %v", err)
	}
	setManifestStatus(t, fp, runID, history.StatusComplete)

	restore := viewer.SetFollowIdleDeadline(50 * time.Millisecond)
	defer restore()

	_, h := newTestServer(t)
	ts := httptest.NewServer(h)
	defer ts.Close()

	fetch := func() []int {
		req, _ := http.NewRequest("GET", ts.URL+"/api/repos/"+fp+"/runs/"+runID+"/transcript/S1/0?follow=true", nil)
		req.Header.Set("X-Ralph-Token", "tok-abc")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("do: %v", err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var indices []int
		for _, line := range bytes.Split(bytes.TrimRight(body, "\n"), []byte("\n")) {
			if len(line) == 0 {
				continue
			}
			var t struct {
				Index int `json:"index"`
			}
			if err := json.Unmarshal(line, &t); err != nil {
				// Skip error envelopes (should not occur in this fixture).
				continue
			}
			indices = append(indices, t.Index)
		}
		return indices
	}

	first := fetch()
	second := fetch()

	want := []int{0, 1, 2}
	if len(first) != len(want) {
		t.Fatalf("first fetch indices=%v want %v", first, want)
	}
	for i, v := range want {
		if first[i] != v {
			t.Fatalf("first fetch indices=%v want %v", first, want)
		}
	}
	if len(second) != len(first) {
		t.Fatalf("second fetch indices=%v want %v", second, first)
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("replay diverged: first=%v second=%v", first, second)
		}
	}

	// The contract test: client de-dup by Turn.Index must yield zero
	// duplicate renders across a reconnect. Simulate the SPA's merge.
	seen := make(map[int]bool)
	for _, idx := range append(append([]int{}, first...), second...) {
		seen[idx] = true
	}
	if len(seen) != len(want) {
		t.Errorf("de-duped set=%v want len=%d", seen, len(want))
	}
}
