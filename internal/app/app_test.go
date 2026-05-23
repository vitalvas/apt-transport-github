package app

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCmdRunsMethod(t *testing.T) {
	var stdout bytes.Buffer

	cmd := NewRootCmdWithIO("test", strings.NewReader(""), &stdout)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.NoError(t, err)

	assert.Contains(t, stdout.String(), "100 Capabilities")
}

func TestNewRootCmd(t *testing.T) {
	cmd := NewRootCmd("1.2.3")

	assert.Equal(t, "apt-transport-github", cmd.Use)
	assert.Equal(t, "1.2.3", cmd.Version)
	assert.NotNil(t, cmd.RunE)
}

func TestSetupSubcommand(t *testing.T) {
	cmd := NewRootCmd("test")
	cmd.SetArgs([]string{"setup"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "root")
}

func TestUnknownSubcommand(t *testing.T) {
	cmd := NewRootCmd("test")
	cmd.SetArgs([]string{"unknown"})

	err := cmd.Execute()
	assert.Error(t, err)
}

func TestSetupCmdExists(t *testing.T) {
	cmd := NewRootCmd("test")

	setupCmd, _, err := cmd.Find([]string{"setup"})
	require.NoError(t, err)
	assert.Equal(t, "setup", setupCmd.Use)
}

func TestCleanSubcommand(t *testing.T) {
	cmd := NewRootCmd("test")
	cmd.SetArgs([]string{"clean"})

	err := cmd.Execute()
	require.NoError(t, err)
}

func TestCleanCmdExists(t *testing.T) {
	cmd := NewRootCmd("test")

	cleanCmd, _, err := cmd.Find([]string{"clean"})
	require.NoError(t, err)
	assert.Equal(t, "clean", cleanCmd.Use)
}

func TestAddRepoSubcommand(t *testing.T) {
	cmd := NewRootCmd("test")
	cmd.SetArgs([]string{"add-repo", "owner", "repo"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "root")
}

func TestAddRepoCmdExists(t *testing.T) {
	cmd := NewRootCmd("test")

	addRepoCmd, _, err := cmd.Find([]string{"add-repo"})
	require.NoError(t, err)
	assert.Equal(t, "add-repo <owner> <repo>", addRepoCmd.Use)
}

func TestAddRepoRequiresArgs(t *testing.T) {
	cmd := NewRootCmd("test")
	cmd.SetArgs([]string{"add-repo"})

	err := cmd.Execute()
	require.Error(t, err)
}
