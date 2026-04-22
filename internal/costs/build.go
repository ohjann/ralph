package costs

import (
	"time"

	"github.com/ohjann/ralphplusplus/internal/events"
	"github.com/ohjann/ralphplusplus/internal/prd"
)

// BuildInputs is the mode-agnostic view of a run needed to produce a
// RunSummary. Callers (TUI, daemon) resolve mode-dependent fields like
// TotalIterations, FailedCount, FirstPassRate from their own state and
// pass the computed values in.
type BuildInputs struct {
	PRD             *prd.PRD
	TotalIterations int
	FailedCount     int
	FirstPassRate   float64

	RunCosting    *RunCosting    // optional; supplies cost/token/model data
	Events        []events.Event // optional; read for stuck count and judge rejects
	FusionMetrics *FusionMetrics

	StartTime time.Time

	// Config flags carried into RunSummary verbatim.
	Workers       int
	NoArchitect   bool
	NoFusion      bool
	NoSimplify    bool
	QualityReview bool
	FusionWorkers int

	// Kind attributes the run in history (daemon, retro, memory-consolidate, ad-hoc).
	// Callers should set it; empty is accepted.
	Kind string
}

// BuildRunSummary assembles a RunSummary from the provided inputs. It
// reads PRD.UserStories for completion counts, aggregates events for
// judge/stuck metrics, and pulls cost/token data from RunCosting.
//
// The function is pure with respect to the inputs (no disk reads beyond
// what's already materialized in in.Events). Callers persist via
// AppendRun separately.
func BuildRunSummary(in BuildInputs) RunSummary {
	p := in.PRD

	var completed int
	if p != nil {
		for _, s := range p.UserStories {
			if s.Passes {
				completed++
			}
		}
	}

	var avgIter float64
	if completed > 0 {
		avgIter = float64(in.TotalIterations) / float64(completed)
	}

	// Judge metrics + stuck count from events.
	var judgeTotal, judgeRejections, stuckCount int
	judgeRejectsPerStory := make(map[string]int)
	for _, e := range in.Events {
		switch e.Type {
		case events.EventJudgeResult:
			judgeTotal++
			if e.Meta["verdict"] == "fail" {
				judgeRejections++
				judgeRejectsPerStory[e.StoryID]++
			}
		case events.EventStuck:
			stuckCount++
		}
	}

	var rejectionRate float64
	if judgeTotal > 0 {
		rejectionRate = float64(judgeRejections) / float64(judgeTotal)
	}

	// Cost/token/model data from the costing snapshot.
	var totalCost float64
	var totalInputTokens, totalOutputTokens int
	var cacheHitRate float64
	modelsSet := make(map[string]bool)
	storyIterCounts := make(map[string]int)
	storyModels := make(map[string]string)

	if in.RunCosting != nil {
		snap := in.RunCosting.Snapshot()
		totalCost = snap.TotalCost
		totalInputTokens = snap.TotalInputTokens
		totalOutputTokens = snap.TotalOutputTokens
		cacheHitRate = in.RunCosting.CacheHitRate()

		for storyID, sc := range snap.Stories {
			storyIterCounts[storyID] = len(sc.Iterations)
			for _, ic := range sc.Iterations {
				if ic.TokenUsage.Model != "" {
					modelsSet[ic.TokenUsage.Model] = true
					storyModels[storyID] = ic.TokenUsage.Model
				}
			}
		}
	}

	var modelsUsed []string
	for model := range modelsSet {
		modelsUsed = append(modelsUsed, model)
	}

	var storyDetails []StorySummary
	if p != nil {
		for _, s := range p.UserStories {
			storyDetails = append(storyDetails, StorySummary{
				StoryID:      s.ID,
				Title:        s.Title,
				Iterations:   storyIterCounts[s.ID],
				Passed:       s.Passes,
				JudgeRejects: judgeRejectsPerStory[s.ID],
				Model:        storyModels[s.ID],
			})
		}
	}

	durationMinutes := time.Since(in.StartTime).Minutes()

	project := ""
	storiesTotal := 0
	if p != nil {
		project = p.Project
		storiesTotal = len(p.UserStories)
	}

	return RunSummary{
		PRD:                   project,
		Date:                  time.Now().Format(time.RFC3339),
		StoriesTotal:          storiesTotal,
		StoriesCompleted:      completed,
		StoriesFailed:         in.FailedCount,
		TotalCost:             totalCost,
		DurationMinutes:       durationMinutes,
		TotalIterations:       in.TotalIterations,
		AvgIterationsPerStory: avgIter,
		StuckCount:            stuckCount,
		JudgeRejectionRate:    rejectionRate,
		FirstPassRate:         in.FirstPassRate,
		ModelsUsed:            modelsUsed,
		TotalInputTokens:      totalInputTokens,
		TotalOutputTokens:     totalOutputTokens,
		CacheHitRate:          cacheHitRate,
		StoryDetails:          storyDetails,
		Workers:               in.Workers,
		NoArchitect:           in.NoArchitect,
		NoFusion:              in.NoFusion,
		NoSimplify:            in.NoSimplify,
		QualityReview:         in.QualityReview,
		FusionWorkers:         in.FusionWorkers,
		FusionMetrics:         in.FusionMetrics,
		Kind:                  in.Kind,
	}
}
