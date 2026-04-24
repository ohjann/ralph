package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/ohjann/ralphplusplus/internal/debuglog"
	"github.com/ohjann/ralphplusplus/internal/lockfile"
	"github.com/ohjann/ralphplusplus/internal/userdata"
)

// globalSettingsFilename lives at <userdata>/ralph/global-settings.toml so
// every daemon and the viewer can read the same user-wide defaults.
const globalSettingsFilename = "global-settings.toml"

// GlobalConfigPath returns the absolute path to the user-wide settings file.
// Safe to call even when the file does not yet exist.
func GlobalConfigPath() (string, error) {
	d, err := userdata.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, globalSettingsFilename), nil
}

// LoadGlobal reads the user-wide settings TOML. Returns nil, nil when the
// file is absent so callers can treat missing as "no overrides".
func LoadGlobal() (*TomlConfig, error) {
	path, err := GlobalConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var tc TomlConfig
	if err := toml.Unmarshal(data, &tc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &tc, nil
}

// SaveGlobal writes tc to the user-wide settings TOML, mirroring
// Config.SaveConfig's atomic-rename + advisory-lock pattern. Callers pass
// a TomlConfig with nil-pointer fields for "leave unset".
func SaveGlobal(tc *TomlConfig) error {
	if tc == nil {
		tc = &TomlConfig{}
	}
	path, err := GlobalConfigPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	var buf bytes.Buffer
	buf.WriteString("# Ralph user-wide settings — applied as defaults to every run.\n")
	buf.WriteString("# Repo-local .ralph/config.toml overrides these; CLI flags override both.\n\n")
	if err := toml.NewEncoder(&buf).Encode(tc); err != nil {
		return err
	}

	lock, err := lockfile.Acquire(path + ".lock")
	if err != nil {
		return fmt.Errorf("lock global settings: %w", err)
	}
	defer lock.Release()

	tmp, err := os.CreateTemp(dir, globalSettingsFilename+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp global settings: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	_ = tmp.Chmod(0o644)
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return err
	}
	debuglog.Log("global settings: saved %d bytes to %s", buf.Len(), path)
	return nil
}

// LoadRepoOverride reads <repoPath>/.ralph/config.toml using the package's
// existing loader. Exposed so callers (the viewer's override scan) can
// detect which fields a repo locally overrides without threading through
// a full Config construction.
func LoadRepoOverride(repoPath string) (*TomlConfig, error) {
	return loadTomlConfig(repoPath)
}
