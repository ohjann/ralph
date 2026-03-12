package memory

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/eoghanhynes/ralph/internal/events"
	"github.com/eoghanhynes/ralph/internal/storystate"
)

// trackingChromaServer returns an httptest server that tracks which ChromaDB
// endpoints were called (add, update, query) and which collections were targeted.
// It simulates the dedup query path: if nearDuplicate is true, the query endpoint
// returns a close match (distance < 0.1) so DeduplicateInsert merges instead of adding.
type serverCalls struct {
	mu      sync.Mutex
	adds    []string // collection names that received /add calls
	updates []string // collection names that received /update calls
	queries []string // collection names that received /query calls
}

func (sc *serverCalls) record(kind, collection string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	switch kind {
	case "add":
		sc.adds = append(sc.adds, collection)
	case "update":
		sc.updates = append(sc.updates, collection)
	case "query":
		sc.queries = append(sc.queries, collection)
	}
}

func trackingChromaServer(t *testing.T, nearDuplicate bool) (*httptest.Server, *serverCalls) {
	t.Helper()
	calls := &serverCalls{}

	collectionIDs := map[string]string{
		CollectionPatterns.Name:    "uuid-patterns",
		CollectionCompletions.Name: "uuid-completions",
		CollectionErrors.Name:      "uuid-errors",
		CollectionDecisions.Name:   "uuid-decisions",
	}
	uuidToName := make(map[string]string)
	for name, id := range collectionIDs {
		uuidToName[id] = name
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GET endpoints: collection lookup by name or UUID-based count
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/collections/") {
			parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/collections/"), "/")
			key := parts[0]

			// Check if key is a UUID (for count endpoint)
			if collName, isUUID := uuidToName[key]; isUUID {
				if len(parts) > 1 && parts[1] == "count" {
					calls.record("count", collName)
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte("1"))
					return
				}
			}

			// Collection lookup by name
			if id, ok := collectionIDs[key]; ok {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"id": id})
				return
			}
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// POST endpoints: add, query, update, get
		if r.Method == http.MethodPost {
			// Parse collection UUID from path
			parts := strings.Split(r.URL.Path, "/")
			var collUUID, action string
			for i, p := range parts {
				if p == "collections" && i+1 < len(parts) {
					collUUID = parts[i+1]
					if i+2 < len(parts) {
						action = parts[i+2]
					}
					break
				}
			}
			collName := uuidToName[collUUID]

			switch action {
			case "query":
				calls.record("query", collName)
				w.Header().Set("Content-Type", "application/json")
				if nearDuplicate {
					// Return a close match (distance < 0.1) to trigger merge
					json.NewEncoder(w).Encode(map[string]interface{}{
						"ids":       [][]string{{"existing-doc"}},
						"documents": [][]string{{"existing content"}},
						"metadatas": [][]map[string]interface{}{{
							{"story_id": "test", "relevance_score": 0.8},
						}},
						"distances":  [][]float64{{0.05}},
						"embeddings": [][][]float64{{{0.1, 0.2, 0.3}}},
					})
				} else {
					// Return empty results (no duplicates found)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"ids":        [][]string{{}},
						"documents":  [][]string{{}},
						"metadatas":  [][]map[string]interface{}{{}},
						"distances":  [][]float64{{}},
						"embeddings": [][][]float64{{}},
					})
				}
				return
			case "add":
				calls.record("add", collName)
				io.ReadAll(r.Body)
				w.WriteHeader(http.StatusOK)
				return
			case "update":
				calls.record("update", collName)
				io.ReadAll(r.Body)
				w.WriteHeader(http.StatusOK)
				return
			case "get":
				// For EnforceCollectionCap — return empty
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"ids":       []string{},
					"documents": []string{},
					"metadatas": []map[string]interface{}{},
				})
				return
			case "count":
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte("1"))
				return
			}
		}

		// GET count endpoint: /api/v1/collections/{uuid}/count
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/count") {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("1"))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))

	return srv, calls
}

// TestPipelineEmbedPatterns_UsesDeduplicateInsertBatch verifies that embedPatterns
// calls DeduplicateInsertBatch (which queries for duplicates then adds/updates)
// rather than directly calling AddDocuments.
func TestPipelineEmbedPatterns_UsesDeduplicateInsertBatch(t *testing.T) {
	srv, calls := trackingChromaServer(t, false)
	defer srv.Close()

	client := NewClient(srv.URL)
	embedder := &fakeEmbedder{embedding: []float64{0.1, 0.2, 0.3}}
	pipeline := NewPipeline(client, embedder)

	evts := []events.Event{
		{
			Type:     events.EventPattern,
			Patterns: []string{"always use DeduplicateInsertBatch"},
		},
	}

	err := pipeline.embedPatterns(context.Background(), "TEST-001", evts, fixedTime())
	if err != nil {
		t.Fatalf("embedPatterns: %v", err)
	}

	calls.mu.Lock()
	defer calls.mu.Unlock()

	// DeduplicateInsertBatch calls QueryCollection first (to check for duplicates),
	// then AddDocuments if no duplicate found.
	if len(calls.queries) == 0 {
		t.Error("expected at least one query call (from DeduplicateInsert dedup check)")
	}
	if len(calls.adds) == 0 {
		t.Error("expected at least one add call (document inserted)")
	}
	// Verify the target collection is ralph_patterns
	for _, c := range calls.queries {
		if c != CollectionPatterns.Name {
			t.Errorf("query targeted %q, want %q", c, CollectionPatterns.Name)
		}
	}
}

// TestPipelineEmbedPatterns_NearDuplicateMerged verifies that when a near-duplicate
// document exists (cosine distance < 0.1), the pipeline merges via UpdateDocument
// instead of inserting a new document.
func TestPipelineEmbedPatterns_NearDuplicateMerged(t *testing.T) {
	srv, calls := trackingChromaServer(t, true) // nearDuplicate=true
	defer srv.Close()

	client := NewClient(srv.URL)
	embedder := &fakeEmbedder{embedding: []float64{0.1, 0.2, 0.3}}
	pipeline := NewPipeline(client, embedder)

	evts := []events.Event{
		{
			Type:     events.EventPattern,
			Patterns: []string{"use DeduplicateInsertBatch for all embeds"},
		},
	}

	err := pipeline.embedPatterns(context.Background(), "TEST-001", evts, fixedTime())
	if err != nil {
		t.Fatalf("embedPatterns: %v", err)
	}

	calls.mu.Lock()
	defer calls.mu.Unlock()

	// With a near-duplicate, DeduplicateInsert should query, find a match, and update
	if len(calls.queries) == 0 {
		t.Error("expected query call for dedup check")
	}
	if len(calls.updates) == 0 {
		t.Error("expected update call for merging near-duplicate document")
	}
	// Should NOT have added a new document since it was merged
	if len(calls.adds) > 0 {
		t.Error("expected no add calls when near-duplicate is merged, but got adds")
	}
}

// TestPipelineEmbedErrors_UsesDeduplicateInsertBatch verifies that embedErrors
// also goes through the dedup path.
func TestPipelineEmbedErrors_UsesDeduplicateInsertBatch(t *testing.T) {
	srv, calls := trackingChromaServer(t, false)
	defer srv.Close()

	client := NewClient(srv.URL)
	embedder := &fakeEmbedder{embedding: []float64{0.1, 0.2, 0.3}}
	pipeline := NewPipeline(client, embedder)

	state := storystate.StoryState{
		ErrorsEncountered: []storystate.ErrorEntry{
			{Error: "nil pointer", Resolution: "added nil check"},
		},
	}

	err := pipeline.embedErrors(context.Background(), "TEST-001", state, fixedTime())
	if err != nil {
		t.Fatalf("embedErrors: %v", err)
	}

	calls.mu.Lock()
	defer calls.mu.Unlock()

	if len(calls.queries) == 0 {
		t.Error("expected query call from DeduplicateInsert")
	}
	for _, c := range calls.queries {
		if c != CollectionErrors.Name {
			t.Errorf("query targeted %q, want %q", c, CollectionErrors.Name)
		}
	}
}

// TestPipelineEmbedDecisions_UsesDeduplicateInsertBatch verifies embedDecisions
// uses the dedup path.
func TestPipelineEmbedDecisions_UsesDeduplicateInsertBatch(t *testing.T) {
	srv, calls := trackingChromaServer(t, false)
	defer srv.Close()

	client := NewClient(srv.URL)
	embedder := &fakeEmbedder{embedding: []float64{0.1, 0.2, 0.3}}
	pipeline := NewPipeline(client, embedder)

	decisions := "## Decision 1\nUse Go interfaces for testability"

	err := pipeline.embedDecisions(context.Background(), "TEST-001", decisions, fixedTime())
	if err != nil {
		t.Fatalf("embedDecisions: %v", err)
	}

	calls.mu.Lock()
	defer calls.mu.Unlock()

	if len(calls.queries) == 0 {
		t.Error("expected query call from DeduplicateInsert")
	}
	for _, c := range calls.queries {
		if c != CollectionDecisions.Name {
			t.Errorf("query targeted %q, want %q", c, CollectionDecisions.Name)
		}
	}
}

// fixedTime returns a deterministic time for test reproducibility.
func fixedTime() time.Time {
	t, _ := time.Parse(time.RFC3339, "2026-01-15T10:00:00Z")
	return t
}
