package paths

import (
	"os"
	"path/filepath"
)

type Paths struct {
	AppDir      string
	GitHubToken string
	ConfigPath  string
}

var Default Paths

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	appDir := filepath.Join(home, ".local", "share", "copilot-api")
	Default = Paths{
		AppDir:      appDir,
		GitHubToken: filepath.Join(appDir, "github_token"),
		ConfigPath:  filepath.Join(appDir, "config.json"),
	}
}

// EnsurePaths mirrors the TypeScript ensurePaths behavior.
func EnsurePaths(p Paths) error {
	if err := os.MkdirAll(p.AppDir, 0o755); err != nil {
		return err
	}
	if err := ensureFile(p.GitHubToken); err != nil {
		return err
	}
	return ensureFile(p.ConfigPath)
}

func ensureFile(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Mode().Perm()&0o600 != 0o600 {
		return file.Chmod(0o600)
	}
	return nil
}
