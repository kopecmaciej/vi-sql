package util

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	githubLatestRelease = "https://api.github.com/repos/kopecmaciej/vi-sql/releases/latest"
	githubDownloadBase  = "https://github.com/kopecmaciej/vi-sql/releases/download"
	checksumAsset       = "vi-sql_checksum_sha256.txt"
)

func FetchLatestVersionFromGithub(ctx context.Context) string {
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

func TagToVersion(tag string) string {
	tag = strings.TrimPrefix(tag, "v")
	return strings.SplitN(tag, "-", 2)[0]
}

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
		p = strings.SplitN(p, "-", 2)[0]
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

func InstallRelease(ctx context.Context, tag string, progress func(string)) error {
	asset := buildAssetName(runtime.GOOS, runtime.GOARCH)
	base := fmt.Sprintf("%s/%s", githubDownloadBase, tag)

	progress("Downloading checksum file…")
	checksumData, err := downloadBytes(ctx, base+"/"+checksumAsset)
	if err != nil {
		return fmt.Errorf("download checksum: %w", err)
	}

	progress(fmt.Sprintf("Downloading %s…", asset))
	archiveData, err := downloadBytes(ctx, base+"/"+asset)
	if err != nil {
		return fmt.Errorf("download archive: %w", err)
	}

	progress("Verifying checksum…")
	if err := verifyShaChecksum(archiveData, asset, string(checksumData)); err != nil {
		return fmt.Errorf("checksum: %w", err)
	}

	progress("Extracting binary…")
	binary, err := extractBinary(archiveData, asset)
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}

	progress("Replacing binary…")
	if err := applyBinary(binary); err != nil {
		return fmt.Errorf("apply: %w", err)
	}

	return nil
}

func buildAssetName(goos, goarch string) string {
	os := titleOS(goos)
	arch := goarch
	switch arch {
	case "amd64":
		arch = "x86_64"
	case "386":
		arch = "i386"
	}
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("vi-sql_%s_%s%s", os, arch, ext)
}

func titleOS(goos string) string {
	switch goos {
	case "linux":
		return "Linux"
	case "darwin":
		return "Darwin"
	case "windows":
		return "Windows"
	default:
		if len(goos) == 0 {
			return goos
		}
		return strings.ToUpper(goos[:1]) + goos[1:]
	}
}

func downloadBytes(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

func verifyShaChecksum(archive []byte, assetName, checksumContent string) error {
	scanner := bufio.NewScanner(strings.NewReader(checksumContent))
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) != 2 || parts[1] != assetName {
			continue
		}
		expected, err := hex.DecodeString(parts[0])
		if err != nil {
			return fmt.Errorf("invalid checksum hex: %w", err)
		}
		sum := sha256.Sum256(archive)
		if !bytes.Equal(sum[:], expected) {
			return fmt.Errorf("checksum mismatch")
		}
		return nil
	}
	return fmt.Errorf("no checksum entry for %s", assetName)
}

func extractBinary(archiveData []byte, asset string) (io.Reader, error) {
	if strings.HasSuffix(asset, ".zip") {
		return extractFromZip(archiveData)
	}
	return extractFromTarGz(archiveData)
}

func extractFromTarGz(data []byte) (io.Reader, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Name == "vi-sql" || strings.HasSuffix(hdr.Name, "/vi-sql") {
			content, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			return bytes.NewReader(content), nil
		}
	}
	return nil, fmt.Errorf("vi-sql not found in archive")
}

func extractFromZip(data []byte) (io.Reader, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, f := range r.File {
		if f.Name == "vi-sql.exe" || strings.HasSuffix(f.Name, "/vi-sql.exe") {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer func() { _ = rc.Close() }()
			content, err := io.ReadAll(rc)
			if err != nil {
				return nil, err
			}
			return bytes.NewReader(content), nil
		}
	}
	return nil, fmt.Errorf("vi-sql.exe not found in archive")
}

func applyBinary(r io.Reader) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	info, err := os.Stat(exe)
	if err != nil {
		return fmt.Errorf("stat executable: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(exe), ".vi-sql-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := io.Copy(tmp, r); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write update: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmp.Name(), info.Mode()); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}
	if err := os.Rename(tmp.Name(), exe); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}
	return nil
}
