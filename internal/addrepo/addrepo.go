package addrepo

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/vitalvas/apt-transport-github/internal/signing"
)

const SourcesDir = "/etc/apt/sources.list.d"

var githubNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

func Run(w io.Writer, euid int, owner, repo, destDir string) error {
	if euid != 0 {
		return fmt.Errorf("add-repo must be run as root")
	}

	if !githubNamePattern.MatchString(owner) {
		return fmt.Errorf("invalid GitHub owner: %s", owner)
	}

	if !githubNamePattern.MatchString(repo) {
		return fmt.Errorf("invalid GitHub repo: %s", repo)
	}

	filename := filepath.Join(destDir, fmt.Sprintf("%s.sources", repo))

	content := fmt.Sprintf("Types: deb\nURIs: github://%s/%s\nSuites: stable\nComponents: main\nSigned-By: %s\n",
		owner, repo, signing.DefaultPubKey,
	)

	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("repository file already exists: %s", filename)
		}

		return fmt.Errorf("failed to write sources file: %w", err)
	}

	if _, err := file.WriteString(content); err != nil {
		file.Close()
		return fmt.Errorf("failed to write sources file: %w", err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to write sources file: %w", err)
	}

	fmt.Fprintf(w, "Repository added: %s\n", filename)

	return nil
}
