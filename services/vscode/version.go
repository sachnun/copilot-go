package vscode

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"time"
)

const fallbackVersion = "1.104.3"

var pkgverRegex = regexp.MustCompile(`pkgver=([0-9.]+)`)

// GetVersion replicates the TypeScript logic of scraping the AUR package.
func GetVersion(ctx context.Context, client *http.Client) string {
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://aur.archlinux.org/cgit/aur.git/plain/PKGBUILD?h=visual-studio-code-bin", nil)
	if err != nil {
		return fallbackVersion
	}

	ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return fallbackVersion
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fallbackVersion
	}

	matches := pkgverRegex.FindSubmatch(body)
	if len(matches) == 2 {
		return string(matches[1])
	}

	return fallbackVersion
}
