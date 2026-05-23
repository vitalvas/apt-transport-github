# apt-transport-github

APT transport method for installing `.deb` packages directly from GitHub Releases.

## Overview

`apt-transport-github` is an APT transport plugin that allows you to use GitHub repositories as APT package sources. It fetches `.deb` packages from GitHub Releases, verifies tag signatures via the GitHub API, and signs APT repository metadata with a local GPG key.

Designed to work with [goreleaser](https://goreleaser.com/) projects that publish `.deb` packages as release assets.

## How It Works

```
GitHub Release (signed tag)
       |
       v
GitHub API (verify tag signature)
       |
       v
Generate APT metadata (Release, Packages)
       |
       v
Clearsign with local Ed25519 GPG key
       |
       v
APT verifies signature via signed-by keyring
```

1. On `apt update`, the transport fetches the latest GitHub release and verifies the tag signature via the GitHub API.
2. It parses `.deb` assets, resolves SHA256 hashes, and generates APT-compatible `Packages` and `Release` index files.
3. The `Release` file is clearsigned to produce `InRelease`, which APT verifies using the local public key.
4. On `apt install`, the `.deb` is downloaded directly from the GitHub release asset URL.

## Installation

Download and install the latest `.deb` package from [GitHub Releases](https://github.com/vitalvas/apt-transport-github/releases).

The postinstall script automatically generates the GPG signing key. To regenerate it manually:

```bash
sudo /usr/lib/apt/methods/github setup
```

The signing key is stored at:
- Private key in `/etc/apt-transport-github/gpg/`
- Public key at `/etc/apt/keyrings/apt-transport-github.gpg`

## Usage

Add a GitHub repository as an APT source:

```bash
echo 'deb [signed-by=/etc/apt/keyrings/apt-transport-github.gpg] github://OWNER/REPO stable main' \
  | sudo tee /etc/apt/sources.list.d/REPO.list
```

Then use standard APT commands:

```bash
sudo apt update
sudo apt install PACKAGE_NAME
```

### Example

Once installed, apt-transport-github can manage its own updates:

```bash
echo 'deb [signed-by=/etc/apt/keyrings/apt-transport-github.gpg] github://vitalvas/apt-transport-github stable main' \
  | sudo tee /etc/apt/sources.list.d/apt-transport-github.list

sudo apt update
sudo apt install apt-transport-github
```

### DEB822 Format

You can also use the modern DEB822 format (`.sources` files):

```bash
cat <<EOF | sudo tee /etc/apt/sources.list.d/apt-transport-github.sources
Types: deb
URIs: github://vitalvas/apt-transport-github
Suites: stable
Components: main
Signed-By: /etc/apt/keyrings/apt-transport-github.gpg
EOF
```

### Version History

By default, the latest release is available. To include older releases for version pinning, add the `versions` query parameter:

```
deb [signed-by=/etc/apt/keyrings/apt-transport-github.gpg] github://OWNER/REPO?versions=20 stable main
```

> [!WARNING]
> Each version requires downloading the `.deb` file to extract package metadata during `apt update` (results are cached on disk for subsequent runs). Higher version counts increase the initial `apt update` time and GitHub API usage. The unauthenticated GitHub API rate limit is 60 requests per hour.

### Priority Pinning

APT priority pinning lets you control how packages from GitHub repos are preferred relative to other sources. The transport generates `Origin: github.com` and `Label: {owner}/{repo}` in the Release file.

Pin a specific repo higher than default:

```
# /etc/apt/preferences.d/apt-transport-github.pref
Package: *
Pin: release o=github.com,l=vitalvas/apt-transport-github
Pin-Priority: 990
```

Pin all GitHub repos lower than official:

```
# /etc/apt/preferences.d/apt-transport-github.pref
Package: *
Pin: release o=github.com
Pin-Priority: 400
```

Verify with:

```bash
apt-cache policy <package-name>
```

### Cache

Release metadata and package control data are cached locally at `/var/cache/apt-transport-github/` in a tree organized by `{owner}/{repo}/{tag}/`. The release metadata cache has a 5-minute TTL; control metadata and downloaded `.deb` files are cached indefinitely. Stale tag directories are automatically removed when releases are refreshed.

To clear the cache:

```bash
sudo /usr/lib/apt/methods/github clean
```

### Authentication

To avoid GitHub API rate limits (60 requests/hour unauthenticated) or to access private repositories, provide a Personal Access Token (PAT).

Tokens are stored in `/etc/apt-transport-github/tokens/` with one file per scope. The token is resolved in the following order:

1. `tokens/repo_<owner>__<repo>` - specific repository
2. `tokens/repo_<owner>` - all repositories under an owner/organization
3. `tokens/default` - fallback for all repositories
4. `GITHUB_TOKEN` environment variable

Each file contains just the raw token. To set up tokens:

```bash
# Default token for all repos
echo "ghp_defaulttoken" | sudo tee /etc/apt-transport-github/tokens/default

# Token for all repos under an owner/organization
echo "ghp_ownertoken" | sudo tee /etc/apt-transport-github/tokens/repo_vitalvas

# Token for a specific repo
echo "ghp_repotoken" | sudo tee /etc/apt-transport-github/tokens/repo_vitalvas__myapp

# Secure the directory
sudo chmod 700 /etc/apt-transport-github/tokens
sudo chmod 600 /etc/apt-transport-github/tokens/*
```

A classic token with no scopes (public repo access only) is sufficient for public repositories.

#### Fine-Grained Tokens

For private repositories or to avoid rate limits, create a [fine-grained personal access token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens#creating-a-fine-grained-personal-access-token) with the following repository permission:

| Permission | Access | Used for |
|---|---|---|
| Contents | Read-only | Fetching releases, downloading assets, verifying tag signatures |
| Metadata | Read-only | Accessing repository information (automatically included) |

The token must be scoped to the repositories you want to install packages from.

## Requirements

- `gpg` (runtime, for signing)
- GitHub releases with `.deb` assets (goreleaser naming convention)
- goreleaser's `checksums.txt` in the release assets (optional; see hash resolution below)

### Hash Resolution

SHA256 hashes for `.deb` packages are resolved in the following order:

1. **checksums.txt** from the release assets (goreleaser default)
2. **GitHub API `digest`** field from asset metadata
3. **Local computation** from the downloaded `.deb` file

### Supported `.deb` Naming Patterns

Both goreleaser naming conventions are supported:

- `{name}_{version}_{os}_{arch}.deb` (e.g., `myapp_1.0.0_linux_amd64.deb`)
- `{name}_{version}_{arch}.deb` (e.g., `myapp_1.0.0_amd64.deb`)

## Security

The trust chain:

1. The GitHub release tag must be signed (verified via GitHub API's `verification.verified` field).
2. If verification passes, the transport signs the generated APT metadata with a local Ed25519 GPG key.
3. APT verifies the `InRelease` signature using the public key specified in `signed-by`.

If the GitHub tag signature verification fails, the transport refuses to serve signed metadata.

