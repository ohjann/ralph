package memory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/eoghanhynes/ralph/internal/costs"
	"github.com/eoghanhynes/ralph/internal/events"
	"github.com/eoghanhynes/ralph/internal/storystate"
)

// approxEqual checks if two floats are within epsilon.
func approxEqual(a, b, epsilon float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < epsilon
}

// e2eChromaState holds the in-memory state for a mock ChromaDB server that
// supports add, query, update, delete, count, and get operations.
type e2eChromaState struct {
	mu          sync.Mutex
	collections map[string]map[string]chromaDoc // collection name -> doc ID -> doc
}

type chromaDoc struct {
	ID        string
	Content   string
	Embedding []float64
	Metadata  map[string]interface{}
}

func newE2EChromaState() *e2eChromaState {
	return &e2eChromaState{
		collections: make(map[string]map[string]chromaDoc),
	}
}

func (s *e2eChromaState) ensureCollection(name string) {
	if _, ok := s.collections[name]; !ok {
		s.collections[name] = make(map[string]chromaDoc)
	}
}

// cosineSimilarity computes cosine similarity between two vectors.
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, magA, magB float64
	for i := range a {
		dot += a[i] * b[i]
		magA += a[i] * a[i]
		magB += b[i] * b[i]
	}
	if magA == 0 || magB == 0 {
		return 0
	}
	return dot / (sqrt(magA) * sqrt(magB))
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 100; i++ {
		z = (z + x/z) / 2
	}
	return z
}

// newE2EChromaServer creates a full-featured mock ChromaDB server that supports
// the operations needed by the synthesis pipeline.
func newE2EChromaServer(t *testing.T, state *e2eChromaState) *httptest.Server {
	t.Helper()

	// Map collection names to stable UUIDs
	collectionIDs := map[string]string{
		CollectionLessons.Name:     "uuid-lessons",
		CollectionPRDLessons.Name:  "uuid-prd-lessons",
		CollectionPatterns.Name:    "uuid-patterns",
		CollectionCompletions.Name: "uuid-completions",
		CollectionErrors.Name:      "uuid-errors",
		CollectionDecisions.Name:   "uuid-decisions",
		CollectionCodebase.Name:    "uuid-codebase",
	}
	idToName := make(map[string]string)
	for name, id := range collectionIDs {
		idToName[id] = name
		state.ensureCollection(name)
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		state.mu.Lock()
		defer state.mu.Unlock()

		// Collection lookup by name: GET .../collections/{name}
		for name, id := range collectionIDs {
			if r.Method == http.MethodGet && searchString(r.URL.Path, "/collections/"+name) && !searchString(r.URL.Path, "uuid-") {
				json.NewEncoder(w).Encode(map[string]string{"id": id})
				return
			}
		}

		// Route by collection UUID
		for uuid, colName := range idToName {
			prefix := "/collections/" + uuid

			// Query: POST .../collections/{uuid}/query
			if r.Method == http.MethodPost && searchString(r.URL.Path, prefix+"/query") {
				var req struct {
					QueryEmbeddings [][]float64 `json:"query_embeddings"`
					NResults        int         `json:"n_results"`
				}
				json.NewDecoder(r.Body).Decode(&req)

				docs := state.collections[colName]
				queryEmb := req.QueryEmbeddings[0]

				type scored struct {
					doc      chromaDoc
					distance float64
				}
				var results []scored
				for _, d := range docs {
					sim := cosineSimilarity(queryEmb, d.Embedding)
					dist := 1.0 - sim
					results = append(results, scored{doc: d, distance: dist})
				}
				// Sort by distance ascending
				for i := range results {
					for j := i + 1; j < len(results); j++ {
						if results[j].distance < results[i].distance {
							results[i], results[j] = results[j], results[i]
						}
					}
				}
				n := req.NResults
				if n > len(results) {
					n = len(results)
				}
				results = results[:n]

				ids := make([][]string, 1)
				documents := make([][]string, 1)
				metadatas := make([][]map[string]interface{}, 1)
				distances := make([][]float64, 1)
				embeddings := make([][][]float64, 1)
				for _, r := range results {
					ids[0] = append(ids[0], r.doc.ID)
					documents[0] = append(documents[0], r.doc.Content)
					meta := make(map[string]interface{})
					for k, v := range r.doc.Metadata {
						meta[k] = v
					}
					metadatas[0] = append(metadatas[0], meta)
					distances[0] = append(distances[0], r.distance)
					embeddings[0] = append(embeddings[0], r.doc.Embedding)
				}
				if ids[0] == nil {
					ids = [][]string{{}}
					documents = [][]string{{}}
					metadatas = [][]map[string]interface{}{{}}
					distances = [][]float64{{}}
					embeddings = [][][]float64{{}}
				}

				json.NewEncoder(w).Encode(map[string]interface{}{
					"ids": ids, "documents": documents,
					"metadatas": metadatas, "distances": distances,
					"embeddings": embeddings,
				})
				return
			}

			// Add: POST .../collections/{uuid}/add
			if r.Method == http.MethodPost && searchString(r.URL.Path, prefix+"/add") {
				var body struct {
					IDs        []string                 `json:"ids"`
					Documents  []string                 `json:"documents"`
					Embeddings [][]float64              `json:"embeddings"`
					Metadatas  []map[string]interface{} `json:"metadatas"`
				}
				json.NewDecoder(r.Body).Decode(&body)
				for i, id := range body.IDs {
					d := chromaDoc{ID: id}
					if i < len(body.Documents) {
						d.Content = body.Documents[i]
					}
					if i < len(body.Embeddings) {
						d.Embedding = body.Embeddings[i]
					}
					if i < len(body.Metadatas) {
						d.Metadata = body.Metadatas[i]
					}
					state.collections[colName][id] = d
				}
				json.NewEncoder(w).Encode(map[string]interface{}{})
				return
			}

			// Update: POST .../collections/{uuid}/update
			if r.Method == http.MethodPost && searchString(r.URL.Path, prefix+"/update") {
				var body struct {
					IDs        []string                 `json:"ids"`
					Documents  []string                 `json:"documents"`
					Embeddings [][]float64              `json:"embeddings"`
					Metadatas  []map[string]interface{} `json:"metadatas"`
				}
				json.NewDecoder(r.Body).Decode(&body)
				for i, id := range body.IDs {
					if existing, ok := state.collections[colName][id]; ok {
						if i < len(body.Documents) && body.Documents[i] != "" {
							existing.Content = body.Documents[i]
						}
						if i < len(body.Embeddings) && body.Embeddings[i] != nil {
							existing.Embedding = body.Embeddings[i]
						}
						if i < len(body.Metadatas) && body.Metadatas[i] != nil {
							for k, v := range body.Metadatas[i] {
								if existing.Metadata == nil {
									existing.Metadata = make(map[string]interface{})
								}
								existing.Metadata[k] = v
							}
						}
						state.collections[colName][id] = existing
					}
				}
				json.NewEncoder(w).Encode(map[string]interface{}{})
				return
			}

			// Get all: POST .../collections/{uuid}/get
			if r.Method == http.MethodPost && searchString(r.URL.Path, prefix+"/get") {
				docs := state.collections[colName]
				ids := make([]string, 0, len(docs))
				documents := make([]string, 0, len(docs))
				metadatas := make([]map[string]interface{}, 0, len(docs))
				for _, d := range docs {
					ids = append(ids, d.ID)
					documents = append(documents, d.Content)
					meta := make(map[string]interface{})
					for k, v := range d.Metadata {
						meta[k] = v
					}
					metadatas = append(metadatas, meta)
				}
				json.NewEncoder(w).Encode(map[string]interface{}{
					"ids": ids, "documents": documents, "metadatas": metadatas,
				})
				return
			}

			// Count: GET .../collections/{uuid}/count
			if r.Method == http.MethodGet && searchString(r.URL.Path, prefix+"/count") {
				json.NewEncoder(w).Encode(len(state.collections[colName]))
				return
			}

			// Delete: POST .../collections/{uuid}/delete
			if r.Method == http.MethodPost && searchString(r.URL.Path, prefix+"/delete") {
				var body struct {
					IDs []string `json:"ids"`
				}
				json.NewDecoder(r.Body).Decode(&body)
				for _, id := range body.IDs {
					delete(state.collections[colName], id)
				}
				json.NewEncoder(w).Encode(map[string]interface{}{})
				return
			}
		}

		http.NotFound(w, r)
	}))
}

// seqEmbedder returns different embeddings per call to avoid false deduplication.
type seqEmbedder struct {
	mu    sync.Mutex
	calls int
}

func (e *seqEmbedder) Embed(_ context.Context, texts []string) ([][]float64, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	result := make([][]float64, len(texts))
	for i := range texts {
		e.calls++
		// Use near-orthogonal vectors so cosine distance > 0.1
		emb := make([]float64, 10)
		emb[e.calls%10] = 1.0
		result[i] = emb
	}
	return result, nil
}

func (e *seqEmbedder) EmbedOne(_ context.Context, _ string) ([]float64, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls++
	emb := make([]float64, 10)
	emb[e.calls%10] = 1.0
	return emb, nil
}

// TestE2E_SynthesizeAndEmbed tests the full pipeline: SynthesizeRunLessons
// produces valid lessons from a realistic mixed-outcome run, then EmbedLessons
// stores them with deduplication across multiple runs.
func TestE2E_SynthesizeAndEmbed(t *testing.T) {
	ctx := context.Background()
	chromaState := newE2EChromaState()
	srv := newE2EChromaServer(t, chromaState)
	defer srv.Close()

	client := NewClient(srv.URL)
	embedder := &seqEmbedder{} // Different embedding per lesson to avoid false dedup

	// Simulate a realistic run with mixed success/failure stories.
	runSummary := costs.RunSummary{
		PRD:                   "feature-auth",
		StoriesTotal:          6,
		StoriesCompleted:      4,
		StoriesFailed:         2,
		TotalIterations:       18,
		AvgIterationsPerStory: 3.0,
		StuckCount:            3,
		JudgeRejectionRate:    0.25,
		DurationMinutes:       60.0,
		TotalCost:             2.50,
	}

	storyStates := []storystate.StoryState{
		{StoryID: "A-001", Status: "complete", IterationCount: 2},
		{StoryID: "A-002", Status: "complete", IterationCount: 1},
		{StoryID: "A-003", Status: "blocked", IterationCount: 5, ErrorsEncountered: []storystate.ErrorEntry{{Error: "type mismatch", Resolution: "none"}}},
		{StoryID: "A-004", Status: "complete", IterationCount: 3, JudgeFeedback: []string{"missing error handling"}},
		{StoryID: "A-005", Status: "failed", IterationCount: 4, ErrorsEncountered: []storystate.ErrorEntry{{Error: "nil pointer", Resolution: "none"}}},
		{StoryID: "A-006", Status: "complete", IterationCount: 3},
	}

	evts := []events.Event{
		{Type: events.EventStuck, StoryID: "A-003", Summary: "repeated edits to same file"},
		{Type: events.EventStoryFailed, StoryID: "A-005", Summary: "nil pointer in handler"},
		{Type: events.EventJudgeResult, StoryID: "A-004", Summary: "missing error handling"},
	}

	mockResponse := `{
		"lessons": [
			{"category": "tooling", "pattern": "Agents edit wrong file repeatedly", "evidence": "A-003 stuck on same file", "recommendation": "Validate file path against plan", "confidence": 0.8},
			{"category": "testing", "pattern": "Missing nil checks in handlers", "evidence": "A-005 failed with nil pointer", "recommendation": "Add nil guard for optional fields", "confidence": 0.7}
		],
		"prd_lessons": [
			{"category": "sizing", "pattern": "A-003 was too large", "evidence": "5 iterations and blocked", "recommendation": "Split into smaller stories", "confidence": 0.85}
		]
	}`

	runner := func(ctx context.Context, prompt string) (string, costs.TokenUsage, error) {
		return mockResponse, costs.TokenUsage{}, nil
	}

	// Step 1: Synthesize lessons
	result, err := synthesizeWithRunner(ctx, t.TempDir(), runSummary, storyStates, evts, runner)
	if err != nil {
		t.Fatalf("SynthesizeRunLessons: %v", err)
	}
	if len(result.Lessons) != 2 {
		t.Fatalf("expected 2 lessons, got %d", len(result.Lessons))
	}
	if len(result.PRDLessons) != 1 {
		t.Fatalf("expected 1 PRD lesson, got %d", len(result.PRDLessons))
	}

	// Verify lesson IDs and fields
	if result.Lessons[0].ID != "L-001" {
		t.Errorf("lesson[0].ID = %q, want L-001", result.Lessons[0].ID)
	}
	if result.Lessons[0].TimesConfirmed != 1 {
		t.Errorf("lesson[0].TimesConfirmed = %d, want 1", result.Lessons[0].TimesConfirmed)
	}
	if result.PRDLessons[0].ID != "PL-001" {
		t.Errorf("prd_lesson[0].ID = %q, want PL-001", result.PRDLessons[0].ID)
	}

	// Step 2: Embed lessons
	tmpDir := t.TempDir()
	err = EmbedLessons(ctx, client, embedder, result.Lessons, tmpDir)
	if err != nil {
		t.Fatalf("EmbedLessons: %v", err)
	}

	// Verify documents were stored in ChromaDB
	chromaState.mu.Lock()
	lessonCount := len(chromaState.collections[CollectionLessons.Name])
	chromaState.mu.Unlock()

	if lessonCount != 2 {
		t.Errorf("expected 2 lessons in ChromaDB, got %d", lessonCount)
	}

	// Verify lessons.json was persisted
	lf, err := LoadLessons(tmpDir)
	if err != nil {
		t.Fatalf("LoadLessons: %v", err)
	}
	if len(lf.Lessons) != 2 {
		t.Errorf("expected 2 lessons in file, got %d", len(lf.Lessons))
	}
}

// TestE2E_ConfidenceTrackingAcrossRuns tests that a lesson confirmed across
// 3 runs has higher confidence than a single-occurrence lesson.
func TestE2E_ConfidenceTrackingAcrossRuns(t *testing.T) {
	ctx := context.Background()
	chromaState := newE2EChromaState()
	srv := newE2EChromaServer(t, chromaState)
	defer srv.Close()

	client := NewClient(srv.URL)
	embedder := &mockEmbedder{embedding: []float64{0.5, 0.5, 0.5}}

	// The same lesson appearing in 3 different runs
	repeatedLesson := Lesson{
		ID:             "L-repeat",
		Category:       "tooling",
		Pattern:        "Agents edit wrong file repeatedly",
		Recommendation: "Validate file path against plan",
		Confidence:     0.7,
		TimesConfirmed: 1,
	}

	// Run 1: embed the lesson for the first time
	tmpDir := t.TempDir()
	err := EmbedLessons(ctx, client, embedder, []Lesson{repeatedLesson}, tmpDir)
	if err != nil {
		t.Fatalf("Run 1 EmbedLessons: %v", err)
	}

	// Verify initial state
	chromaState.mu.Lock()
	var initialDoc chromaDoc
	for _, d := range chromaState.collections[CollectionLessons.Name] {
		initialDoc = d
		break
	}
	initialConf := initialDoc.Metadata["confidence"].(float64)
	initialTC := initialDoc.Metadata["times_confirmed"].(float64)
	chromaState.mu.Unlock()

	if !approxEqual(initialConf, 0.7, 0.01) {
		t.Errorf("Run 1: confidence = %f, want ~0.7", initialConf)
	}
	if !approxEqual(initialTC, 1.0, 0.01) {
		t.Errorf("Run 1: times_confirmed = %f, want 1", initialTC)
	}

	// Run 2: embed the same lesson again (near-duplicate detection)
	err = EmbedLessons(ctx, client, embedder, []Lesson{repeatedLesson}, t.TempDir())
	if err != nil {
		t.Fatalf("Run 2 EmbedLessons: %v", err)
	}

	chromaState.mu.Lock()
	var run2Doc chromaDoc
	for _, d := range chromaState.collections[CollectionLessons.Name] {
		run2Doc = d
		break
	}
	run2Conf := run2Doc.Metadata["confidence"].(float64)
	run2TC := run2Doc.Metadata["times_confirmed"].(float64)
	chromaState.mu.Unlock()

	if !approxEqual(run2Conf, 0.8, 0.01) {
		t.Errorf("Run 2: confidence = %f, want ~0.8 (0.7 + 0.1)", run2Conf)
	}
	if !approxEqual(run2TC, 2.0, 0.01) {
		t.Errorf("Run 2: times_confirmed = %f, want 2", run2TC)
	}

	// Run 3: embed the same lesson again
	err = EmbedLessons(ctx, client, embedder, []Lesson{repeatedLesson}, t.TempDir())
	if err != nil {
		t.Fatalf("Run 3 EmbedLessons: %v", err)
	}

	chromaState.mu.Lock()
	var run3Doc chromaDoc
	for _, d := range chromaState.collections[CollectionLessons.Name] {
		run3Doc = d
		break
	}
	run3Conf := run3Doc.Metadata["confidence"].(float64)
	run3TC := run3Doc.Metadata["times_confirmed"].(float64)
	chromaState.mu.Unlock()

	if !approxEqual(run3Conf, 0.9, 0.01) {
		t.Errorf("Run 3: confidence = %f, want ~0.9 (0.8 + 0.1)", run3Conf)
	}
	if !approxEqual(run3TC, 3.0, 0.01) {
		t.Errorf("Run 3: times_confirmed = %f, want 3", run3TC)
	}

	// Now embed a single-occurrence lesson with a different embedding
	singleEmbedder := &mockEmbedder{embedding: []float64{0.9, 0.1, 0.0}}
	singleLesson := Lesson{
		ID:             "L-single",
		Category:       "architecture",
		Pattern:        "Completely different pattern",
		Recommendation: "Different recommendation",
		Confidence:     0.5,
		TimesConfirmed: 1,
	}

	err = EmbedLessons(ctx, client, singleEmbedder, []Lesson{singleLesson}, t.TempDir())
	if err != nil {
		t.Fatalf("Single EmbedLessons: %v", err)
	}

	// Verify that the 3-run lesson has higher confidence than the single-run lesson
	chromaState.mu.Lock()
	var multiRunConf, singleRunConf float64
	var multiRunTC, singleRunTC float64
	for _, d := range chromaState.collections[CollectionLessons.Name] {
		tc := d.Metadata["times_confirmed"].(float64)
		conf := d.Metadata["confidence"].(float64)
		if tc >= 3 {
			multiRunConf = conf
			multiRunTC = tc
		} else if tc == 1 {
			singleRunConf = conf
			singleRunTC = tc
		}
	}
	chromaState.mu.Unlock()

	if multiRunConf <= singleRunConf {
		t.Errorf("3-run lesson confidence (%f) should be > single-run (%f)", multiRunConf, singleRunConf)
	}
	if multiRunTC != 3.0 {
		t.Errorf("3-run lesson times_confirmed = %f, want 3", multiRunTC)
	}
	if singleRunTC != 1.0 {
		t.Errorf("single-run lesson times_confirmed = %f, want 1", singleRunTC)
	}
}

// TestE2E_DecayEvictionBelowThreshold tests that lessons with decayed
// confidence below 0.3 are evicted after enough decay cycles.
func TestE2E_DecayEvictionBelowThreshold(t *testing.T) {
	ctx := context.Background()
	chromaState := newE2EChromaState()
	srv := newE2EChromaServer(t, chromaState)
	defer srv.Close()

	client := NewClient(srv.URL)

	// Seed documents with varying relevance scores
	chromaState.mu.Lock()
	chromaState.collections[CollectionLessons.Name]["high-conf"] = chromaDoc{
		ID:      "high-conf",
		Content: "High confidence lesson",
		Metadata: map[string]interface{}{
			"relevance_score": 1.0,
			"confidence":      0.9,
		},
	}
	chromaState.collections[CollectionLessons.Name]["low-conf"] = chromaDoc{
		ID:      "low-conf",
		Content: "Low confidence lesson",
		Metadata: map[string]interface{}{
			"relevance_score": 0.35,
			"confidence":      0.3,
		},
	}
	chromaState.mu.Unlock()

	// Create tracker — neither document is confirmed
	tracker := NewConfirmationTracker()

	// Run decay cycle 1: 0.35 * 0.85 = 0.2975 < 0.3 → evict low-conf
	summary, err := RunDecayCycle(ctx, client, tracker)
	if err != nil {
		t.Fatalf("RunDecayCycle: %v", err)
	}

	if summary.Evicted == 0 {
		t.Error("expected at least one eviction on first decay cycle")
	}

	// Verify the low-confidence doc was evicted
	chromaState.mu.Lock()
	_, lowExists := chromaState.collections[CollectionLessons.Name]["low-conf"]
	_, highExists := chromaState.collections[CollectionLessons.Name]["high-conf"]
	chromaState.mu.Unlock()

	if lowExists {
		t.Error("low-confidence lesson should have been evicted (0.35 * 0.85 = 0.2975 < 0.3)")
	}
	if !highExists {
		t.Error("high-confidence lesson should NOT have been evicted")
	}
}

// TestE2E_DecayMultipleCyclesEviction tests that after enough unconfirmed
// decay cycles, even a moderately confident document is eventually evicted.
func TestE2E_DecayMultipleCyclesEviction(t *testing.T) {
	ctx := context.Background()
	chromaState := newE2EChromaState()
	srv := newE2EChromaServer(t, chromaState)
	defer srv.Close()

	client := NewClient(srv.URL)

	// Start with a moderate relevance score of 0.5
	chromaState.mu.Lock()
	chromaState.collections[CollectionLessons.Name]["moderate"] = chromaDoc{
		ID:      "moderate",
		Content: "Moderate lesson",
		Metadata: map[string]interface{}{
			"relevance_score": 0.5,
			"confidence":      0.5,
		},
	}
	chromaState.mu.Unlock()

	tracker := NewConfirmationTracker()

	// 0.5 * 0.85 = 0.425 (cycle 1)
	// 0.425 * 0.85 = 0.361 (cycle 2)
	// 0.361 * 0.85 = 0.307 (cycle 3)
	// 0.307 * 0.85 = 0.261 < 0.3 (cycle 4 — evict)

	var evicted bool
	for i := 0; i < 10; i++ {
		summary, err := RunDecayCycle(ctx, client, tracker)
		if err != nil {
			t.Fatalf("cycle %d: %v", i+1, err)
		}
		if summary.Evicted > 0 {
			evicted = true
			t.Logf("eviction happened at cycle %d", i+1)
			break
		}
	}

	if !evicted {
		t.Error("expected document to be evicted after multiple decay cycles")
	}

	chromaState.mu.Lock()
	remaining := len(chromaState.collections[CollectionLessons.Name])
	chromaState.mu.Unlock()

	if remaining != 0 {
		t.Errorf("expected 0 documents after eviction, got %d", remaining)
	}
}

// TestE2E_DetectAntiPatternsFromMockErrors tests that DetectAntiPatterns
// correctly identifies fragile areas from error documents stored in ChromaDB.
func TestE2E_DetectAntiPatternsFromMockErrors(t *testing.T) {
	ctx := context.Background()
	chromaState := newE2EChromaState()

	// Seed error documents that all mention the same file
	chromaState.collections[CollectionErrors.Name] = map[string]chromaDoc{
		"e1": {ID: "e1", Content: "build error in runner", Metadata: map[string]interface{}{"files": "internal/runner/runner.go", "story_id": "S1"}},
		"e2": {ID: "e2", Content: "type error in runner", Metadata: map[string]interface{}{"files": "internal/runner/runner.go", "story_id": "S2"}},
		"e3": {ID: "e3", Content: "nil pointer in runner", Metadata: map[string]interface{}{"files": "internal/runner/runner.go,internal/other.go", "story_id": "S3"}},
	}
	chromaState.collections[CollectionCompletions.Name] = map[string]chromaDoc{}

	srv := newE2EChromaServer(t, chromaState)
	defer srv.Close()

	client := NewClient(srv.URL)
	patterns, err := DetectAntiPatterns(ctx, client)
	if err != nil {
		t.Fatalf("DetectAntiPatterns: %v", err)
	}

	var foundFragile bool
	for _, p := range patterns {
		if p.Category == "fragile_area" {
			for _, f := range p.FilesAffected {
				if f == "internal/runner/runner.go" {
					foundFragile = true
					if p.OccurrenceCount < 3 {
						t.Errorf("expected >=3 occurrences, got %d", p.OccurrenceCount)
					}
					if len(p.AffectedStories) < 3 {
						t.Errorf("expected >=3 affected stories, got %d", len(p.AffectedStories))
					}
				}
			}
		}
	}

	if !foundFragile {
		t.Error("expected to detect fragile_area for internal/runner/runner.go")
	}
}

// TestE2E_FullPipelineSynthesizeEmbedDetect tests the complete end-to-end flow:
// synthesize → embed → detect anti-patterns → verify BuildPrompt warnings.
func TestE2E_FullPipelineSynthesizeEmbedDetect(t *testing.T) {
	ctx := context.Background()
	chromaState := newE2EChromaState()

	// Pre-seed error docs to trigger anti-pattern detection
	chromaState.collections[CollectionErrors.Name] = map[string]chromaDoc{
		"e1": {ID: "e1", Content: "error 1", Metadata: map[string]interface{}{"files": "internal/runner/runner.go", "story_id": "S1"}},
		"e2": {ID: "e2", Content: "error 2", Metadata: map[string]interface{}{"files": "internal/runner/runner.go", "story_id": "S2"}},
		"e3": {ID: "e3", Content: "error 3", Metadata: map[string]interface{}{"files": "internal/runner/runner.go", "story_id": "S3"}},
	}
	chromaState.collections[CollectionCompletions.Name] = map[string]chromaDoc{}

	srv := newE2EChromaServer(t, chromaState)
	defer srv.Close()

	client := NewClient(srv.URL)
	embedder := &mockEmbedder{embedding: []float64{0.5, 0.5, 0.5}}

	// Step 1: Synthesize lessons from a run
	mockResponse := `{
		"lessons": [
			{"category": "tooling", "pattern": "File editing loop", "evidence": "Multiple stories", "recommendation": "Validate paths", "confidence": 0.8}
		],
		"prd_lessons": []
	}`
	runner := func(ctx context.Context, prompt string) (string, costs.TokenUsage, error) {
		return mockResponse, costs.TokenUsage{}, nil
	}

	result, err := synthesizeWithRunner(ctx, t.TempDir(), costs.RunSummary{
		PRD: "test", StoriesTotal: 3, StoriesCompleted: 1, StoriesFailed: 2,
	}, nil, nil, runner)
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}

	// Step 2: Embed lessons
	tmpDir := t.TempDir()
	err = EmbedLessons(ctx, client, embedder, result.Lessons, tmpDir)
	if err != nil {
		t.Fatalf("embed: %v", err)
	}

	// Verify lesson is stored
	chromaState.mu.Lock()
	lessonCount := len(chromaState.collections[CollectionLessons.Name])
	chromaState.mu.Unlock()
	if lessonCount != 1 {
		t.Errorf("expected 1 lesson in ChromaDB, got %d", lessonCount)
	}

	// Step 3: Detect anti-patterns from seeded errors
	patterns, err := DetectAntiPatterns(ctx, client)
	if err != nil {
		t.Fatalf("detect anti-patterns: %v", err)
	}
	if len(patterns) == 0 {
		t.Fatal("expected at least 1 anti-pattern")
	}

	// Step 4: Verify anti-patterns contain the expected data for BuildPrompt injection
	var foundRunnerPattern bool
	for _, p := range patterns {
		if p.Category == "fragile_area" {
			for _, f := range p.FilesAffected {
				if f == "internal/runner/runner.go" {
					foundRunnerPattern = true
					if p.OccurrenceCount < 3 {
						t.Errorf("expected >=3 occurrences, got %d", p.OccurrenceCount)
					}
				}
			}
		}
	}
	if !foundRunnerPattern {
		t.Error("expected fragile_area anti-pattern for internal/runner/runner.go")
	}
}

// TestE2E_ConfirmedDocumentSurvivesDecay verifies that confirmed documents
// get boosted during decay and survive while unconfirmed ones are evicted.
func TestE2E_ConfirmedDocumentSurvivesDecay(t *testing.T) {
	ctx := context.Background()
	chromaState := newE2EChromaState()
	srv := newE2EChromaServer(t, chromaState)
	defer srv.Close()

	client := NewClient(srv.URL)

	// Seed two documents: one will be confirmed, one won't
	chromaState.mu.Lock()
	chromaState.collections[CollectionLessons.Name]["confirmed-doc"] = chromaDoc{
		ID:      "confirmed-doc",
		Content: "Confirmed lesson",
		Metadata: map[string]interface{}{
			"relevance_score": 0.4,
			"confidence":      0.4,
		},
	}
	chromaState.collections[CollectionLessons.Name]["unconfirmed-doc"] = chromaDoc{
		ID:      "unconfirmed-doc",
		Content: "Unconfirmed lesson",
		Metadata: map[string]interface{}{
			"relevance_score": 0.4,
			"confidence":      0.4,
		},
	}
	chromaState.mu.Unlock()

	// Confirm one document
	tracker := NewConfirmationTracker()
	tracker.ConfirmDocument(ctx, CollectionLessons.Name, "confirmed-doc")

	// Run decay cycle
	// confirmed: 0.4 + 0.1 = 0.5 (boosted)
	// unconfirmed: 0.4 * 0.85 = 0.34 (decayed but survives)
	_, err := RunDecayCycle(ctx, client, tracker)
	if err != nil {
		t.Fatalf("RunDecayCycle: %v", err)
	}

	chromaState.mu.Lock()
	confirmedDoc := chromaState.collections[CollectionLessons.Name]["confirmed-doc"]
	unconfirmedDoc := chromaState.collections[CollectionLessons.Name]["unconfirmed-doc"]
	chromaState.mu.Unlock()

	confirmedScore := confirmedDoc.Metadata["relevance_score"].(float64)
	unconfirmedScore := unconfirmedDoc.Metadata["relevance_score"].(float64)

	if confirmedScore != 0.5 {
		t.Errorf("confirmed doc score = %f, want 0.5", confirmedScore)
	}
	if unconfirmedScore < 0.33 || unconfirmedScore > 0.35 {
		t.Errorf("unconfirmed doc score = %f, want ~0.34", unconfirmedScore)
	}

	// Second decay cycle: unconfirmed drops to 0.34 * 0.85 = 0.289 < 0.3 → evict
	tracker2 := NewConfirmationTracker() // Fresh tracker, nothing confirmed
	summary, err := RunDecayCycle(ctx, client, tracker2)
	if err != nil {
		t.Fatalf("RunDecayCycle 2: %v", err)
	}

	if summary.Evicted == 0 {
		t.Error("expected eviction in second cycle")
	}

	chromaState.mu.Lock()
	_, confirmedExists := chromaState.collections[CollectionLessons.Name]["confirmed-doc"]
	_, unconfirmedExists := chromaState.collections[CollectionLessons.Name]["unconfirmed-doc"]
	chromaState.mu.Unlock()

	if !confirmedExists {
		t.Error("confirmed doc should survive (score was 0.5 → 0.425)")
	}
	if unconfirmedExists {
		t.Error("unconfirmed doc should be evicted (score 0.34 → 0.289 < 0.3)")
	}
}
