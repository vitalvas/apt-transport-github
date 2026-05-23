package method

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingWriter struct{}

func (w *failingWriter) Write(_ []byte) (int, error) {
	return 0, fmt.Errorf("write failed")
}

func TestMessageWrite(t *testing.T) {
	msg := &Message{Code: 100, Text: "Capabilities"}
	msg.Set("Version", "1.2")
	msg.Set("Single-Instance", "true")

	var buf bytes.Buffer
	err := msg.Write(&buf)

	require.NoError(t, err)

	expected := "100 Capabilities\nVersion: 1.2\nSingle-Instance: true\n\n"
	assert.Equal(t, expected, buf.String())
}

func TestMessageWriteError(t *testing.T) {
	msg := &Message{Code: 100, Text: "Capabilities"}

	err := msg.Write(&failingWriter{})
	assert.Error(t, err)
}

func TestReadMessage(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedMsg *Message
		expectErr   bool
	}{
		{
			name:  "acquire message",
			input: "600 URI Acquire\nURI: github://owner/repo/dists/stable/Release\nFilename: /tmp/release\n\n",
			expectedMsg: &Message{
				Code: 600,
				Text: "URI Acquire",
				Headers: []Header{
					{Key: "URI", Value: "github://owner/repo/dists/stable/Release"},
					{Key: "Filename", Value: "/tmp/release"},
				},
			},
		},
		{
			name:  "configuration message",
			input: "601 Configuration\nConfig-Item: Acquire::http::Proxy=http://proxy\n\n",
			expectedMsg: &Message{
				Code: 601,
				Text: "Configuration",
				Headers: []Header{
					{Key: "Config-Item", Value: "Acquire::http::Proxy=http://proxy"},
				},
			},
		},
		{
			name:  "no headers",
			input: "601 Configuration\n\n",
			expectedMsg: &Message{
				Code: 601,
				Text: "Configuration",
			},
		},
		{
			name:      "invalid code",
			input:     "abc Invalid\n\n",
			expectErr: true,
		},
		{
			name:      "no text",
			input:     "600\n\n",
			expectErr: true,
		},
		{
			name:  "eof after header",
			input: "600 URI Acquire\nURI: github://owner/repo",
			expectedMsg: &Message{
				Code: 600,
				Text: "URI Acquire",
				Headers: []Header{
					{Key: "URI", Value: "github://owner/repo"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			msg, err := ReadMessage(reader)

			if tt.expectErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedMsg.Code, msg.Code)
			assert.Equal(t, tt.expectedMsg.Text, msg.Text)
			assert.Equal(t, tt.expectedMsg.Headers, msg.Headers)
		})
	}
}

func TestMessageGet(t *testing.T) {
	msg := &Message{Code: 600, Text: "URI Acquire"}
	msg.Set("URI", "github://owner/repo/test")
	msg.Set("Filename", "/tmp/test")

	assert.Equal(t, "github://owner/repo/test", msg.Get("URI"))
	assert.Equal(t, "/tmp/test", msg.Get("Filename"))
	assert.Equal(t, "", msg.Get("Missing"))
}

func TestMessageRoundTrip(t *testing.T) {
	original := &Message{Code: 201, Text: "URI Done"}
	original.Set("URI", "github://owner/repo/pool/v1.0.0/test.deb")
	original.Set("Size", "12345")
	original.Set("SHA256-Hash", "abc123")

	var buf bytes.Buffer
	require.NoError(t, original.Write(&buf))

	reader := bufio.NewReader(&buf)
	parsed, err := ReadMessage(reader)

	require.NoError(t, err)
	assert.Equal(t, original.Code, parsed.Code)
	assert.Equal(t, original.Text, parsed.Text)
	assert.Equal(t, original.Get("URI"), parsed.Get("URI"))
	assert.Equal(t, original.Get("Size"), parsed.Get("Size"))
	assert.Equal(t, original.Get("SHA256-Hash"), parsed.Get("SHA256-Hash"))
}
