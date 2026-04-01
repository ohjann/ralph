package costs

import (
	"testing"
	"time"
)

func TestCalculateCost(t *testing.T) {
	pricing := PricingTable{
		"claude-opus-4-6":   {InputPricePerMToken: 15.0, OutputPricePerMToken: 75.0},
		"claude-sonnet-4-6": {InputPricePerMToken: 3.0, OutputPricePerMToken: 15.0},
		"gemini-2.5-flash":  {InputPricePerMToken: 0.15, OutputPricePerMToken: 0.60},
	}

	tests := []struct {
		name     string
		usage    TokenUsage
		expected float64
	}{
		{
			name: "claude opus known tokens",
			usage: TokenUsage{
				InputTokens:  1_000_000,
				OutputTokens: 100_000,
				Model:        "claude-opus-4-6",
			},
			expected: 15.0 + 7.5, // 15 input + 7.5 output
		},
		{
			name: "claude sonnet known tokens",
			usage: TokenUsage{
				InputTokens:  500_000,
				OutputTokens: 200_000,
				Model:        "claude-sonnet-4-6",
			},
			expected: 1.5 + 3.0, // 1.5 input + 3.0 output
		},
		{
			name: "gemini flash known tokens",
			usage: TokenUsage{
				InputTokens:  2_000_000,
				OutputTokens: 500_000,
				Model:        "gemini-2.5-flash",
			},
			expected: 0.30 + 0.30, // 0.3 input + 0.3 output
		},
		{
			name: "zero tokens returns zero cost",
			usage: TokenUsage{
				InputTokens:  0,
				OutputTokens: 0,
				Model:        "claude-opus-4-6",
			},
			expected: 0,
		},
		{
			name: "unknown model returns zero cost",
			usage: TokenUsage{
				InputTokens:  1000,
				OutputTokens: 1000,
				Model:        "unknown-model",
			},
			expected: 0,
		},
		{
			name: "only input tokens",
			usage: TokenUsage{
				InputTokens:  1_000_000,
				OutputTokens: 0,
				Model:        "claude-opus-4-6",
			},
			expected: 15.0,
		},
		{
			name: "only output tokens",
			usage: TokenUsage{
				InputTokens:  0,
				OutputTokens: 1_000_000,
				Model:        "claude-opus-4-6",
			},
			expected: 75.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateCost(tt.usage, pricing)
			if got != tt.expected {
				t.Errorf("CalculateCost() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewRunCosting(t *testing.T) {
	before := time.Now()
	rc := NewRunCosting()
	after := time.Now()

	if rc.Stories == nil {
		t.Fatal("Stories map should be initialized, got nil")
	}
	if len(rc.Stories) != 0 {
		t.Errorf("Stories map should be empty, got %d entries", len(rc.Stories))
	}
	if rc.TotalCost != 0 {
		t.Errorf("TotalCost should be 0, got %v", rc.TotalCost)
	}
	if rc.TotalInputTokens != 0 {
		t.Errorf("TotalInputTokens should be 0, got %d", rc.TotalInputTokens)
	}
	if rc.TotalOutputTokens != 0 {
		t.Errorf("TotalOutputTokens should be 0, got %d", rc.TotalOutputTokens)
	}
	if rc.StartTime.Before(before) || rc.StartTime.After(after) {
		t.Errorf("StartTime %v should be between %v and %v", rc.StartTime, before, after)
	}
}

func TestAddIteration(t *testing.T) {
	rc := NewRunCosting()

	usage1 := TokenUsage{
		InputTokens:  100_000,
		OutputTokens: 10_000,
		Model:        "claude-opus-4-6",
		Provider:     "claude",
	}
	usage2 := TokenUsage{
		InputTokens:  200_000,
		OutputTokens: 20_000,
		Model:        "claude-opus-4-6",
		Provider:     "claude",
	}

	rc.AddIteration("S-001", usage1, 30*time.Second)
	rc.AddIteration("S-001", usage2, 45*time.Second)

	sc := rc.Stories["S-001"]
	if sc == nil {
		t.Fatal("Story S-001 should exist")
	}
	if len(sc.Iterations) != 2 {
		t.Fatalf("Expected 2 iterations, got %d", len(sc.Iterations))
	}

	// Verify individual iteration costs
	cost1 := CalculateCost(usage1, DefaultPricing)
	cost2 := CalculateCost(usage2, DefaultPricing)

	if sc.Iterations[0].Cost != cost1 {
		t.Errorf("Iteration 0 cost = %v, want %v", sc.Iterations[0].Cost, cost1)
	}
	if sc.Iterations[1].Cost != cost2 {
		t.Errorf("Iteration 1 cost = %v, want %v", sc.Iterations[1].Cost, cost2)
	}

	// Verify accumulation on story
	expectedStoryCost := cost1 + cost2
	if sc.TotalCost != expectedStoryCost {
		t.Errorf("StoryCosting.TotalCost = %v, want %v", sc.TotalCost, expectedStoryCost)
	}

	// Verify RunCosting totals
	if rc.TotalCost != expectedStoryCost {
		t.Errorf("RunCosting.TotalCost = %v, want %v", rc.TotalCost, expectedStoryCost)
	}
	if rc.TotalInputTokens != 300_000 {
		t.Errorf("TotalInputTokens = %d, want 300000", rc.TotalInputTokens)
	}
	if rc.TotalOutputTokens != 30_000 {
		t.Errorf("TotalOutputTokens = %d, want 30000", rc.TotalOutputTokens)
	}
}

func TestAddJudgeCost(t *testing.T) {
	rc := NewRunCosting()

	// Add a regular iteration first
	rc.AddIteration("S-001", TokenUsage{
		InputTokens:  100_000,
		OutputTokens: 10_000,
		Model:        "claude-opus-4-6",
		Provider:     "claude",
	}, 30*time.Second)

	// Add judge cost
	judgeUsage := TokenUsage{
		InputTokens:  50_000,
		OutputTokens: 5_000,
		Model:        "gemini-2.5-flash",
		Provider:     "gemini",
	}
	rc.AddJudgeCost("S-001", judgeUsage)

	sc := rc.Stories["S-001"]
	if sc == nil {
		t.Fatal("Story S-001 should exist")
	}
	if len(sc.JudgeCosts) != 1 {
		t.Fatalf("Expected 1 judge cost, got %d", len(sc.JudgeCosts))
	}
	if sc.JudgeCosts[0].Model != "gemini-2.5-flash" {
		t.Errorf("Judge cost model = %q, want %q", sc.JudgeCosts[0].Model, "gemini-2.5-flash")
	}

	// Verify judge cost was added to story total
	iterCost := CalculateCost(TokenUsage{InputTokens: 100_000, OutputTokens: 10_000, Model: "claude-opus-4-6"}, DefaultPricing)
	judgeCost := CalculateCost(judgeUsage, DefaultPricing)
	expectedTotal := iterCost + judgeCost

	if sc.TotalCost != expectedTotal {
		t.Errorf("StoryCosting.TotalCost = %v, want %v", sc.TotalCost, expectedTotal)
	}
	if rc.TotalCost != expectedTotal {
		t.Errorf("RunCosting.TotalCost = %v, want %v", rc.TotalCost, expectedTotal)
	}
}

func TestAddJudgeCostNewStory(t *testing.T) {
	rc := NewRunCosting()

	judgeUsage := TokenUsage{
		InputTokens:  50_000,
		OutputTokens: 5_000,
		Model:        "gemini-2.5-pro",
		Provider:     "gemini",
	}
	rc.AddJudgeCost("S-NEW", judgeUsage)

	sc := rc.Stories["S-NEW"]
	if sc == nil {
		t.Fatal("Story S-NEW should be created by AddJudgeCost")
	}
	if len(sc.JudgeCosts) != 1 {
		t.Errorf("Expected 1 judge cost, got %d", len(sc.JudgeCosts))
	}
}

func TestCacheHitRate(t *testing.T) {
	rc := NewRunCosting()

	// Add iteration with cache reads
	rc.AddIteration("S-001", TokenUsage{
		InputTokens:  3000,
		OutputTokens: 500,
		CacheRead:    1000,
		Model:        "claude-opus-4-6",
		Provider:     "claude",
	}, 10*time.Second)

	rate := rc.CacheHitRate()
	// 1000 cache reads / 3000 total input = 0.333...
	expected := 1000.0 / 3000.0
	if rate != expected {
		t.Errorf("CacheHitRate() = %v, want %v", rate, expected)
	}
}

func TestCacheHitRateZeroTokens(t *testing.T) {
	rc := NewRunCosting()

	rate := rc.CacheHitRate()
	if rate != 0 {
		t.Errorf("CacheHitRate() with no tokens = %v, want 0", rate)
	}
}

func TestCacheHitRateIncludesJudgeCosts(t *testing.T) {
	rc := NewRunCosting()

	rc.AddIteration("S-001", TokenUsage{
		InputTokens:  2000,
		OutputTokens: 100,
		CacheRead:    500,
		Model:        "claude-opus-4-6",
	}, 10*time.Second)

	rc.AddJudgeCost("S-001", TokenUsage{
		InputTokens:  1000,
		OutputTokens: 100,
		CacheRead:    300,
		Model:        "gemini-2.5-flash",
	})

	rate := rc.CacheHitRate()
	// (500 + 300) / (2000 + 1000) = 800/3000
	expected := 800.0 / 3000.0
	if rate != expected {
		t.Errorf("CacheHitRate() = %v, want %v", rate, expected)
	}
}

func TestCombineUsage(t *testing.T) {
	tests := []struct {
		name string
		a, b *TokenUsage
		want *TokenUsage
	}{
		{"both nil", nil, nil, nil},
		{"a nil", nil, &TokenUsage{InputTokens: 10, Model: "m"}, &TokenUsage{InputTokens: 10, Model: "m"}},
		{"b nil", &TokenUsage{InputTokens: 10, Model: "m"}, nil, &TokenUsage{InputTokens: 10, Model: "m"}},
		{"sums fields", &TokenUsage{
			InputTokens: 100, OutputTokens: 50, CacheRead: 10, CacheWrite: 5,
			Model: "old", Provider: "p1", NumTurns: 3, DurationMS: 1000,
		}, &TokenUsage{
			InputTokens: 200, OutputTokens: 100, CacheRead: 20, CacheWrite: 10,
			Model: "new", Provider: "p2", NumTurns: 5, DurationMS: 2000,
		}, &TokenUsage{
			InputTokens: 300, OutputTokens: 150, CacheRead: 30, CacheWrite: 15,
			Model: "new", Provider: "p2", NumTurns: 8, DurationMS: 3000,
		}},
		{"b empty model keeps a model", &TokenUsage{Model: "kept"}, &TokenUsage{Model: ""}, &TokenUsage{Model: "kept"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CombineUsage(tt.a, tt.b)
			if tt.want == nil {
				if got != nil {
					t.Errorf("CombineUsage() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("CombineUsage() = nil, want %v", tt.want)
			}
			if *got != *tt.want {
				t.Errorf("CombineUsage() = %+v, want %+v", *got, *tt.want)
			}
		})
	}
}

func TestMultipleStoriesTrackedIndependently(t *testing.T) {
	rc := NewRunCosting()

	usage1 := TokenUsage{
		InputTokens:  100_000,
		OutputTokens: 10_000,
		Model:        "claude-opus-4-6",
		Provider:     "claude",
	}
	usage2 := TokenUsage{
		InputTokens:  200_000,
		OutputTokens: 20_000,
		Model:        "claude-sonnet-4-6",
		Provider:     "claude",
	}

	rc.AddIteration("S-001", usage1, 30*time.Second)
	rc.AddIteration("S-002", usage2, 45*time.Second)

	// Verify stories exist independently
	if len(rc.Stories) != 2 {
		t.Fatalf("Expected 2 stories, got %d", len(rc.Stories))
	}

	sc1 := rc.Stories["S-001"]
	sc2 := rc.Stories["S-002"]

	cost1 := CalculateCost(usage1, DefaultPricing)
	cost2 := CalculateCost(usage2, DefaultPricing)

	if sc1.TotalCost != cost1 {
		t.Errorf("S-001 TotalCost = %v, want %v", sc1.TotalCost, cost1)
	}
	if sc2.TotalCost != cost2 {
		t.Errorf("S-002 TotalCost = %v, want %v", sc2.TotalCost, cost2)
	}

	// Verify run totals aggregate both stories
	if rc.TotalCost != cost1+cost2 {
		t.Errorf("RunCosting.TotalCost = %v, want %v", rc.TotalCost, cost1+cost2)
	}
	if rc.TotalInputTokens != 300_000 {
		t.Errorf("TotalInputTokens = %d, want 300000", rc.TotalInputTokens)
	}
	if rc.TotalOutputTokens != 30_000 {
		t.Errorf("TotalOutputTokens = %d, want 30000", rc.TotalOutputTokens)
	}

	// Verify each story has correct number of iterations
	if len(sc1.Iterations) != 1 {
		t.Errorf("S-001 iterations = %d, want 1", len(sc1.Iterations))
	}
	if len(sc2.Iterations) != 1 {
		t.Errorf("S-002 iterations = %d, want 1", len(sc2.Iterations))
	}
}
