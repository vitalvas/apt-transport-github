package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestControlCache(t *testing.T) {
	t.Run("put and get", func(t *testing.T) {
		c := New(t.TempDir())

		entry := &Entry{
			Fields: []Field{
				{Key: "Package", Value: "testpkg"},
				{Key: "Depends", Value: "libc6"},
			},
		}

		require.NoError(t, c.PutControl("owner", "repo", "v1.0.0", "testpkg_1.0.0_amd64.deb", entry))

		got, ok := c.GetControl("owner", "repo", "v1.0.0", "testpkg_1.0.0_amd64.deb")
		require.True(t, ok)
		assert.Equal(t, "testpkg", got.Fields[0].Value)
		assert.Equal(t, "libc6", got.Fields[1].Value)
	})

	t.Run("stores at expected path", func(t *testing.T) {
		base := t.TempDir()
		c := New(base)

		entry := &Entry{Fields: []Field{{Key: "Package", Value: "testpkg"}}}
		require.NoError(t, c.PutControl("owner", "repo", "v1.0.0", "testpkg_1.0.0_amd64.deb", entry))

		expected := filepath.Join(base, "owner", "repo", "v1.0.0", "testpkg_1.0.0_amd64.json")
		assert.FileExists(t, expected)
	})

	t.Run("escapes tag path segment", func(t *testing.T) {
		base := t.TempDir()
		c := New(base)

		entry := &Entry{Fields: []Field{{Key: "Package", Value: "testpkg"}}}
		require.NoError(t, c.PutControl("owner", "repo", "release/v1.0.0", "testpkg_1.0.0_amd64.deb", entry))

		expected := filepath.Join(base, "owner", "repo", "release%2Fv1.0.0", "testpkg_1.0.0_amd64.json")
		assert.FileExists(t, expected)

		got, ok := c.GetControl("owner", "repo", "release/v1.0.0", "testpkg_1.0.0_amd64.deb")
		require.True(t, ok)
		assert.Equal(t, "testpkg", got.Fields[0].Value)
	})

	t.Run("multiple packages per tag", func(t *testing.T) {
		c := New(t.TempDir())

		minion := &Entry{Fields: []Field{{Key: "Package", Value: "salt-minion"}}}
		master := &Entry{Fields: []Field{{Key: "Package", Value: "salt-master"}}}

		require.NoError(t, c.PutControl("owner", "repo", "v1.0.0", "salt-minion_1.0.0_amd64.deb", minion))
		require.NoError(t, c.PutControl("owner", "repo", "v1.0.0", "salt-master_1.0.0_amd64.deb", master))

		gotMinion, ok := c.GetControl("owner", "repo", "v1.0.0", "salt-minion_1.0.0_amd64.deb")
		require.True(t, ok)
		assert.Equal(t, "salt-minion", gotMinion.Fields[0].Value)

		gotMaster, ok := c.GetControl("owner", "repo", "v1.0.0", "salt-master_1.0.0_amd64.deb")
		require.True(t, ok)
		assert.Equal(t, "salt-master", gotMaster.Fields[0].Value)
	})

	t.Run("miss on unknown filename", func(t *testing.T) {
		c := New(t.TempDir())

		_, ok := c.GetControl("owner", "repo", "v9.9.9", "missing_1.0.0_amd64.deb")
		assert.False(t, ok)
	})

	t.Run("miss on corrupt json", func(t *testing.T) {
		base := t.TempDir()
		c := New(base)

		dir := filepath.Join(base, "owner", "repo", "v1.0.0")
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "pkg_1.0.0_amd64.json"), []byte("{"), 0644))

		_, ok := c.GetControl("owner", "repo", "v1.0.0", "pkg_1.0.0_amd64.deb")
		assert.False(t, ok)
	})

	t.Run("put mkdir error", func(t *testing.T) {
		base := filepath.Join(t.TempDir(), "cache-file")
		require.NoError(t, os.WriteFile(base, []byte("not a dir"), 0644))
		c := New(base)

		err := c.PutControl("owner", "repo", "v1.0.0", "pkg_1.0.0_amd64.deb", &Entry{})
		assert.Error(t, err)
	})

	t.Run("overwrite existing", func(t *testing.T) {
		c := New(t.TempDir())

		entry1 := &Entry{Fields: []Field{{Key: "Version", Value: "1.0"}}}
		entry2 := &Entry{Fields: []Field{{Key: "Version", Value: "2.0"}}}

		require.NoError(t, c.PutControl("owner", "repo", "v1.0.0", "pkg_1.0.0_amd64.deb", entry1))
		require.NoError(t, c.PutControl("owner", "repo", "v1.0.0", "pkg_1.0.0_amd64.deb", entry2))

		got, ok := c.GetControl("owner", "repo", "v1.0.0", "pkg_1.0.0_amd64.deb")
		require.True(t, ok)
		assert.Equal(t, "2.0", got.Fields[0].Value)
	})
}

func TestControlFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "standard deb", input: "testpkg_1.0.0_amd64.deb", expected: "testpkg_1.0.0_amd64.json"},
		{name: "no deb extension", input: "testpkg_1.0.0_amd64", expected: "testpkg_1.0.0_amd64.json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, controlFilename(tt.input))
		})
	}
}

func TestReleasesCache(t *testing.T) {
	t.Run("put and get", func(t *testing.T) {
		c := New(t.TempDir())

		data := json.RawMessage(`[{"tag_name":"v1.0.0"}]`)

		require.NoError(t, c.PutReleases("owner", "repo", 1, data))

		got, ok := c.GetReleases("owner", "repo", 1)
		require.True(t, ok)
		assert.JSONEq(t, `[{"tag_name":"v1.0.0"}]`, string(got))
	})

	t.Run("stores at expected path", func(t *testing.T) {
		base := t.TempDir()
		c := New(base)

		require.NoError(t, c.PutReleases("owner", "repo", 1, json.RawMessage(`[]`)))

		expected := filepath.Join(base, "owner", "repo", "releases_1.json")
		assert.FileExists(t, expected)
	})

	t.Run("miss on unknown repo", func(t *testing.T) {
		c := New(t.TempDir())

		_, ok := c.GetReleases("owner", "missing", 1)
		assert.False(t, ok)
	})

	t.Run("miss on corrupt json", func(t *testing.T) {
		base := t.TempDir()
		c := New(base)

		dir := filepath.Join(base, "owner", "repo")
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "releases_1.json"), []byte("{"), 0644))

		_, ok := c.GetReleases("owner", "repo", 1)
		assert.False(t, ok)
	})

	t.Run("put mkdir error", func(t *testing.T) {
		base := filepath.Join(t.TempDir(), "cache-file")
		require.NoError(t, os.WriteFile(base, []byte("not a dir"), 0644))
		c := New(base)

		err := c.PutReleases("owner", "repo", 1, json.RawMessage(`[]`))
		assert.Error(t, err)
	})

	t.Run("separate entries by limit", func(t *testing.T) {
		c := New(t.TempDir())

		require.NoError(t, c.PutReleases("owner", "repo", 1, json.RawMessage(`[{"tag_name":"v1.0.0"}]`)))
		require.NoError(t, c.PutReleases("owner", "repo", 20, json.RawMessage(`[{"tag_name":"v1.0.0"},{"tag_name":"v0.9.0"}]`)))

		gotDefault, ok := c.GetReleases("owner", "repo", 1)
		require.True(t, ok)
		assert.JSONEq(t, `[{"tag_name":"v1.0.0"}]`, string(gotDefault))

		gotHistory, ok := c.GetReleases("owner", "repo", 20)
		require.True(t, ok)
		assert.JSONEq(t, `[{"tag_name":"v1.0.0"},{"tag_name":"v0.9.0"}]`, string(gotHistory))
	})

	t.Run("expired entry", func(t *testing.T) {
		base := t.TempDir()
		c := New(base)

		entry := ReleasesEntry{
			Data:      json.RawMessage(`[]`),
			FetchedAt: time.Now().Add(-10 * time.Minute),
		}

		dir := filepath.Join(base, "owner", "repo")
		require.NoError(t, os.MkdirAll(dir, 0755))

		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "releases_1.json"), raw, 0644))

		_, ok := c.GetReleases("owner", "repo", 1)
		assert.False(t, ok)
	})
}

func TestPackageCache(t *testing.T) {
	t.Run("put and get", func(t *testing.T) {
		c := New(t.TempDir())

		debData := []byte("fake deb content")

		path, err := c.PutPackage("owner", "repo", "v1.0.0", "test_1.0.0_amd64.deb", debData)
		require.NoError(t, err)
		assert.Contains(t, path, "owner/repo/v1.0.0/test_1.0.0_amd64.deb")

		cachedPath, ok := c.GetPackage("owner", "repo", "v1.0.0", "test_1.0.0_amd64.deb")
		require.True(t, ok)
		assert.Equal(t, path, cachedPath)

		got, err := os.ReadFile(cachedPath)
		require.NoError(t, err)
		assert.Equal(t, debData, got)
	})

	t.Run("miss on unknown path", func(t *testing.T) {
		c := New(t.TempDir())

		_, ok := c.GetPackage("owner", "repo", "v1.0.0", "missing.deb")
		assert.False(t, ok)
	})

	t.Run("put mkdir error", func(t *testing.T) {
		base := filepath.Join(t.TempDir(), "cache-file")
		require.NoError(t, os.WriteFile(base, []byte("not a dir"), 0644))
		c := New(base)

		_, err := c.PutPackage("owner", "repo", "v1.0.0", "pkg.deb", []byte("data"))
		assert.Error(t, err)
	})
}

func TestCleanStaleTags(t *testing.T) {
	t.Run("removes stale keeps valid", func(t *testing.T) {
		c := New(t.TempDir())

		_, err := c.PutPackage("owner", "repo", "v1.0.0", "pkg_1.0.0_amd64.deb", []byte("v1"))
		require.NoError(t, err)

		_, err = c.PutPackage("owner", "repo", "v0.9.0", "pkg_0.9.0_amd64.deb", []byte("v09"))
		require.NoError(t, err)

		_, err = c.PutPackage("owner", "repo", "v0.8.0", "pkg_0.8.0_amd64.deb", []byte("v08"))
		require.NoError(t, err)

		validTags := map[string]bool{
			"v1.0.0": true,
			"v0.9.0": true,
		}

		require.NoError(t, c.CleanStaleTags("owner", "repo", validTags))

		_, ok := c.GetPackage("owner", "repo", "v1.0.0", "pkg_1.0.0_amd64.deb")
		assert.True(t, ok)

		_, ok = c.GetPackage("owner", "repo", "v0.9.0", "pkg_0.9.0_amd64.deb")
		assert.True(t, ok)

		_, ok = c.GetPackage("owner", "repo", "v0.8.0", "pkg_0.8.0_amd64.deb")
		assert.False(t, ok)
	})

	t.Run("keeps escaped tag dirs", func(t *testing.T) {
		c := New(t.TempDir())

		_, err := c.PutPackage("owner", "repo", "release/v1.0.0", "pkg_1.0.0_amd64.deb", []byte("v1"))
		require.NoError(t, err)

		_, err = c.PutPackage("owner", "repo", "release/v0.9.0", "pkg_0.9.0_amd64.deb", []byte("v09"))
		require.NoError(t, err)

		validTags := map[string]bool{
			"release/v1.0.0": true,
		}

		require.NoError(t, c.CleanStaleTags("owner", "repo", validTags))

		_, ok := c.GetPackage("owner", "repo", "release/v1.0.0", "pkg_1.0.0_amd64.deb")
		assert.True(t, ok)

		_, ok = c.GetPackage("owner", "repo", "release/v0.9.0", "pkg_0.9.0_amd64.deb")
		assert.False(t, ok)
	})

	t.Run("no dir exists", func(t *testing.T) {
		c := New(t.TempDir())

		err := c.CleanStaleTags("owner", "nonexistent", map[string]bool{})
		assert.NoError(t, err)
	})

	t.Run("repo path is file", func(t *testing.T) {
		base := t.TempDir()
		c := New(base)

		dir := filepath.Join(base, "owner")
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "repo"), []byte("not a dir"), 0644))

		err := c.CleanStaleTags("owner", "repo", map[string]bool{})
		assert.Error(t, err)
	})
}

func TestClean(t *testing.T) {
	c := New(t.TempDir())

	require.NoError(t, c.PutControl("owner", "repo", "v1.0.0", "test_1.0.0_amd64.deb", &Entry{
		Fields: []Field{{Key: "Package", Value: "test"}},
	}))

	require.NoError(t, c.PutReleases("owner", "repo", 1, json.RawMessage(`[]`)))

	require.NoError(t, c.Clean())

	_, ok := c.GetControl("owner", "repo", "v1.0.0", "test_1.0.0_amd64.deb")
	assert.False(t, ok)

	_, ok = c.GetReleases("owner", "repo", 1)
	assert.False(t, ok)
}
