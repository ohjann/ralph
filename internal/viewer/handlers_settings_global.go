package viewer

import (
	"encoding/json"
	"net/http"

	"github.com/ohjann/ralphplusplus/internal/config"
)

// GlobalSettingsResponse is GET /api/settings/global. Config is always a
// full map (defaults layered with whatever the global TOML sets) so the UI
// can render every field. Overrides names which repo fingerprints locally
// override each field, letting the UI flag "N repos override this".
type GlobalSettingsResponse struct {
	Config    map[string]interface{}        `json:"config"`
	Overrides map[string][]OverrideRepoInfo `json:"overrides"`
}

// OverrideRepoInfo identifies one repo that locally overrides a field.
type OverrideRepoInfo struct {
	FP   string `json:"fp"`
	Name string `json:"name"`
}

// handleGlobalSettingsGet serves GET /api/settings/global. It returns the
// effective global config (defaults merged with <userdata>/global-settings.toml)
// plus, per field, the list of repos that locally override it.
func (s *Server) handleGlobalSettingsGet(w http.ResponseWriter, r *http.Request) {
	gc, err := config.LoadGlobal()
	if err != nil {
		http.Error(w, "load global: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// Materialise defaults by creating a neutral cfg, applying global on
	// top, and echoing the tunable subset back as a loose map so the UI
	// does not need a hard-coded schema.
	base, err := neutralDefaults()
	if err != nil {
		http.Error(w, "defaults: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if gc != nil {
		applyToMap(gc, base)
	}

	overrides := s.collectOverrides(r)

	writeJSON(w, http.StatusOK, GlobalSettingsResponse{
		Config:    base,
		Overrides: overrides,
	})
}

// handleGlobalSettingsPost serves POST /api/settings/global. Body is a
// partial TomlConfig — nil-pointer fields mean "leave untouched" to match
// the per-repo PATCH semantics the UI already uses.
func (s *Server) handleGlobalSettingsPost(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var in config.TomlConfig
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "parse body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if errs := in.Validate(); len(errs) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":  "validation_failed",
			"fields": errs,
		})
		return
	}
	// Merge into whatever is already on disk so a single-field POST only
	// touches that one field.
	cur, err := config.LoadGlobal()
	if err != nil {
		http.Error(w, "load global: "+err.Error(), http.StatusInternalServerError)
		return
	}
	merged := mergeTomlConfig(cur, &in)
	if err := config.SaveGlobal(merged); err != nil {
		http.Error(w, "save global: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, GlobalSettingsResponse{
		Config:    tomlConfigToMap(merged),
		Overrides: s.collectOverrides(r),
	})
}

// collectOverrides scans every known repo's .ralph/config.toml and returns
// a map from setting-field to the repos that locally override it.
func (s *Server) collectOverrides(r *http.Request) map[string][]OverrideRepoInfo {
	result := make(map[string][]OverrideRepoInfo)
	repos, err := s.Index.Get(r.Context())
	if err != nil {
		return result
	}
	for _, rp := range repos {
		tc, err := config.LoadRepoOverride(rp.Meta.Path)
		if err != nil || tc == nil {
			continue
		}
		for _, field := range tc.ChangedFields() {
			result[field] = append(result[field], OverrideRepoInfo{
				FP:   rp.FP,
				Name: rp.Meta.Name,
			})
		}
	}
	return result
}

// neutralDefaults renders the package-default values the UI should show
// when the global TOML sets nothing. We build a tiny Config with the same
// defaults NewForRepo uses and project it to the wire map.
func neutralDefaults() (map[string]interface{}, error) {
	tmp, err := config.NewForRepo(".")
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"judge_enabled":         tmp.JudgeEnabled,
		"judge_max_rejections":  tmp.JudgeMaxRejections,
		"judge_test_integrity":  tmp.JudgeTestIntegrity,
		"judge_devils_advocate": tmp.JudgeDevilsAdvocate,
		"workers":               tmp.Workers,
		"workers_auto":          tmp.WorkersAuto,
		"auto_max_workers":      tmp.AutoMaxWorkers,
		"quality_review":        tmp.QualityReview,
		"quality_workers":       tmp.QualityWorkers,
		"quality_max_iterations": tmp.QualityMaxIters,
		"memory_disable":        tmp.Memory.Disabled,
		"no_architect":          tmp.NoArchitect,
		"no_simplify":           tmp.NoSimplify,
		"no_fusion":             tmp.NoFusion,
		"fusion_workers":        tmp.FusionWorkers,
		"sprite_enabled":        tmp.SpriteEnabled,
		"workspace_base":        tmp.WorkspaceBase,
		"model_override":        tmp.ModelOverride,
		"architect_model":       tmp.ArchitectModel,
		"implementer_model":     tmp.ImplementerModel,
		"utility_model":         tmp.UtilityModel,
	}, nil
}

// applyToMap overlays non-nil TomlConfig fields onto the defaults map. The
// keys match the TOML tag names so the frontend sees a single flat object.
func applyToMap(tc *config.TomlConfig, out map[string]interface{}) {
	if tc == nil {
		return
	}
	if tc.JudgeEnabled != nil {
		out["judge_enabled"] = *tc.JudgeEnabled
	}
	if tc.JudgeMaxRejections != nil {
		out["judge_max_rejections"] = *tc.JudgeMaxRejections
	}
	if tc.JudgeTestIntegrity != nil {
		out["judge_test_integrity"] = *tc.JudgeTestIntegrity
	}
	if tc.JudgeDevilsAdvocate != nil {
		out["judge_devils_advocate"] = *tc.JudgeDevilsAdvocate
	}
	if tc.Workers != nil {
		out["workers"] = *tc.Workers
	}
	if tc.WorkersAuto != nil {
		out["workers_auto"] = *tc.WorkersAuto
	}
	if tc.AutoMaxWorkers != nil {
		out["auto_max_workers"] = *tc.AutoMaxWorkers
	}
	if tc.QualityReview != nil {
		out["quality_review"] = *tc.QualityReview
	}
	if tc.QualityWorkers != nil {
		out["quality_workers"] = *tc.QualityWorkers
	}
	if tc.QualityMaxIters != nil {
		out["quality_max_iterations"] = *tc.QualityMaxIters
	}
	if tc.MemoryDisable != nil {
		out["memory_disable"] = *tc.MemoryDisable
	}
	if tc.NoArchitect != nil {
		out["no_architect"] = *tc.NoArchitect
	}
	if tc.NoSimplify != nil {
		out["no_simplify"] = *tc.NoSimplify
	}
	if tc.NoFusion != nil {
		out["no_fusion"] = *tc.NoFusion
	}
	if tc.FusionWorkers != nil {
		out["fusion_workers"] = *tc.FusionWorkers
	}
	if tc.SpriteEnabled != nil {
		out["sprite_enabled"] = *tc.SpriteEnabled
	}
	if tc.WorkspaceBase != nil {
		out["workspace_base"] = *tc.WorkspaceBase
	}
	if tc.ModelOverride != nil {
		out["model_override"] = *tc.ModelOverride
	}
	if tc.ArchitectModel != nil {
		out["architect_model"] = *tc.ArchitectModel
	}
	if tc.ImplementerModel != nil {
		out["implementer_model"] = *tc.ImplementerModel
	}
	if tc.UtilityModel != nil {
		out["utility_model"] = *tc.UtilityModel
	}
}

// mergeTomlConfig returns a new config where each field comes from patch if
// set, otherwise from base. Preserves base-unset-patch-unset as nil so the
// saved file stays minimal.
func mergeTomlConfig(base, patch *config.TomlConfig) *config.TomlConfig {
	if base == nil {
		base = &config.TomlConfig{}
	}
	if patch == nil {
		return base
	}
	out := *base
	if patch.JudgeEnabled != nil {
		out.JudgeEnabled = patch.JudgeEnabled
	}
	if patch.JudgeMaxRejections != nil {
		out.JudgeMaxRejections = patch.JudgeMaxRejections
	}
	if patch.JudgeTestIntegrity != nil {
		out.JudgeTestIntegrity = patch.JudgeTestIntegrity
	}
	if patch.JudgeDevilsAdvocate != nil {
		out.JudgeDevilsAdvocate = patch.JudgeDevilsAdvocate
	}
	if patch.Workers != nil {
		out.Workers = patch.Workers
	}
	if patch.WorkersAuto != nil {
		out.WorkersAuto = patch.WorkersAuto
	}
	if patch.AutoMaxWorkers != nil {
		out.AutoMaxWorkers = patch.AutoMaxWorkers
	}
	if patch.QualityReview != nil {
		out.QualityReview = patch.QualityReview
	}
	if patch.QualityWorkers != nil {
		out.QualityWorkers = patch.QualityWorkers
	}
	if patch.QualityMaxIters != nil {
		out.QualityMaxIters = patch.QualityMaxIters
	}
	if patch.MemoryDisable != nil {
		out.MemoryDisable = patch.MemoryDisable
	}
	if patch.NoArchitect != nil {
		out.NoArchitect = patch.NoArchitect
	}
	if patch.NoSimplify != nil {
		out.NoSimplify = patch.NoSimplify
	}
	if patch.NoFusion != nil {
		out.NoFusion = patch.NoFusion
	}
	if patch.FusionWorkers != nil {
		out.FusionWorkers = patch.FusionWorkers
	}
	if patch.SpriteEnabled != nil {
		out.SpriteEnabled = patch.SpriteEnabled
	}
	if patch.WorkspaceBase != nil {
		out.WorkspaceBase = patch.WorkspaceBase
	}
	if patch.ModelOverride != nil {
		out.ModelOverride = patch.ModelOverride
	}
	if patch.ArchitectModel != nil {
		out.ArchitectModel = patch.ArchitectModel
	}
	if patch.ImplementerModel != nil {
		out.ImplementerModel = patch.ImplementerModel
	}
	if patch.UtilityModel != nil {
		out.UtilityModel = patch.UtilityModel
	}
	return &out
}

func tomlConfigToMap(tc *config.TomlConfig) map[string]interface{} {
	out, _ := neutralDefaults()
	if out == nil {
		out = map[string]interface{}{}
	}
	applyToMap(tc, out)
	return out
}
