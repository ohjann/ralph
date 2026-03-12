package worker

import (
	"context"
	"testing"

	"github.com/eoghanhynes/ralph/internal/config"
	"github.com/eoghanhynes/ralph/internal/memory"
	"github.com/eoghanhynes/ralph/internal/runner"
)

// fakeEmbedder implements memory.Embedder for testing.
type fakeEmbedder struct {
	embedding []float64
}

func (f *fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i := range texts {
		result[i] = f.embedding
	}
	return result, nil
}

func (f *fakeEmbedder) EmbedOne(_ context.Context, _ string) ([]float64, error) {
	return f.embedding, nil
}

// buildPromptOpts mirrors the logic in Run() that constructs BuildPromptOpts
// from worker fields and config. This is extracted to be independently testable
// since Run() has heavy side effects (workspace creation, Claude invocation, etc.).
func buildPromptOpts(w *Worker, cfg *config.Config) []runner.BuildPromptOpts {
	var opts []runner.BuildPromptOpts
	if w.ChromaClient != nil && w.Embedder != nil && !cfg.Memory.Disabled {
		retriever := memory.NewRetriever(w.ChromaClient, w.Embedder)
		if retriever != nil {
			opts = append(opts, runner.BuildPromptOpts{
				Memory: retriever,
				MemoryOpts: memory.RetrievalOptions{
					TopK:      cfg.Memory.TopK,
					MinScore:  cfg.Memory.MinScore,
					MaxTokens: cfg.Memory.MaxTokens,
				},
			})
		}
	}
	return opts
}

// TestBuildPromptOpts_WithMemory verifies that when a Worker has ChromaClient
// and Embedder set, BuildPromptOpts are constructed with a valid retriever and
// the config's memory options.
func TestBuildPromptOpts_WithMemory(t *testing.T) {
	client := memory.NewClient("http://localhost:0")
	embedder := &fakeEmbedder{embedding: []float64{0.1, 0.2}}

	w := &Worker{
		ChromaClient: client,
		Embedder:     embedder,
	}
	cfg := &config.Config{
		Memory: config.MemoryConfig{
			TopK:      10,
			MinScore:  0.7,
			MaxTokens: 500,
		},
	}

	opts := buildPromptOpts(w, cfg)

	if len(opts) != 1 {
		t.Fatalf("expected 1 BuildPromptOpts, got %d", len(opts))
	}
	if opts[0].Memory == nil {
		t.Error("expected non-nil Memory retriever")
	}
	if opts[0].MemoryOpts.TopK != 10 {
		t.Errorf("TopK = %d, want 10", opts[0].MemoryOpts.TopK)
	}
	if opts[0].MemoryOpts.MinScore != 0.7 {
		t.Errorf("MinScore = %f, want 0.7", opts[0].MemoryOpts.MinScore)
	}
	if opts[0].MemoryOpts.MaxTokens != 500 {
		t.Errorf("MaxTokens = %d, want 500", opts[0].MemoryOpts.MaxTokens)
	}
}

// TestBuildPromptOpts_NilMemory verifies that workers with nil ChromaClient or
// Embedder produce no BuildPromptOpts (memory is disabled/unavailable).
func TestBuildPromptOpts_NilMemory(t *testing.T) {
	cfg := &config.Config{}

	tests := []struct {
		name   string
		worker *Worker
	}{
		{
			name:   "both nil",
			worker: &Worker{},
		},
		{
			name: "nil embedder",
			worker: &Worker{
				ChromaClient: memory.NewClient("http://localhost:0"),
			},
		},
		{
			name: "nil client",
			worker: &Worker{
				Embedder: &fakeEmbedder{embedding: []float64{0.1}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := buildPromptOpts(tt.worker, cfg)
			if len(opts) != 0 {
				t.Errorf("expected 0 BuildPromptOpts for %s, got %d", tt.name, len(opts))
			}
		})
	}
}

// TestBuildPromptOpts_MemoryDisabled verifies that even when ChromaClient and
// Embedder are set, no BuildPromptOpts are created if memory is disabled in config.
func TestBuildPromptOpts_MemoryDisabled(t *testing.T) {
	client := memory.NewClient("http://localhost:0")
	embedder := &fakeEmbedder{embedding: []float64{0.1, 0.2}}

	w := &Worker{
		ChromaClient: client,
		Embedder:     embedder,
	}
	cfg := &config.Config{
		Memory: config.MemoryConfig{
			Disabled: true,
			TopK:     10,
			MinScore: 0.7,
		},
	}

	opts := buildPromptOpts(w, cfg)

	if len(opts) != 0 {
		t.Errorf("expected 0 BuildPromptOpts when memory disabled, got %d", len(opts))
	}
}
