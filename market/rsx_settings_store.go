package market

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const defaultRSXSettingsFile = "rsx_settings.json"

var (
	rsxSettingsPathMu sync.RWMutex
	rsxSettingsPath   = defaultRSXSettingsFile
)

// SetRSXSettingsPath overrides the autosave file (tests). Empty resets to default.
func SetRSXSettingsPath(path string) {
	rsxSettingsPathMu.Lock()
	defer rsxSettingsPathMu.Unlock()
	if path == "" {
		rsxSettingsPath = defaultRSXSettingsFile
		return
	}
	rsxSettingsPath = path
}

func rsxSettingsFilePath() string {
	rsxSettingsPathMu.RLock()
	defer rsxSettingsPathMu.RUnlock()
	return rsxSettingsPath
}

// LoadRSXSettingsFromDisk loads engine RSX SSOT from autosave file if present.
// Missing file is not an error (defaults remain). Corrupt file logs and keeps defaults.
func LoadRSXSettingsFromDisk() error {
	path := rsxSettingsFilePath()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read RSX settings %s: %w", path, err)
	}
	var stored RSXSettings
	if err := json.Unmarshal(raw, &stored); err != nil {
		return fmt.Errorf("parse RSX settings %s: %w", path, err)
	}
	replaceRSXSettingsForBoot(stored)
	return nil
}

// saveRSXSettingsToDisk writes the current SSOT (caller holds or uses Get).
func saveRSXSettingsToDisk(settings RSXSettings) error {
	path := rsxSettingsFilePath()
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir RSX settings: %w", err)
		}
	}
	raw, err := json.MarshalIndent(NormalizeRSXSettings(settings), "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("write RSX settings tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		// Fallback: direct write (some sandboxes block rename onto ignored paths).
		if err2 := os.WriteFile(path, raw, 0o644); err2 != nil {
			return fmt.Errorf("rename RSX settings: %w (fallback write: %v)", err, err2)
		}
	}
	return nil
}
