package cache

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultBaseDir = "/var/cache/apt-transport-github"
	releasesTTL    = 5 * time.Minute
)

type Entry struct {
	Fields []Field `json:"fields"`
	SHA256 string  `json:"sha256,omitempty"`
}

type ReleasesEntry struct {
	Data      json.RawMessage `json:"data"`
	FetchedAt time.Time       `json:"fetched_at"`
}

type Field struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type DiskCache struct {
	baseDir string
}

func New(baseDir string) *DiskCache {
	return &DiskCache{baseDir: baseDir}
}

func controlFilename(debFilename string) string {
	return fmt.Sprintf("%s.json", strings.TrimSuffix(debFilename, ".deb"))
}

func (c *DiskCache) GetControl(owner, repo, tag, filename string) (*Entry, bool) {
	path := filepath.Join(c.baseDir, owner, repo, url.PathEscape(tag), controlFilename(filename))

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}

	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false
	}

	return &entry, true
}

func (c *DiskCache) PutControl(owner, repo, tag, filename string, entry *Entry) error {
	dir := filepath.Join(c.baseDir, owner, repo, url.PathEscape(tag))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, controlFilename(filename)), data, 0644)
}

func (c *DiskCache) GetReleases(owner, repo string, limit int) (json.RawMessage, bool) {
	path := filepath.Join(c.baseDir, owner, repo, fmt.Sprintf("releases_%d.json", limit))

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}

	var entry ReleasesEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false
	}

	if time.Since(entry.FetchedAt) > releasesTTL {
		return nil, false
	}

	return entry.Data, true
}

func (c *DiskCache) PutReleases(owner, repo string, limit int, data json.RawMessage) error {
	dir := filepath.Join(c.baseDir, owner, repo)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	entry := ReleasesEntry{
		Data:      data,
		FetchedAt: time.Now(),
	}

	raw, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, fmt.Sprintf("releases_%d.json", limit)), raw, 0644)
}

func (c *DiskCache) GetPackage(owner, repo, tag, filename string) (string, bool) {
	path := filepath.Join(c.baseDir, owner, repo, url.PathEscape(tag), filename)

	if _, err := os.Stat(path); err != nil {
		return "", false
	}

	return path, true
}

func (c *DiskCache) PutPackage(owner, repo, tag, filename string, data []byte) (string, error) {
	dir := filepath.Join(c.baseDir, owner, repo, url.PathEscape(tag))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	path := filepath.Join(dir, filename)

	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}

	return path, nil
}

func (c *DiskCache) CleanStaleTags(owner, repo string, validTags map[string]bool) error {
	repoDir := filepath.Join(c.baseDir, owner, repo)

	entries, err := os.ReadDir(repoDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	validTagDirs := make(map[string]bool, len(validTags))
	for tag := range validTags {
		validTagDirs[url.PathEscape(tag)] = true
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if !validTagDirs[entry.Name()] {
			if err := os.RemoveAll(filepath.Join(repoDir, entry.Name())); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *DiskCache) Clean() error {
	return os.RemoveAll(c.baseDir)
}
