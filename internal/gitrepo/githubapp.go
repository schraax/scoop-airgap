package gitrepo

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ResolveGitHubAppToken generates a short-lived GitHub App installation access
// token. privateKeyPEM must be the RSA private key in PEM format (PKCS#1 or
// PKCS#8). The returned token is valid for up to one hour and can be used for
// both HTTPS git operations and the PR API.
func ResolveGitHubAppToken(appID, installationID int64, privateKeyPEM string) (string, error) {
	jwt, err := newAppJWT(appID, privateKeyPEM)
	if err != nil {
		return "", fmt.Errorf("generate GitHub App JWT: %w", err)
	}
	return exchangeInstallationToken(jwt, installationID)
}

// newAppJWT builds a signed RS256 JWT suitable for the GitHub Apps REST API.
func newAppJWT(appID int64, privateKeyPEM string) (string, error) {
	key, err := parseRSAKey(privateKeyPEM)
	if err != nil {
		return "", err
	}

	now := time.Now()
	header := b64url(mustJSON(map[string]string{"alg": "RS256", "typ": "JWT"}))
	payload := b64url(mustJSON(map[string]any{
		"iss": fmt.Sprintf("%d", appID),
		"iat": now.Add(-60 * time.Second).Unix(), // 60 s clock-skew buffer
		"exp": now.Add(9 * time.Minute).Unix(),   // GitHub max is 10 min
	}))

	msg := header + "." + payload
	digest := sha256.Sum256([]byte(msg))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign JWT: %w", err)
	}
	return msg + "." + b64url(sig), nil
}

// exchangeInstallationToken calls the GitHub Apps API to exchange a JWT for an
// installation access token.
func exchangeInstallationToken(jwt string, installationID int64) (string, error) {
	url := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("installation token request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("installation token API returned %s: %.300s", resp.Status, body)
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse installation token response: %w", err)
	}
	if result.Token == "" {
		return "", fmt.Errorf("GitHub API returned an empty token")
	}
	return result.Token, nil
}

func parseRSAKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in private key")
	}
	// Try PKCS#1 (the format GitHub generates for App keys)
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	// Fall back to PKCS#8
	keyI, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key (tried PKCS#1 and PKCS#8): %w", err)
	}
	key, ok := keyI.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not RSA (got %T)", keyI)
	}
	return key, nil
}

func b64url(data []byte) string { return base64.RawURLEncoding.EncodeToString(data) }

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
