// Package manifest handles fetching, parsing, and rewriting Scoop bucket manifests.
// Only the url fields (top-level and per-architecture) are rewritten; all other
// fields — including hashes — are preserved verbatim, since the mirrored files are
// byte-identical to the originals.
package manifest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

// ArtifactRef describes one file that must be mirrored.
type ArtifactRef struct {
	// DownloadURL is the original URL without any Scoop fragment.
	DownloadURL string
	// Fragment is the Scoop rename suffix, e.g. "#/git-setup.exe". May be empty.
	Fragment string
	// Hash is the expected hash string as it appears in the manifest (e.g. "sha256:abc…").
	Hash string
	// ArtifactPath is the relative path inside the Artifactory repo:
	//   {bucket}/{app}/{version}/{filename}
	ArtifactPath string
	// ArtifactURL is the full URL clients will use to fetch this file.
	ArtifactURL string
}

// Fetch retrieves a manifest JSON from a public bucket raw URL and returns the
// parsed document plus the version string.
func Fetch(bucketBaseURL, app string) (map[string]interface{}, string, error) {
	rawURL := strings.TrimRight(bucketBaseURL, "/") + "/" + app + ".json"
	resp, err := http.Get(rawURL) //nolint:noctx
	if err != nil {
		return nil, "", fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, "", fmt.Errorf("manifest not found: %s", rawURL)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("fetch %s: HTTP %d", rawURL, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read %s: %w", rawURL, err)
	}

	var doc map[string]interface{}
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil {
		return nil, "", fmt.Errorf("parse %s: %w", rawURL, err)
	}

	version, _ := doc["version"].(string)
	if version == "" {
		return nil, "", fmt.Errorf("manifest %s has no version field", rawURL)
	}

	return doc, version, nil
}

// Process rewrites all download URLs in doc to point at Artifactory and returns
// the list of artifacts that need to be mirrored.
// artifBaseURL is the full base URL of the Artifactory repo,
// e.g. https://artifactory.example.com/artifactory/scoop-mirror
func Process(doc map[string]interface{}, bucket, app, version, artifBaseURL string) ([]ArtifactRef, error) {
	base := strings.TrimRight(artifBaseURL, "/")
	var refs []ArtifactRef

	// Top-level url + hash
	if urls, hashes, ok := extractURLsHashes(doc, "url", "hash"); ok {
		newURLs := make([]string, len(urls))
		for i, u := range urls {
			ref, err := makeRef(u, hashes[i], bucket, app, version, base)
			if err != nil {
				return nil, err
			}
			refs = append(refs, ref)
			newURLs[i] = ref.ArtifactURL
		}
		setURLField(doc, "url", newURLs)
	}

	// Per-architecture urls
	if archRaw, ok := doc["architecture"]; ok {
		archMap, ok := archRaw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected type for 'architecture' field")
		}
		for _, arch := range []string{"64bit", "32bit", "arm64"} {
			archEntryRaw, ok := archMap[arch]
			if !ok {
				continue
			}
			archEntry, ok := archEntryRaw.(map[string]interface{})
			if !ok {
				continue
			}
			if urls, hashes, ok := extractURLsHashes(archEntry, "url", "hash"); ok {
				newURLs := make([]string, len(urls))
				for i, u := range urls {
					ref, err := makeRef(u, hashes[i], bucket, app, version, base)
					if err != nil {
						return nil, err
					}
					refs = append(refs, ref)
					newURLs[i] = ref.ArtifactURL
				}
				setURLField(archEntry, "url", newURLs)
			}
		}
	}

	return refs, nil
}

// Marshal serialises the (potentially rewritten) manifest back to indented JSON.
func Marshal(doc map[string]interface{}) ([]byte, error) {
	return json.MarshalIndent(doc, "", "    ")
}

// CommitAge returns how long ago the manifest file was last committed in the
// upstream bucket repository. This is used to implement the cooldown period:
// versions that were published very recently are skipped until they stabilise.
//
// Only GitHub-hosted buckets are supported (raw.githubusercontent.com URLs).
// For any other host, (0, nil) is returned and the caller should treat it as
// "cooldown check not available, proceed normally".
//
// The GitHub REST API allows 60 unauthenticated requests per hour. Set the
// GITHUB_TOKEN environment variable to raise this limit to 5 000/hour.
func CommitAge(bucketURL, app string) (time.Duration, error) {
	owner, repo, branch, ok := parseGitHubRawURL(bucketURL)
	if !ok {
		return 0, nil
	}

	apiURL := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/commits?path=bucket/%s.json&sha=%s&per_page=1",
		owner, repo, app, branch,
	)

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if tok := githubToken(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("GitHub API %s: %w", apiURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("GitHub API %s: HTTP %d: %s", apiURL, resp.StatusCode, body)
	}

	var commits []struct {
		Commit struct {
			Committer struct {
				Date time.Time `json:"date"`
			} `json:"committer"`
		} `json:"commit"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&commits); err != nil {
		return 0, fmt.Errorf("decode GitHub API response: %w", err)
	}
	if len(commits) == 0 {
		return 0, fmt.Errorf("no commits found for bucket/%s.json in %s/%s", app, owner, repo)
	}

	return time.Since(commits[0].Commit.Committer.Date), nil
}

// parseGitHubRawURL extracts owner, repo, and branch from a raw.githubusercontent.com URL.
// Returns ok=false for any other host.
func parseGitHubRawURL(rawBucketURL string) (owner, repo, branch string, ok bool) {
	u, err := url.Parse(rawBucketURL)
	if err != nil || u.Host != "raw.githubusercontent.com" {
		return "", "", "", false
	}
	// Path: /{owner}/{repo}/{branch}/bucket
	parts := strings.SplitN(strings.TrimPrefix(u.Path, "/"), "/", 5)
	if len(parts) < 4 || parts[3] != "bucket" {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

func githubToken() string {
	return os.Getenv("GITHUB_TOKEN")
}

// extractURLsHashes reads parallel url+hash fields from a map node.
// Both fields may be a single string or an array of strings.
// Returns (urls, hashes, found).
func extractURLsHashes(node map[string]interface{}, urlKey, hashKey string) ([]string, []string, bool) {
	urlRaw, ok := node[urlKey]
	if !ok {
		return nil, nil, false
	}

	urls := toStringSlice(urlRaw)
	if len(urls) == 0 {
		return nil, nil, false
	}

	var hashes []string
	if hashRaw, ok := node[hashKey]; ok {
		hashes = toStringSlice(hashRaw)
	}
	// Pad hashes with empty strings if fewer hashes than URLs.
	for len(hashes) < len(urls) {
		hashes = append(hashes, "")
	}

	return urls, hashes, true
}

func toStringSlice(v interface{}) []string {
	switch val := v.(type) {
	case string:
		return []string{val}
	case []interface{}:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// setURLField writes back a url field; uses a plain string if len==1.
func setURLField(node map[string]interface{}, key string, values []string) {
	if len(values) == 1 {
		node[key] = values[0]
	} else {
		iface := make([]interface{}, len(values))
		for i, v := range values {
			iface[i] = v
		}
		node[key] = iface
	}
}

// splitScoopURL separates the HTTP URL from the Scoop rename fragment (#/name).
func splitScoopURL(raw string) (downloadURL, fragment string) {
	idx := strings.Index(raw, "#")
	if idx == -1 {
		return raw, ""
	}
	return raw[:idx], raw[idx:]
}

func filenameFromURL(rawURL string) string {
	downloadURL, _ := splitScoopURL(rawURL)
	u, err := url.Parse(downloadURL)
	if err != nil {
		return path.Base(downloadURL)
	}
	name := path.Base(u.Path)
	if name == "." || name == "/" {
		name = "download"
	}
	return name
}

func makeRef(rawURL, hash, bucket, app, version, artifBase string) (ArtifactRef, error) {
	downloadURL, fragment := splitScoopURL(rawURL)
	if downloadURL == "" {
		return ArtifactRef{}, fmt.Errorf("empty URL in manifest")
	}

	filename := filenameFromURL(rawURL)
	// sanitise version for use as a path component
	safeVersion := strings.ReplaceAll(version, "/", "_")
	artifPath := path.Join(bucket, app, safeVersion, filename)
	artifURL := artifBase + "/" + artifPath + fragment

	return ArtifactRef{
		DownloadURL:  downloadURL,
		Fragment:     fragment,
		Hash:         hash,
		ArtifactPath: artifPath,
		ArtifactURL:  artifURL,
	}, nil
}
