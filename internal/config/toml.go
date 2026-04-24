package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/ohjann/ralphplusplus/internal/debuglog"
	"github.com/ohjann/ralphplusplus/internal/lockfile"
)

// TomlConfig mirrors the tunable subset of Config.
// Pointer fields distinguish "not set in file" (nil) from "set to zero value".
// Both TOML and JSON tags are present because the struct is decoded from
// TOML (config.toml) and JSON (daemon /api/settings).
type TomlConfig struct {
	JudgeEnabled        *bool   `toml:"judge_enabled" json:"judge_enabled,omitempty"`
	JudgeMaxRejections  *int    `toml:"judge_max_rejections" json:"judge_max_rejections,omitempty"`
	JudgeTestIntegrity  *bool   `toml:"judge_test_integrity" json:"judge_test_integrity,omitempty"`
	JudgeDevilsAdvocate *bool   `toml:"judge_devils_advocate" json:"judge_devils_advocate,omitempty"`
	Workers            *int    `toml:"workers" json:"workers,omitempty"`
	WorkersAuto        *bool   `toml:"workers_auto" json:"workers_auto,omitempty"`
	AutoMaxWorkers     *int    `toml:"auto_max_workers" json:"auto_max_workers,omitempty"`
	QualityReview      *bool   `toml:"quality_review" json:"quality_review,omitempty"`
	QualityWorkers     *int    `toml:"quality_workers" json:"quality_workers,omitempty"`
	QualityMaxIters    *int    `toml:"quality_max_iterations" json:"quality_max_iterations,omitempty"`
	MemoryDisable      *bool   `toml:"memory_disable" json:"memory_disable,omitempty"`
	NoArchitect        *bool   `toml:"no_architect" json:"no_architect,omitempty"`
	SpriteEnabled      *bool   `toml:"sprite_enabled" json:"sprite_enabled,omitempty"`
	WorkspaceBase      *string `toml:"workspace_base" json:"workspace_base,omitempty"`
	ModelOverride      *string `toml:"model_override" json:"model_override,omitempty"`
	ArchitectModel     *string `toml:"architect_model" json:"architect_model,omitempty"`
	ImplementerModel   *string `toml:"implementer_model" json:"implementer_model,omitempty"`
	UtilityModel       *string `toml:"utility_model" json:"utility_model,omitempty"`
	NoSimplify         *bool   `toml:"no_simplify" json:"no_simplify,omitempty"`
	NoFusion           *bool   `toml:"no_fusion" json:"no_fusion,omitempty"`
	FusionWorkers      *int    `toml:"fusion_workers" json:"fusion_workers,omitempty"`
}

// loadTomlConfig reads .ralph/config.toml from the given project directory.
// Returns nil, nil if the file does not exist.
func loadTomlConfig(projectDir string) (*TomlConfig, error) {
	path := filepath.Join(projectDir, ".ralph", "config.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var tc TomlConfig
	if err := toml.Unmarshal(data, &tc); err != nil {
		return nil, err
	}

	debuglog.Log("config.toml: loaded from %s", path)
	return &tc, nil
}

// Validate checks incoming settings for invariant violations.
// Returns a field-keyed map of error messages; an empty map means valid.
func (tc *TomlConfig) Validate() map[string]string {
	errs := map[string]string{}
	if tc == nil {
		return errs
	}
	if tc.Workers != nil && *tc.Workers < 1 {
		errs["workers"] = "must be >= 1"
	}
	if tc.AutoMaxWorkers != nil && tc.Workers != nil && *tc.AutoMaxWorkers < *tc.Workers {
		errs["auto_max_workers"] = "must be >= workers"
	}
	if tc.QualityWorkers != nil && *tc.QualityWorkers < 1 {
		errs["quality_workers"] = "must be >= 1"
	}
	if tc.QualityMaxIters != nil && *tc.QualityMaxIters < 1 {
		errs["quality_max_iterations"] = "must be >= 1"
	}
	if tc.FusionWorkers != nil && *tc.FusionWorkers < 2 {
		errs["fusion_workers"] = "must be >= 2"
	}
	if tc.JudgeMaxRejections != nil && *tc.JudgeMaxRejections < 0 {
		errs["judge_max_rejections"] = "must be >= 0"
	}
	return errs
}

// ChangedFields returns the TOML tag names of every non-nil field in tc.
// Used by ApplySettings to report which settings were overwritten.
func (tc *TomlConfig) ChangedFields() []string {
	if tc == nil {
		return nil
	}
	out := []string{}
	if tc.JudgeEnabled != nil {
		out = append(out, "judge_enabled")
	}
	if tc.JudgeMaxRejections != nil {
		out = append(out, "judge_max_rejections")
	}
	if tc.JudgeTestIntegrity != nil {
		out = append(out, "judge_test_integrity")
	}
	if tc.JudgeDevilsAdvocate != nil {
		out = append(out, "judge_devils_advocate")
	}
	if tc.Workers != nil {
		out = append(out, "workers")
	}
	if tc.WorkersAuto != nil {
		out = append(out, "workers_auto")
	}
	if tc.AutoMaxWorkers != nil {
		out = append(out, "auto_max_workers")
	}
	if tc.QualityReview != nil {
		out = append(out, "quality_review")
	}
	if tc.QualityWorkers != nil {
		out = append(out, "quality_workers")
	}
	if tc.QualityMaxIters != nil {
		out = append(out, "quality_max_iterations")
	}
	if tc.MemoryDisable != nil {
		out = append(out, "memory_disable")
	}
	if tc.NoArchitect != nil {
		out = append(out, "no_architect")
	}
	if tc.SpriteEnabled != nil {
		out = append(out, "sprite_enabled")
	}
	if tc.WorkspaceBase != nil {
		out = append(out, "workspace_base")
	}
	if tc.ModelOverride != nil {
		out = append(out, "model_override")
	}
	if tc.ArchitectModel != nil {
		out = append(out, "architect_model")
	}
	if tc.ImplementerModel != nil {
		out = append(out, "implementer_model")
	}
	if tc.UtilityModel != nil {
		out = append(out, "utility_model")
	}
	if tc.NoSimplify != nil {
		out = append(out, "no_simplify")
	}
	if tc.NoFusion != nil {
		out = append(out, "no_fusion")
	}
	if tc.FusionWorkers != nil {
		out = append(out, "fusion_workers")
	}
	return out
}

// applyTo overwrites non-nil fields onto the given Config. Must be called
// with cfg.mu held for writing when applying to a live, shared Config.
func (tc *TomlConfig) applyTo(cfg *Config) {
	if tc == nil {
		return
	}
	if tc.JudgeEnabled != nil {
		cfg.JudgeEnabled = *tc.JudgeEnabled
	}
	if tc.JudgeMaxRejections != nil {
		cfg.JudgeMaxRejections = *tc.JudgeMaxRejections
	}
	if tc.JudgeTestIntegrity != nil {
		cfg.JudgeTestIntegrity = *tc.JudgeTestIntegrity
	}
	if tc.JudgeDevilsAdvocate != nil {
		cfg.JudgeDevilsAdvocate = *tc.JudgeDevilsAdvocate
	}
	if tc.Workers != nil {
		cfg.Workers = *tc.Workers
	}
	if tc.WorkersAuto != nil {
		cfg.WorkersAuto = *tc.WorkersAuto
	}
	if tc.AutoMaxWorkers != nil {
		cfg.AutoMaxWorkers = *tc.AutoMaxWorkers
	}
	if tc.QualityReview != nil {
		cfg.QualityReview = *tc.QualityReview
	}
	if tc.QualityWorkers != nil {
		cfg.QualityWorkers = *tc.QualityWorkers
	}
	if tc.QualityMaxIters != nil {
		cfg.QualityMaxIters = *tc.QualityMaxIters
	}
	if tc.MemoryDisable != nil {
		cfg.Memory.Disabled = *tc.MemoryDisable
	}
	if tc.NoArchitect != nil {
		cfg.NoArchitect = *tc.NoArchitect
	}
	if tc.SpriteEnabled != nil {
		cfg.SpriteEnabled = *tc.SpriteEnabled
	}
	if tc.WorkspaceBase != nil {
		cfg.WorkspaceBase = *tc.WorkspaceBase
	}
	if tc.ModelOverride != nil {
		cfg.ModelOverride = *tc.ModelOverride
	}
	if tc.ArchitectModel != nil {
		cfg.ArchitectModel = *tc.ArchitectModel
	}
	if tc.ImplementerModel != nil {
		cfg.ImplementerModel = *tc.ImplementerModel
	}
	if tc.UtilityModel != nil {
		cfg.UtilityModel = *tc.UtilityModel
	}
	if tc.NoSimplify != nil {
		cfg.NoSimplify = *tc.NoSimplify
	}
	if tc.NoFusion != nil {
		cfg.NoFusion = *tc.NoFusion
	}
	if tc.FusionWorkers != nil {
		cfg.FusionWorkers = *tc.FusionWorkers
	}
}

// SaveConfig writes the current tunable settings to .ralph/config.toml.
// Writes are serialized across processes via an fcntl advisory lock on
// .ralph/config.toml.lock, and land atomically via a tmp file + os.Rename
// so concurrent writers always see a complete file (one of the writes wins
// in full; there is no intermediate half-written state).
func (cfg *Config) SaveConfig() error {
	cfg.mu.RLock()
	tc := TomlConfig{
		JudgeEnabled:        boolPtr(cfg.JudgeEnabled),
		JudgeMaxRejections:  intPtr(cfg.JudgeMaxRejections),
		JudgeTestIntegrity:  boolPtr(cfg.JudgeTestIntegrity),
		JudgeDevilsAdvocate: boolPtr(cfg.JudgeDevilsAdvocate),
		Workers:             intPtr(cfg.Workers),
		WorkersAuto:        boolPtr(cfg.WorkersAuto),
		AutoMaxWorkers:     intPtr(cfg.AutoMaxWorkers),
		QualityReview:      boolPtr(cfg.QualityReview),
		QualityWorkers:     intPtr(cfg.QualityWorkers),
		QualityMaxIters:    intPtr(cfg.QualityMaxIters),
		MemoryDisable:      boolPtr(cfg.Memory.Disabled),
		NoArchitect:        boolPtr(cfg.NoArchitect),
		SpriteEnabled:      boolPtr(cfg.SpriteEnabled),
		WorkspaceBase:      stringPtr(cfg.WorkspaceBase),
		ModelOverride:      stringPtr(cfg.ModelOverride),
		ArchitectModel:     stringPtr(cfg.ArchitectModel),
		ImplementerModel:   stringPtr(cfg.ImplementerModel),
		UtilityModel:       stringPtr(cfg.UtilityModel),
		NoSimplify:         boolPtr(cfg.NoSimplify),
		NoFusion:           boolPtr(cfg.NoFusion),
		FusionWorkers:      intPtr(cfg.FusionWorkers),
	}
	dir := filepath.Join(cfg.ProjectDir, ".ralph")
	cfg.mu.RUnlock()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	var buf bytes.Buffer
	buf.WriteString("# Ralph configuration — saved settings across runs.\n")
	buf.WriteString("# CLI flags override these values.\n\n")
	if err := toml.NewEncoder(&buf).Encode(tc); err != nil {
		return err
	}

	target := filepath.Join(dir, "config.toml")
	lockPath := target + ".lock"

	lock, err := lockfile.Acquire(lockPath)
	if err != nil {
		return fmt.Errorf("lock config.toml: %w", err)
	}
	defer lock.Release()

	tmp, err := os.CreateTemp(dir, "config.toml.*.tmp")
	if err != nil {
		return fmt.Errorf("create temp config.toml: %w", err)
	}
	tmpPath := tmp.Name()
	// Best-effort cleanup if we fail before the rename.
	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := tmp.Write(buf.Bytes()); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write temp config.toml: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("chmod temp config.toml: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("sync temp config.toml: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp config.toml: %w", err)
	}
	if err := os.Rename(tmpPath, target); err != nil {
		cleanup()
		return fmt.Errorf("rename config.toml: %w", err)
	}
	debuglog.Log("config.toml: saved %d bytes to %s", buf.Len(), target)
	return nil
}

func boolPtr(v bool) *bool      { return &v }
func intPtr(v int) *int         { return &v }
func stringPtr(v string) *string { return &v }
