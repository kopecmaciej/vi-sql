package util

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const githubLatestRelease = "https://api.github.com/repos/kopecmaciej/vi-sql/releases/latest"

// FetchLatestVersion returns the latest release tag from GitHub (e.g. "v1.2.3").
func FetchLatestVersion(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubLatestRelease, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return ""
	}

	return strings.TrimSpace(payload.TagName)
}

// NormalizeVersion strips the "v" prefix and any pre-release/build metadata
// (everything after the first "-"), returning a bare "MAJOR.MINOR.PATCH" string.
func NormalizeVersion(tag string) string {
	tag = strings.TrimPrefix(tag, "v")
	return strings.SplitN(tag, "-", 2)[0]
}

// IsNewerVersion returns true when latestTag is strictly newer than currentTag.
// Both tags are expected in "vMAJOR.MINOR.PATCH" form; any parse failure returns false.
func IsNewerVersion(currentTag, latestTag string) bool {
	cv := parseVersion(currentTag)
	lv := parseVersion(latestTag)
	if cv == [3]int{} || lv == [3]int{} {
		return false
	}
	return lv[0] > cv[0] ||
		(lv[0] == cv[0] && lv[1] > cv[1]) ||
		(lv[0] == cv[0] && lv[1] == cv[1] && lv[2] > cv[2])
}

func parseVersion(tag string) [3]int {
	tag = strings.TrimPrefix(tag, "v")
	parts := strings.SplitN(tag, ".", 3)
	if len(parts) != 3 {
		return [3]int{}
	}
	var v [3]int
	for i, p := range parts {
		p = strings.SplitN(p, "-", 2)[0] // strip pre-release suffix
		n := 0
		for _, c := range p {
			if c < '0' || c > '9' {
				return [3]int{}
			}
			n = n*10 + int(c-'0')
		}
		v[i] = n
	}
	return v
}
