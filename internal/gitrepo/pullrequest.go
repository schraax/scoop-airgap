package gitrepo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type prRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Head  string `json:"head"`
	Base  string `json:"base"`
}

// CreatePR opens a pull request from head into base on the remote platform.
// It auto-detects GitHub (github.com) from the URL and falls back to the
// Gitea/Forgejo-compatible API for any other host.
func CreatePR(remoteURL, authToken, base, head, title, body string) error {
	endpoint, authHeader, err := prEndpoint(remoteURL, authToken)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(prRequest{Title: title, Body: body, Head: head, Base: base})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("PR API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("PR API returned %s for %s", resp.Status, endpoint)
	}
	return nil
}

// prEndpoint derives the full PR API endpoint URL and the Authorization header
// value from the repository remote URL and token.
func prEndpoint(remoteURL, authToken string) (endpoint, authHeader string, err error) {
	u, err := url.Parse(remoteURL)
	if err != nil {
		return "", "", fmt.Errorf("parse remote URL %q: %w", remoteURL, err)
	}
	u.User = nil // strip any embedded credentials

	ownerRepo := strings.TrimPrefix(strings.TrimSuffix(u.Path, ".git"), "/")
	parts := strings.SplitN(ownerRepo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("cannot parse owner/repo from URL %q", remoteURL)
	}
	owner, repo := parts[0], parts[1]

	var apiRoot, tokenScheme string
	if u.Host == "github.com" {
		// GitHub.com — dedicated API host, Bearer token
		apiRoot = "https://api.github.com"
		tokenScheme = "Bearer"
	} else {
		// Gitea / Forgejo / self-hosted — API is at /api/v1 on the same host
		apiRoot = u.Scheme + "://" + u.Host + "/api/v1"
		tokenScheme = "token"
	}

	endpoint = apiRoot + "/repos/" + owner + "/" + repo + "/pulls"
	if authToken != "" {
		authHeader = tokenScheme + " " + authToken
	}
	return endpoint, authHeader, nil
}
