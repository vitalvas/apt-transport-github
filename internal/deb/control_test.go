package deb

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildControlTarGz(t *testing.T, controlContent string) []byte {
	t.Helper()

	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)

	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: "./control",
		Size: int64(len(controlContent)),
		Mode: 0644,
	}))

	_, err := tw.Write([]byte(controlContent))
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	var gzBuf bytes.Buffer
	gz := gzip.NewWriter(&gzBuf)
	_, err = gz.Write(tarBuf.Bytes())
	require.NoError(t, err)
	require.NoError(t, gz.Close())

	return gzBuf.Bytes()
}

func buildAr(t *testing.T, members map[string][]byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	buf.WriteString("!<arch>\n")

	for name, data := range members {
		header := fmt.Sprintf("%-16s%-12s%-6s%-6s%-8s%-10d`\n",
			name, "0", "0", "0", "100644", len(data))
		buf.WriteString(header)
		buf.Write(data)

		if len(data)%2 != 0 {
			buf.WriteByte('\n')
		}
	}

	return buf.Bytes()
}

func buildTestDeb(t *testing.T, controlContent string) []byte {
	t.Helper()

	controlTarGz := buildControlTarGz(t, controlContent)

	debBinary := []byte("2.0\n")

	var buf bytes.Buffer
	buf.WriteString("!<arch>\n")

	writeArEntry := func(name string, data []byte) {
		header := fmt.Sprintf("%-16s%-12s%-6s%-6s%-8s%-10d`\n",
			name, "0", "0", "0", "100644", len(data))
		buf.WriteString(header)
		buf.Write(data)

		if len(data)%2 != 0 {
			buf.WriteByte('\n')
		}
	}

	writeArEntry("debian-binary", debBinary)
	writeArEntry("control.tar.gz", controlTarGz)
	writeArEntry("data.tar.gz", []byte{})

	return buf.Bytes()
}

func TestParseControl(t *testing.T) {
	controlContent := `Package: myapp
Version: 1.0.0
Architecture: amd64
Maintainer: Test User <test@example.com>
Depends: libc6 (>= 2.17), gnupg
Description: My test application
 This is a longer description
 that spans multiple lines.
`

	debData := buildTestDeb(t, controlContent)

	ctrl, err := ParseControl(debData)
	require.NoError(t, err)

	assert.Equal(t, "myapp", ctrl.Get("Package"))
	assert.Equal(t, "1.0.0", ctrl.Get("Version"))
	assert.Equal(t, "amd64", ctrl.Get("Architecture"))
	assert.Equal(t, "Test User <test@example.com>", ctrl.Get("Maintainer"))
	assert.Equal(t, "libc6 (>= 2.17), gnupg", ctrl.Get("Depends"))
	assert.Contains(t, ctrl.Get("Description"), "My test application")
	assert.Contains(t, ctrl.Get("Description"), "multiple lines")
}

func TestParseControlNoDeps(t *testing.T) {
	controlContent := `Package: simple
Version: 2.0.0
Architecture: arm64
Description: Simple package
`

	debData := buildTestDeb(t, controlContent)

	ctrl, err := ParseControl(debData)
	require.NoError(t, err)

	assert.Equal(t, "simple", ctrl.Get("Package"))
	assert.Equal(t, "", ctrl.Get("Depends"))
}

func TestParseControlInvalidAr(t *testing.T) {
	_, err := ParseControl([]byte("not an ar archive"))
	assert.Error(t, err)
}

func TestParseControlNoControlTar(t *testing.T) {
	data := buildAr(t, map[string][]byte{
		"debian-binary": []byte("2.0\n"),
		"data.tar.gz":   {},
	})

	_, err := ParseControl(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "control archive not found")
}

func TestExtractControlFileErrors(t *testing.T) {
	t.Run("invalid gzip", func(t *testing.T) {
		_, err := extractControlFile([]byte("not gzip"))
		assert.Error(t, err)
	})

	t.Run("no control file", func(t *testing.T) {
		data := buildControlTarGz(t, "")
		var tarBuf bytes.Buffer
		tw := tar.NewWriter(&tarBuf)
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name: "./postinst",
			Size: int64(len("script")),
			Mode: 0755,
		}))
		_, err := tw.Write([]byte("script"))
		require.NoError(t, err)
		require.NoError(t, tw.Close())

		var gzBuf bytes.Buffer
		gz := gzip.NewWriter(&gzBuf)
		_, err = gz.Write(tarBuf.Bytes())
		require.NoError(t, err)
		require.NoError(t, gz.Close())

		_, err = extractControlFile(gzBuf.Bytes())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "control file not found")

		_ = data
	})
}

func TestControlGet(t *testing.T) {
	ctrl := &Control{
		Fields: []Field{
			{Key: "Package", Value: "test"},
			{Key: "Version", Value: "1.0"},
		},
	}

	assert.Equal(t, "test", ctrl.Get("Package"))
	assert.Equal(t, "1.0", ctrl.Get("Version"))
	assert.Equal(t, "", ctrl.Get("Missing"))
}

func TestParseControlFields(t *testing.T) {
	data := []byte(`Package: myapp
Version: 1.0.0
Depends: libc6, gnupg
Description: Short desc
 Long description line 1
 Long description line 2
`)

	ctrl, err := parseControlFields(data)
	require.NoError(t, err)

	assert.Equal(t, "myapp", ctrl.Get("Package"))
	assert.Equal(t, "1.0.0", ctrl.Get("Version"))
	assert.Equal(t, "libc6, gnupg", ctrl.Get("Depends"))
	assert.Contains(t, ctrl.Get("Description"), "Short desc\n Long description line 1\n Long description line 2")
}

func TestParseControlFieldsScannerError(t *testing.T) {
	data := []byte(fmt.Sprintf("Package: %s", strings.Repeat("a", 70*1024)))

	_, err := parseControlFields(data)
	assert.Error(t, err)
}

func TestExtractControlTarShortHeader(t *testing.T) {
	data := []byte("!<arch>\nshort")

	_, err := extractControlTar(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read ar header")
}

func TestExtractControlTarInvalidSignature(t *testing.T) {
	// Too short for ar signature
	_, err := extractControlTar([]byte("short"))
	assert.Error(t, err)
}

func TestExtractControlTarBadSize(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("!<arch>\n")

	header := fmt.Sprintf("%-16s%-12s%-6s%-6s%-8s%-10s`\n",
		"control.tar.gz", "0", "0", "0", "100644", "badsize")
	buf.WriteString(header)

	_, err := extractControlTar(buf.Bytes())
	assert.Error(t, err)
}

// Suppress unused import warning for binary package
var _ = binary.LittleEndian
