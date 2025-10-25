package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"internal/state"
)

const (
	copilotVersion       = "0.26.7"
	editorPluginVersion  = "copilot-chat/" + copilotVersion
	userAgent            = "GitHubCopilotChat/" + copilotVersion
	apiVersion           = "2025-04-01"
	githubAPIBaseURL     = "https://api.github.com"
	githubBaseURL        = "https://github.com"
	githubClientID       = "Iv1.b507a08c87ecfe98"
	githubAppScopes      = "read:user"
	vscodeUserAgentValue = "electron-fetch"
)

// StandardHeaders returns the common JSON headers.
func StandardHeaders() map[string]string {
	return map[string]string{
		"content-type": "application/json",
		"accept":       "application/json",
	}
}

func randomUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}

	// Variant bits; see RFC 4122 §4.4
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}

// CopilotBaseURL returns the Copilot API base url depending on account type.
func CopilotBaseURL(s *state.State) string {
	accountType := "individual"
	s.Read(func(st *state.State) {
		if st.AccountType != "" {
			accountType = st.AccountType
		}
	})

	if accountType == "individual" {
		return "https://api.githubcopilot.com"
	}
	return fmt.Sprintf("https://api.%s.githubcopilot.com", accountType)
}

// CopilotHeaderOptions mirrors the TypeScript options bag.
type CopilotHeaderOptions struct {
	Vision    bool
	Initiator string
}

// CopilotHeaders builds request headers for Copilot API calls.
func CopilotHeaders(s *state.State, opts CopilotHeaderOptions) map[string]string {
	headers := map[string]string{
		"content-type":                        "application/json",
		"Authorization":                       "Bearer " + getCopilotToken(s),
		"copilot-integration-id":              "vscode-chat",
		"editor-version":                      "vscode/" + getVSCodeVersion(s),
		"editor-plugin-version":               editorPluginVersion,
		"user-agent":                          userAgent,
		"openai-intent":                       "conversation-panel",
		"x-github-api-version":                apiVersion,
		"x-request-id":                        randomUUID(),
		"x-vscode-user-agent-library-version": vscodeUserAgentValue,
	}

	if opts.Vision {
		headers["copilot-vision-request"] = "true"
	}
	if strings.TrimSpace(opts.Initiator) != "" {
		headers["X-Initiator"] = opts.Initiator
	}

	return headers
}

func getCopilotToken(s *state.State) string {
	var token string
	s.Read(func(st *state.State) {
		token = st.CopilotToken
	})
	return token
}

func getVSCodeVersion(s *state.State) string {
	version := ""
	s.Read(func(st *state.State) {
		version = st.VSCodeVersion
	})
	return version
}

// GitHubHeaders returns headers for GitHub API requests.
func GitHubHeaders(s *state.State) map[string]string {
	token := ""
	vscodeVersion := ""
	s.Read(func(st *state.State) {
		token = st.GitHubToken
		vscodeVersion = st.VSCodeVersion
	})

	headers := map[string]string{
		"authorization":                       "token " + token,
		"content-type":                        "application/json",
		"accept":                              "application/json",
		"editor-version":                      "vscode/" + vscodeVersion,
		"editor-plugin-version":               editorPluginVersion,
		"user-agent":                          userAgent,
		"x-github-api-version":                apiVersion,
		"x-vscode-user-agent-library-version": vscodeUserAgentValue,
	}

	return headers
}

// Constants exported for reuse.
const (
	GitHubAPIBaseURL = githubAPIBaseURL
	GitHubBaseURL    = githubBaseURL
	GitHubClientID   = githubClientID
	GitHubAppScopes  = githubAppScopes
)
