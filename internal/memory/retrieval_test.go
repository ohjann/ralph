package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestComputeRecencyWeight_ZeroTime(t *testing.T) {
	now := time.Now()
	w := computeRecencyWeight(time.Time{}, now)
	if w != 0.5 {
		t.Errorf("expected 0.5 for zero time, got %f", w)
	}
}

func TestComputeRecencyWeight_Recent(t *testing.T) {
	now := time.Now()
	w := computeRecencyWeight(now, now)
	if math.Abs(w-1.0) > 0.01 {
		t.Errorf("expected ~1.0 for now, got %f", w)
	}
}

func TestComputeRecencyWeight_FarPast(t *testing.T) {
	now := time.Now()
	past := now.Add(-365 * 24 * time.Hour)
	w := computeRecencyWeight(past, now)
	if w > 0.01 {
		t.Errorf("expected near-zero for 1 year ago, got %f", w)
	}
}

func TestComputeRecencyWeight_Future(t *testing.T) {
	now := time.Now()
	future := now.Add(7 * 24 * time.Hour)
	w := computeRecencyWeight(future, now)
	if math.Abs(w-1.0) > 0.01 {
		t.Errorf("expected ~1.0 for future timestamp (clamped to 0 days), got %f", w)
	}
	if w > 1.0 {
		t.Errorf("recency weight must not exceed 1.0, got %f", w)
	}
}

func TestEstimateTokens_Empty(t *testing.T) {
	if estimateTokens("") != 1 {
		t.Errorf("expected 1 for empty string")
	}
}

func TestEstimateTokens_Short(t *testing.T) {
	if estimateTokens("ab") != 1 {
		t.Errorf("expected 1 for short string")
	}
}

func TestEstimateTokens_Long(t *testing.T) {
	s := strings.Repeat("a", 100)
	if estimateTokens(s) != 25 {
		t.Errorf("expected 25 for 100-char string, got %d", estimateTokens(s))
	}
}

func TestFormatMarkdown_GroupsAndOrders(t *testing.T) {
	results := []rankedResult{
		{
			result:     QueryResult{Document: Document{Content: "pattern1"}},
			collection: CollectionPatterns.Name,
			combined:   0.9,
		},
		{
			result:     QueryResult{Document: Document{Content: "error1"}},
			collection: CollectionErrors.Name,
			combined:   0.8,
		},
		{
			result:     QueryResult{Document: Document{Content: "pattern2"}},
			collection: CollectionPatterns.Name,
			combined:   0.7,
		},
	}

	md := formatMarkdown(results)

	if !strings.Contains(md, "## Relevant Memory") {
		t.Error("missing header")
	}
	if !strings.Contains(md, "### Relevant Patterns") {
		t.Error("missing patterns section")
	}
	if !strings.Contains(md, "### Known Errors") {
		t.Error("missing errors section")
	}
	// Patterns should appear before errors (insertion order)
	patternsIdx := strings.Index(md, "### Relevant Patterns")
	errorsIdx := strings.Index(md, "### Known Errors")
	if patternsIdx > errorsIdx {
		t.Error("patterns section should appear before errors section")
	}
	// Verify relevance score formatting
	if !strings.Contains(md, "pattern1 (relevance: 0.90)") {
		t.Error("missing formatted relevance score for pattern1")
	}
	if !strings.Contains(md, "error1 (relevance: 0.80)") {
		t.Error("missing formatted relevance score for error1")
	}
}

func TestFormatMarkdown_UnknownCollection(t *testing.T) {
	results := []rankedResult{
		{
			result:     QueryResult{Document: Document{Content: "test"}},
			collection: "unknown-collection",
			combined:   0.9,
		},
	}

	md := formatMarkdown(results)
	if !strings.Contains(md, "### unknown-collection") {
		t.Error("expected fallback header for unknown collection")
	}
}

func TestFormatMarkdown_SingleResultExceedsTokenBudget(t *testing.T) {
	// A single large result should still be included even if it exceeds the token budget.
	bigContent := strings.Repeat("word ", 500) // ~2500 chars = ~625 tokens
	results := []rankedResult{
		{
			result:     QueryResult{Document: Document{Content: bigContent}},
			collection: CollectionPatterns.Name,
			combined:   0.9,
		},
		{
			result:     QueryResult{Document: Document{Content: "small"}},
			collection: CollectionPatterns.Name,
			combined:   0.8,
		},
	}

	// Simulate the token budget logic from RetrieveContext
	maxTokens := 100
	var selected []rankedResult
	tokenCount := 0
	for _, r := range results {
		contentTokens := estimateTokens(r.result.Document.Content)
		if tokenCount+contentTokens > maxTokens && len(selected) > 0 {
			break
		}
		selected = append(selected, r)
		tokenCount += contentTokens
	}

	if len(selected) != 1 {
		t.Errorf("expected 1 result (first oversized result included), got %d", len(selected))
	}

	md := formatMarkdown(selected)
	if !strings.Contains(md, "### Relevant Patterns") {
		t.Error("missing patterns section")
	}
}

func TestDefaultRetrievalOptions(t *testing.T) {
	opts := DefaultRetrievalOptions()
	if opts.TopK <= 0 {
		t.Errorf("expected positive TopK, got %d", opts.TopK)
	}
	if opts.MinScore <= 0 {
		t.Errorf("expected positive MinScore, got %f", opts.MinScore)
	}
	if opts.MaxTokens <= 0 {
		t.Errorf("expected positive MaxTokens, got %d", opts.MaxTokens)
	}
}

func TestSanitizeContent_TruncatesLongContent(t *testing.T) {
	long := strings.Repeat("a", maxContentLen+500)
	result := sanitizeContent(long)
	if len(result) != maxContentLen {
		t.Errorf("expected content capped at %d, got %d", maxContentLen, len(result))
	}
}

func TestSanitizeContent_StripsMarkdownHeadings(t *testing.T) {
	input := "some text\n# Heading\n## Another"
	result := sanitizeContent(input)
	if strings.Contains(result, "\n#") {
		t.Error("expected markdown headings to be stripped")
	}
}

func TestSanitizeContent_CollapsesNewlines(t *testing.T) {
	input := "a\n\n\n\nb"
	result := sanitizeContent(input)
	if strings.Contains(result, "\n\n\n") {
		t.Error("expected triple+ newlines to be collapsed")
	}
}

// fakeEmbedder returns a fixed embedding vector.
type fakeEmbedder struct {
	embedding []float64
	err       error
}

func (f *fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float64, error) {
	if f.err != nil {
		return nil, f.err
	}
	result := make([][]float64, len(texts))
	for i := range texts {
		result[i] = f.embedding
	}
	return result, nil
}

func (f *fakeEmbedder) EmbedOne(_ context.Context, _ string) ([]float64, error) {
	return f.embedding, f.err
}

// newFakeChromaServer returns an httptest server that responds to ChromaDB collection list and query APIs.
// results maps collection name -> query results to return.
func newFakeChromaServer(results map[string][]QueryResult, failCollections map[string]bool) *httptest.Server {
	// Build a map of collection name -> UUID
	collectionIDs := make(map[string]string)
	for name := range results {
		collectionIDs[name] = "uuid-" + name
	}
	// Also add collections that should fail
	for name := range failCollections {
		collectionIDs[name] = "uuid-" + name
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle GET /api/v1/collections/{name} (used by getCollectionID)
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/collections/") {
			name := strings.TrimPrefix(r.URL.Path, "/api/v1/collections/")
			if id, ok := collectionIDs[name]; ok {
				json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "name": name})
				return
			}
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Handle query
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/query") {
			// Extract collection UUID from path
			parts := strings.Split(r.URL.Path, "/")
			var collUUID string
			for i, p := range parts {
				if p == "collections" && i+1 < len(parts) {
					collUUID = parts[i+1]
					break
				}
			}

			// Find collection name by UUID
			var collName string
			for name, id := range collectionIDs {
				if id == collUUID {
					collName = name
					break
				}
			}

			if failCollections[collName] {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": "simulated failure"})
				return
			}

			qr := results[collName]
			if qr == nil {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"ids": [][]string{{}}, "documents": [][]string{{}},
					"metadatas": [][]map[string]interface{}{{}}, "distances": [][]float64{{}},
					"embeddings": [][][]float64{{}},
				})
				return
			}

			ids := make([]string, len(qr))
			docs := make([]string, len(qr))
			metas := make([]map[string]interface{}, len(qr))
			dists := make([]float64, len(qr))
			for i, r := range qr {
				ids[i] = r.Document.ID
				docs[i] = r.Document.Content
				metas[i] = r.Document.Metadata
				if metas[i] == nil {
					metas[i] = map[string]interface{}{}
				}
				dists[i] = r.Distance
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ids": [][]string{ids}, "documents": [][]string{docs},
				"metadatas": [][]map[string]interface{}{metas}, "distances": [][]float64{dists},
				"embeddings": [][][]float64{{}},
			})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
}

func TestRetrieveContext_Disabled(t *testing.T) {
	result, err := RetrieveContext(context.Background(), nil, nil, "title", "desc", nil, RetrievalOptions{Disabled: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Text != "" {
		t.Error("expected empty text when disabled")
	}
}

func TestRetrieveContext_NilClient(t *testing.T) {
	embedder := &fakeEmbedder{embedding: []float64{0.1, 0.2}}
	_, err := RetrieveContext(context.Background(), nil, embedder, "title", "desc", nil, RetrievalOptions{})
	if err == nil {
		t.Fatal("expected error for nil client")
	}
	if !strings.Contains(err.Error(), "client is nil") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRetrieveContext_NilEmbedder(t *testing.T) {
	client := NewClient("http://localhost:0")
	_, err := RetrieveContext(context.Background(), client, nil, "title", "desc", nil, RetrievalOptions{})
	if err == nil {
		t.Fatal("expected error for nil embedder")
	}
	if !strings.Contains(err.Error(), "embedder is nil") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRetrieveContext_EmbedderError(t *testing.T) {
	client := NewClient("http://localhost:0")
	embedder := &fakeEmbedder{err: fmt.Errorf("embed failure")}
	_, err := RetrieveContext(context.Background(), client, embedder, "title", "desc", nil, RetrievalOptions{})
	if err == nil {
		t.Fatal("expected error for embedder failure")
	}
	if !strings.Contains(err.Error(), "embed query") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRetrieveContext_DefaultsApplied(t *testing.T) {
	now := time.Now()
	qr := map[string][]QueryResult{
		CollectionPatterns.Name: {
			{
				Document: Document{ID: "d1", Content: "pattern content", Metadata: map[string]interface{}{
					"last_confirmed": now.Format(time.RFC3339),
				}},
				Distance: 0.1, // Score = 0.9
			},
		},
	}
	// Provide results for all collections to avoid failures
	for _, col := range AllCollections() {
		if _, ok := qr[col.Name]; !ok {
			qr[col.Name] = nil
		}
	}

	srv := newFakeChromaServer(qr, nil)
	defer srv.Close()

	client := NewClient(srv.URL)
	embedder := &fakeEmbedder{embedding: []float64{0.1}}

	// Pass zero-value opts to verify defaults are applied
	result, err := RetrieveContext(context.Background(), client, embedder, "test story", "", nil, RetrievalOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Text == "" {
		t.Error("expected non-empty text with matching results")
	}
	if len(result.DocRefs) != 1 {
		t.Errorf("expected 1 doc ref, got %d", len(result.DocRefs))
	}
	if result.DocRefs[0].Collection != CollectionPatterns.Name {
		t.Errorf("expected collection %s, got %s", CollectionPatterns.Name, result.DocRefs[0].Collection)
	}
	if result.DocRefs[0].DocID != "d1" {
		t.Errorf("expected doc ID d1, got %s", result.DocRefs[0].DocID)
	}
}

func TestRetrieveContext_FiltersBelowMinScore(t *testing.T) {
	qr := map[string][]QueryResult{
		CollectionPatterns.Name: {
			{
				Document: Document{ID: "d1", Content: "low score", Metadata: map[string]interface{}{}},
				Distance: 0.5, // Score = 0.5, below default 0.7
			},
		},
	}
	for _, col := range AllCollections() {
		if _, ok := qr[col.Name]; !ok {
			qr[col.Name] = nil
		}
	}

	srv := newFakeChromaServer(qr, nil)
	defer srv.Close()

	client := NewClient(srv.URL)
	embedder := &fakeEmbedder{embedding: []float64{0.1}}

	result, err := RetrieveContext(context.Background(), client, embedder, "test", "", nil, RetrievalOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Text != "" {
		t.Error("expected empty text when all results below MinScore")
	}
}

func TestRetrieveContext_AllCollectionsFail(t *testing.T) {
	failAll := make(map[string]bool)
	for _, col := range AllCollections() {
		failAll[col.Name] = true
	}

	srv := newFakeChromaServer(nil, failAll)
	defer srv.Close()

	client := NewClient(srv.URL)
	embedder := &fakeEmbedder{embedding: []float64{0.1}}

	_, err := RetrieveContext(context.Background(), client, embedder, "test", "", nil, RetrievalOptions{})
	if err == nil {
		t.Fatal("expected error when all collections fail")
	}
	if !strings.Contains(err.Error(), "all collection queries failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRetrieveContext_EmptyResultsFromAllCollections(t *testing.T) {
	qr := make(map[string][]QueryResult)
	for _, col := range AllCollections() {
		qr[col.Name] = nil
	}

	srv := newFakeChromaServer(qr, nil)
	defer srv.Close()

	client := NewClient(srv.URL)
	embedder := &fakeEmbedder{embedding: []float64{0.1}}

	result, err := RetrieveContext(context.Background(), client, embedder, "test", "", nil, RetrievalOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Text != "" {
		t.Error("expected empty text when all collections return empty")
	}
}

func TestRetrieveContext_TokenBudgetTruncation(t *testing.T) {
	now := time.Now()
	meta := map[string]interface{}{"last_confirmed": now.Format(time.RFC3339)}
	qr := map[string][]QueryResult{
		CollectionPatterns.Name: {
			{
				Document: Document{ID: "d1", Content: strings.Repeat("a", 400), Metadata: meta},
				Distance: 0.05, // Score = 0.95
			},
			{
				Document: Document{ID: "d2", Content: strings.Repeat("b", 400), Metadata: meta},
				Distance: 0.1, // Score = 0.9
			},
			{
				Document: Document{ID: "d3", Content: strings.Repeat("c", 400), Metadata: meta},
				Distance: 0.15, // Score = 0.85
			},
		},
	}
	for _, col := range AllCollections() {
		if _, ok := qr[col.Name]; !ok {
			qr[col.Name] = nil
		}
	}

	srv := newFakeChromaServer(qr, nil)
	defer srv.Close()

	client := NewClient(srv.URL)
	embedder := &fakeEmbedder{embedding: []float64{0.1}}

	// Set low token budget: each doc ~100 tokens, budget = 150 → should get 1 (first always included) then stop
	result, err := RetrieveContext(context.Background(), client, embedder, "test", "", nil, RetrievalOptions{
		MaxTokens: 150,
		MinScore:  0.5,
		TopK:      5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should have at most 2 results (first always included even if over budget, second might fit)
	if len(result.DocRefs) > 2 {
		t.Errorf("expected at most 2 doc refs with token budget 150, got %d", len(result.DocRefs))
	}
	if len(result.DocRefs) == 0 {
		t.Error("expected at least 1 doc ref")
	}
}

func TestNewRetriever_BothNonNil(t *testing.T) {
	client := NewClient("http://localhost:0")
	embedder := &fakeEmbedder{embedding: []float64{0.1}}
	r := NewRetriever(client, embedder)
	if r == nil {
		t.Error("expected non-nil retriever")
	}
}

func TestNewRetriever_NilClient(t *testing.T) {
	embedder := &fakeEmbedder{embedding: []float64{0.1}}
	r := NewRetriever(nil, embedder)
	if r != nil {
		t.Error("expected nil retriever for nil client")
	}
}

func TestNewRetriever_NilEmbedder(t *testing.T) {
	client := NewClient("http://localhost:0")
	r := NewRetriever(client, nil)
	if r != nil {
		t.Error("expected nil retriever for nil embedder")
	}
}
