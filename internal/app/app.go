package app

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/vitalvas/apt-transport-github/internal/addrepo"
	"github.com/vitalvas/apt-transport-github/internal/cache"
	"github.com/vitalvas/apt-transport-github/internal/github"
	"github.com/vitalvas/apt-transport-github/internal/method"
	"github.com/vitalvas/apt-transport-github/internal/setup"
	"github.com/vitalvas/apt-transport-github/internal/signing"
)

func NewRootCmd(version string) *cobra.Command {
	return NewRootCmdWithIO(version, os.Stdin, os.Stdout)
}

func NewRootCmdWithIO(version string, stdin io.Reader, stdout io.Writer) *cobra.Command {
	github.SetVersion(version)

	rootCmd := &cobra.Command{
		Use:          "apt-transport-github",
		Short:        "APT transport method for GitHub releases",
		Version:      version,
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			signer := signing.NewGPGSigner(signing.DefaultGPGHome)
			m := method.NewWithSigner(signer)

			return m.Run(stdin, stdout)
		},
	}

	rootCmd.AddCommand(newSetupCmd())
	rootCmd.AddCommand(newCleanCmd())
	rootCmd.AddCommand(newAddRepoCmd())

	return rootCmd
}

func newSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Generate GPG signing key for APT repository metadata",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return setup.Run(cmd.OutOrStdout(), os.Geteuid(), signing.DefaultGPGHome, signing.DefaultPubKey)
		},
	}
}

func newAddRepoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add-repo <owner> <repo>",
		Short: "Add a GitHub repository as an APT source",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return addrepo.Run(cmd.OutOrStdout(), os.Geteuid(), args[0], args[1], addrepo.SourcesDir)
		},
	}
}

func newCleanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "Remove cached release metadata and package control data",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c := cache.New(cache.DefaultBaseDir)
			if err := c.Clean(); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Cache cleaned.")

			return nil
		},
	}
}
