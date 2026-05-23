package addrepo

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vitalvas/apt-transport-github/internal/signing"
)

func TestRun(t *testing.T) {
	t.Run("requires root", func(t *testing.T) {
		var buf bytes.Buffer

		err := Run(&buf, 1000, "owner", "repo", t.TempDir())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "root")
	})

	t.Run("creates sources file", func(t *testing.T) {
		tmpDir := t.TempDir()
		var buf bytes.Buffer

		err := Run(&buf, 0, "vitalvas", "myrepo", tmpDir)
		require.NoError(t, err)

		filename := filepath.Join(tmpDir, "myrepo.sources")
		assert.FileExists(t, filename)

		content, err := os.ReadFile(filename)
		require.NoError(t, err)

		assert.Contains(t, string(content), "Types: deb")
		assert.Contains(t, string(content), "URIs: github://vitalvas/myrepo")
		assert.Contains(t, string(content), "Suites: stable")
		assert.Contains(t, string(content), "Components: main")
		assert.Contains(t, string(content), fmt.Sprintf("Signed-By: %s", signing.DefaultPubKey))

		assert.Contains(t, buf.String(), "Repository added:")
	})

	t.Run("errors if file exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		existing := filepath.Join(tmpDir, "myrepo.sources")
		require.NoError(t, os.WriteFile(existing, []byte("existing"), 0644))

		var buf bytes.Buffer

		err := Run(&buf, 0, "vitalvas", "myrepo", tmpDir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("rejects invalid owner", func(t *testing.T) {
		var buf bytes.Buffer

		err := Run(&buf, 0, "../owner", "myrepo", t.TempDir())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid GitHub owner")
	})

	t.Run("rejects invalid repo", func(t *testing.T) {
		var buf bytes.Buffer

		err := Run(&buf, 0, "vitalvas", "../myrepo", t.TempDir())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid GitHub repo")
	})

	t.Run("write error", func(t *testing.T) {
		tmpDir := t.TempDir()
		blockingPath := filepath.Join(tmpDir, "sources")
		require.NoError(t, os.WriteFile(blockingPath, []byte("not a dir"), 0644))

		var buf bytes.Buffer

		err := Run(&buf, 0, "vitalvas", "myrepo", blockingPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to write sources file")
	})
}
