package method

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/vitalvas/apt-transport-github/internal/cache"
	"github.com/vitalvas/apt-transport-github/internal/deb"
	"github.com/vitalvas/apt-transport-github/internal/github"
	"github.com/vitalvas/apt-transport-github/internal/signing"
)

type Method struct {
	client    *github.Client
	signer    signing.Signer
	diskCache *cache.DiskCache
	arch      string
	sysArchs  []string
	cache     map[string]*repoState
	logger    *log.Logger
}

type repoState struct {
	debInfos []github.DebInfo
	assets   map[string]github.Asset // pool path -> asset
	verified bool
}

func New() *Method {
	return NewWithOptions(nil, cache.DefaultBaseDir)
}

func NewWithSigner(signer signing.Signer) *Method {
	return NewWithOptions(signer, cache.DefaultBaseDir)
}

func NewWithOptions(signer signing.Signer, cacheDir string) *Method {
	return &Method{
		client:    github.NewClient(),
		signer:    signer,
		diskCache: cache.New(cacheDir),
		arch:      systemArch(),
		sysArchs:  systemArchitectures(),
		cache:     make(map[string]*repoState),
		logger:    log.New(os.Stderr, "apt-transport-github: ", 0),
	}
}

func (m *Method) Run(in io.Reader, out io.Writer) error {
	caps := &Message{Code: 100, Text: "Capabilities"}
	caps.Set("Version", "1.2")
	caps.Set("Single-Instance", "true")
	caps.Set("Send-Config", "true")

	if err := caps.Write(out); err != nil {
		return err
	}

	reader := bufio.NewReader(in)

	for {
		msg, err := ReadMessage(reader)
		if err != nil {
			if err == io.EOF {
				return nil
			}

			return err
		}

		if msg.Code == 600 {
			if err := m.handleAcquire(msg, out); err != nil {
				return err
			}
		}
	}
}

const defaultVersions = 1

var goArchToDebian = map[string]string{
	"amd64": "amd64",
	"arm64": "arm64",
	"386":   "i386",
	"arm":   "armhf",
}

func systemArch() string {
	if debArch, ok := goArchToDebian[runtime.GOARCH]; ok {
		return debArch
	}

	return runtime.GOARCH
}

func systemArchitectures() []string {
	archs := make(map[string]struct{})

	if out, err := exec.Command("dpkg", "--print-architecture").Output(); err == nil {
		if arch := strings.TrimSpace(string(out)); arch != "" {
			archs[arch] = struct{}{}
		}
	}

	if out, err := exec.Command("dpkg", "--print-foreign-architectures").Output(); err == nil {
		for _, arch := range strings.Fields(string(out)) {
			archs[arch] = struct{}{}
		}
	}

	if len(archs) == 0 {
		return []string{systemArch()}
	}

	result := make([]string, 0, len(archs))
	for arch := range archs {
		result = append(result, arch)
	}

	sort.Strings(result)

	return result
}

type parsedURI struct {
	Owner    string
	Repo     string
	Path     string
	Versions int
}

func parseURI(uri string) (*parsedURI, error) {
	if !strings.HasPrefix(uri, "github://") {
		return nil, fmt.Errorf("invalid URI scheme: %s", uri)
	}

	trimmed := strings.TrimPrefix(uri, "github://")

	queryPart := ""
	if idx := strings.Index(trimmed, "?"); idx >= 0 {
		queryPart = trimmed[idx+1:]
		trimmed = trimmed[:idx]
	}

	parts := strings.SplitN(trimmed, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid URI: %s", uri)
	}

	p := &parsedURI{
		Owner:    parts[0],
		Repo:     parts[1],
		Versions: defaultVersions,
	}

	if len(parts) == 3 {
		p.Path = parts[2]
	}

	if queryPart != "" {
		params, err := url.ParseQuery(queryPart)
		if err == nil {
			if v := params.Get("versions"); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n > 0 {
					p.Versions = n
				}
			}
		}
	}

	return p, nil
}

func (m *Method) handleAcquire(msg *Message, out io.Writer) error {
	uri := msg.Get("URI")
	filename := msg.Get("Filename")

	parsed, err := parseURI(uri)
	if err != nil {
		return sendFailure(out, uri, err.Error())
	}

	switch {
	case strings.HasSuffix(parsed.Path, "InRelease"):
		return m.handleInRelease(parsed, uri, filename, out)

	case strings.HasSuffix(parsed.Path, "Release.gpg"):
		return sendFailure(out, uri, "Release.gpg not available, use InRelease")

	case strings.HasSuffix(parsed.Path, "/Release"):
		return m.handleRelease(parsed, uri, filename, out)

	case strings.HasSuffix(parsed.Path, "/Packages.gz"):
		return m.handlePackages(parsed, uri, filename, out, true)

	case strings.HasSuffix(parsed.Path, "/Packages"):
		return m.handlePackages(parsed, uri, filename, out, false)

	case strings.HasPrefix(parsed.Path, "pool/"):
		return m.handlePool(parsed, uri, filename, out)

	default:
		return sendFailure(out, uri, "unknown request path")
	}
}

func (m *Method) loadRepo(parsed *parsedURI, out io.Writer) (*repoState, error) {
	key := fmt.Sprintf("%s/%s", parsed.Owner, parsed.Repo)

	if state, ok := m.cache[key]; ok {
		return state, nil
	}

	status := &Message{Code: 102, Text: "Status"}
	status.Set("URI", fmt.Sprintf("github://%s", key))
	status.Set("Message", "Fetching release info from GitHub")

	if err := status.Write(out); err != nil {
		return nil, err
	}

	releases, err := m.fetchReleases(parsed.Owner, parsed.Repo, parsed.Versions)
	if err != nil {
		return nil, fmt.Errorf("failed to get releases: %w", err)
	}

	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found")
	}

	verified, err := m.client.VerifyTagSignature(parsed.Owner, parsed.Repo, releases[0].TagName)
	if err != nil {
		return nil, fmt.Errorf("failed to verify tag signature: %w", err)
	}

	var allDebInfos []github.DebInfo

	assets := make(map[string]github.Asset)

	for ri := range releases {
		release := &releases[ri]

		checksums := make(map[string]string)

		if csAsset := release.FindChecksumsAsset(); csAsset != nil {
			content, err := m.client.FetchAssetContent(parsed.Owner, parsed.Repo, *csAsset)
			if err != nil {
				m.logger.Printf("warning: failed to fetch checksums for %s: %s", release.TagName, err)
			} else {
				checksums = github.ParseChecksums(content)
			}
		}

		allDebInfo := release.CollectDebInfo(checksums)

		for i, info := range allDebInfo {
			uriPath := poolPath(release.TagName, info.Asset.Name)
			assets[uriPath] = info.Asset

			if !m.shouldLoadControl(info.Arch) {
				continue
			}

			fields, computedSHA256 := m.loadControlFields(info, parsed.Owner, parsed.Repo, release.TagName, info.Asset.Name)

			for _, f := range fields {
				allDebInfo[i].Control = append(allDebInfo[i].Control, github.ControlField{
					Key:   f.Key,
					Value: f.Value,
				})
			}

			if allDebInfo[i].SHA256 == "" && computedSHA256 != "" {
				allDebInfo[i].SHA256 = computedSHA256
			}
		}

		allDebInfos = append(allDebInfos, allDebInfo...)
	}

	validTags := make(map[string]bool, len(releases))
	for _, release := range releases {
		validTags[release.TagName] = true
	}

	if err := m.diskCache.CleanStaleTags(parsed.Owner, parsed.Repo, validTags); err != nil {
		m.logger.Printf("warning: failed to clean stale cache tags for %s/%s: %s", parsed.Owner, parsed.Repo, err)
	}

	state := &repoState{
		debInfos: allDebInfos,
		assets:   assets,
		verified: verified,
	}

	m.cache[key] = state

	return state, nil
}

func (m *Method) fetchReleases(owner, repo string, limit int) ([]github.Release, error) {
	if data, ok := m.diskCache.GetReleases(owner, repo, limit); ok {
		var releases []github.Release
		if err := json.Unmarshal(data, &releases); err == nil {
			return releases, nil
		}
	}

	releases, err := m.client.GetReleases(owner, repo, limit)
	if err != nil {
		return nil, err
	}

	if data, err := json.Marshal(releases); err == nil {
		if err := m.diskCache.PutReleases(owner, repo, limit, data); err != nil {
			m.logger.Printf("warning: failed to cache releases for %s/%s: %s", owner, repo, err)
		}
	}

	return releases, nil
}

func (m *Method) shouldLoadControl(arch string) bool {
	if arch == "all" || arch == m.arch {
		return true
	}

	for _, sysArch := range m.sysArchs {
		if arch == sysArch {
			return true
		}
	}

	return false
}

func (m *Method) loadControlFields(info github.DebInfo, owner, repo, tag, filename string) ([]cache.Field, string) {
	if entry, ok := m.diskCache.GetControl(owner, repo, tag, filename); ok {
		return entry.Fields, entry.SHA256
	}

	debData, err := m.client.FetchAssetBytes(owner, repo, info.Asset)
	if err != nil {
		m.logger.Printf("warning: failed to fetch package %s/%s %s: %s", owner, repo, filename, err)
		return nil, ""
	}

	if _, err := m.diskCache.PutPackage(owner, repo, tag, filename, debData); err != nil {
		m.logger.Printf("warning: failed to cache package %s/%s %s: %s", owner, repo, filename, err)
	}

	ctrl, err := deb.ParseControl(debData)
	if err != nil {
		m.logger.Printf("warning: failed to parse control for %s/%s %s: %s", owner, repo, filename, err)
		return nil, ""
	}

	hash := sha256.Sum256(debData)
	sha256Hex := fmt.Sprintf("%x", hash)

	fields := make([]cache.Field, 0, len(ctrl.Fields))
	for _, f := range ctrl.Fields {
		fields = append(fields, cache.Field{Key: f.Key, Value: f.Value})
	}

	if err := m.diskCache.PutControl(owner, repo, tag, filename, &cache.Entry{
		Fields: fields,
		SHA256: sha256Hex,
	}); err != nil {
		m.logger.Printf("warning: failed to cache control for %s/%s %s: %s", owner, repo, filename, err)
	}

	return fields, sha256Hex
}

func (m *Method) handleInRelease(parsed *parsedURI, uri, filename string, out io.Writer) error {
	if m.signer == nil {
		return sendFailure(out, uri, "signing not configured, run: apt-transport-github setup")
	}

	state, err := m.loadRepo(parsed, out)
	if err != nil {
		return sendFailure(out, uri, err.Error())
	}

	if !state.verified {
		return sendFailure(out, uri, "GitHub tag signature verification failed")
	}

	releaseContent := m.generateReleaseContent(parsed, state)

	signed, err := m.signer.ClearSign(releaseContent)
	if err != nil {
		return sendFailure(out, uri, fmt.Sprintf("signing failed: %s", err))
	}

	return writeFileAndRespond(out, uri, filename, signed)
}

func (m *Method) handleRelease(parsed *parsedURI, uri, filename string, out io.Writer) error {
	state, err := m.loadRepo(parsed, out)
	if err != nil {
		return sendFailure(out, uri, err.Error())
	}

	content := m.generateReleaseContent(parsed, state)

	return writeFileAndRespond(out, uri, filename, content)
}

func (m *Method) generateReleaseContent(parsed *parsedURI, state *repoState) []byte {
	archSet := make(map[string]struct{})

	for _, info := range state.debInfos {
		if info.Arch != "all" {
			archSet[info.Arch] = struct{}{}
		}
	}

	for _, arch := range m.sysArchs {
		archSet[arch] = struct{}{}
	}

	archs := make([]string, 0, len(archSet))
	for arch := range archSet {
		archs = append(archs, arch)
	}

	sort.Strings(archs)

	type indexEntry struct {
		path    string
		content []byte
	}

	entries := make([]indexEntry, 0, 2*len(archs))

	for _, arch := range archs {
		pkgContent := m.generatePackagesContent(state, arch)
		pkgPath := fmt.Sprintf("main/binary-%s/Packages", arch)
		entries = append(entries, indexEntry{path: pkgPath, content: pkgContent})

		var gzBuf bytes.Buffer
		gz := gzip.NewWriter(&gzBuf)
		gz.Write(pkgContent)
		gz.Close()

		gzPath := fmt.Sprintf("main/binary-%s/Packages.gz", arch)
		entries = append(entries, indexEntry{path: gzPath, content: gzBuf.Bytes()})
	}

	var buf bytes.Buffer

	fmt.Fprintf(&buf, "Origin: github.com\n")
	fmt.Fprintf(&buf, "Label: %s/%s\n", parsed.Owner, parsed.Repo)
	fmt.Fprintf(&buf, "Suite: stable\n")
	fmt.Fprintf(&buf, "Codename: stable\n")
	fmt.Fprintf(&buf, "Architectures: %s\n", strings.Join(archs, " "))
	fmt.Fprintf(&buf, "Components: main\n")
	fmt.Fprintf(&buf, "Date: %s\n", time.Now().UTC().Format(time.RFC1123))

	fmt.Fprintf(&buf, "MD5Sum:\n")
	for _, e := range entries {
		hash := md5.Sum(e.content)
		fmt.Fprintf(&buf, " %x %d %s\n", hash, len(e.content), e.path)
	}

	fmt.Fprintf(&buf, "SHA256:\n")
	for _, e := range entries {
		hash := sha256.Sum256(e.content)
		fmt.Fprintf(&buf, " %x %d %s\n", hash, len(e.content), e.path)
	}

	return buf.Bytes()
}

func (m *Method) handlePackages(parsed *parsedURI, uri, filename string, out io.Writer, compressed bool) error {
	state, err := m.loadRepo(parsed, out)
	if err != nil {
		return sendFailure(out, uri, err.Error())
	}

	arch := extractArch(parsed.Path)
	if arch == "" {
		return sendFailure(out, uri, "cannot determine architecture from path")
	}

	content := m.generatePackagesContent(state, arch)

	if compressed {
		var gzBuf bytes.Buffer
		gz := gzip.NewWriter(&gzBuf)

		if _, err := gz.Write(content); err != nil {
			return sendFailure(out, uri, err.Error())
		}

		if err := gz.Close(); err != nil {
			return sendFailure(out, uri, err.Error())
		}

		content = gzBuf.Bytes()
	}

	return writeFileAndRespond(out, uri, filename, content)
}

func (m *Method) handlePool(parsed *parsedURI, uri, filename string, out io.Writer) error {
	state, err := m.loadRepo(parsed, out)
	if err != nil {
		return sendFailure(out, uri, err.Error())
	}

	asset, ok := state.assets[parsed.Path]
	if !ok {
		return sendFailure(out, uri, "asset not found")
	}

	tag, assetFilename, err := parsePoolPath(parsed.Path)
	if err != nil {
		return sendFailure(out, uri, err.Error())
	}

	status := &Message{Code: 200, Text: "URI Start"}
	status.Set("URI", uri)

	if err := status.Write(out); err != nil {
		return err
	}

	if cachedPath, ok := m.diskCache.GetPackage(parsed.Owner, parsed.Repo, tag, assetFilename); ok {
		if err := copyFile(cachedPath, filename); err == nil {
			return m.respondPoolDone(uri, filename, out)
		}
	}

	size, err := m.client.DownloadAssetFile(parsed.Owner, parsed.Repo, asset, filename)
	if err != nil {
		return sendFailure(out, uri, fmt.Sprintf("download failed: %s", err))
	}

	done := &Message{Code: 201, Text: "URI Done"}
	done.Set("URI", uri)
	done.Set("Filename", filename)
	done.Set("Size", fmt.Sprintf("%d", size))

	hashes, err := hashFile(filename)
	if err != nil {
		return sendFailure(out, uri, fmt.Sprintf("hash failed: %s", err))
	}

	done.Set("MD5-Hash", hashes.md5)
	done.Set("SHA256-Hash", hashes.sha256)

	return done.Write(out)
}

var controlPassthrough = map[string]bool{
	"Package":        true,
	"Version":        true,
	"Architecture":   true,
	"Maintainer":     true,
	"Description":    true,
	"Depends":        true,
	"Pre-Depends":    true,
	"Recommends":     true,
	"Suggests":       true,
	"Conflicts":      true,
	"Breaks":         true,
	"Replaces":       true,
	"Provides":       true,
	"Enhances":       true,
	"Section":        true,
	"Priority":       true,
	"Installed-Size": true,
	"Homepage":       true,
}

func (m *Method) generatePackagesContent(state *repoState, arch string) []byte {
	var buf bytes.Buffer
	first := true

	for _, info := range state.debInfos {
		if info.Arch != arch && info.Arch != "all" {
			continue
		}

		if !first {
			buf.WriteString("\n")
		}

		first = false

		poolFilename := poolPath(info.Tag, info.Asset.Name)

		if len(info.Control) > 0 {
			for _, f := range info.Control {
				if controlPassthrough[f.Key] && f.Value != "" {
					fmt.Fprintf(&buf, "%s: %s\n", f.Key, f.Value)
				}
			}
		} else {
			fmt.Fprintf(&buf, "Package: %s\n", info.Name)
			fmt.Fprintf(&buf, "Version: %s\n", info.Version)
			fmt.Fprintf(&buf, "Architecture: %s\n", info.Arch)
		}

		fmt.Fprintf(&buf, "Filename: %s\n", poolFilename)
		fmt.Fprintf(&buf, "Size: %d\n", info.Asset.Size)

		if info.SHA256 != "" {
			fmt.Fprintf(&buf, "SHA256: %s\n", info.SHA256)
		}
	}

	return buf.Bytes()
}

func extractArch(path string) string {
	const prefix = "binary-"

	idx := strings.Index(path, prefix)
	if idx < 0 {
		return ""
	}

	rest := path[idx+len(prefix):]

	if slashIdx := strings.Index(rest, "/"); slashIdx >= 0 {
		return rest[:slashIdx]
	}

	return rest
}

func poolPath(tag, filename string) string {
	return fmt.Sprintf("pool/%s/%s", url.PathEscape(tag), url.PathEscape(filename))
}

func parsePoolPath(path string) (tag, filename string, err error) {
	poolSuffix := strings.TrimPrefix(path, "pool/")
	parts := strings.SplitN(poolSuffix, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid pool path")
	}

	tag, err = url.PathUnescape(parts[0])
	if err != nil {
		return "", "", fmt.Errorf("invalid pool tag: %w", err)
	}

	filename, err = url.PathUnescape(parts[1])
	if err != nil {
		return "", "", fmt.Errorf("invalid pool filename: %w", err)
	}

	return tag, filename, nil
}

type fileHashes struct {
	md5    string
	sha256 string
}

func hashFile(path string) (*fileHashes, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	md5Hash := md5.New()
	sha256Hash := sha256.New()
	w := io.MultiWriter(md5Hash, sha256Hash)

	if _, err := io.Copy(w, f); err != nil {
		return nil, err
	}

	return &fileHashes{
		md5:    fmt.Sprintf("%x", md5Hash.Sum(nil)),
		sha256: fmt.Sprintf("%x", sha256Hash.Sum(nil)),
	}, nil
}

func writeFileAndRespond(out io.Writer, uri, filename string, content []byte) error {
	start := &Message{Code: 200, Text: "URI Start"}
	start.Set("URI", uri)
	start.Set("Size", fmt.Sprintf("%d", len(content)))

	if err := start.Write(out); err != nil {
		return err
	}

	if err := os.WriteFile(filename, content, 0644); err != nil {
		return sendFailure(out, uri, fmt.Sprintf("write failed: %s", err))
	}

	md5Hash := md5.Sum(content)
	sha256Hash := sha256.Sum256(content)

	done := &Message{Code: 201, Text: "URI Done"}
	done.Set("URI", uri)
	done.Set("Filename", filename)
	done.Set("Size", fmt.Sprintf("%d", len(content)))
	done.Set("Last-Modified", time.Now().UTC().Format(time.RFC1123))
	done.Set("MD5-Hash", fmt.Sprintf("%x", md5Hash))
	done.Set("SHA256-Hash", fmt.Sprintf("%x", sha256Hash))

	return done.Write(out)
}

func (m *Method) respondPoolDone(uri, filename string, out io.Writer) error {
	hashes, err := hashFile(filename)
	if err != nil {
		return sendFailure(out, uri, fmt.Sprintf("hash failed: %s", err))
	}

	info, err := os.Stat(filename)
	if err != nil {
		return sendFailure(out, uri, fmt.Sprintf("stat failed: %s", err))
	}

	done := &Message{Code: 201, Text: "URI Done"}
	done.Set("URI", uri)
	done.Set("Filename", filename)
	done.Set("Size", fmt.Sprintf("%d", info.Size()))
	done.Set("MD5-Hash", hashes.md5)
	done.Set("SHA256-Hash", hashes.sha256)

	return done.Write(out)
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)

	return err
}

func sendFailure(out io.Writer, uri, message string) error {
	msg := &Message{Code: 400, Text: "URI Failure"}
	msg.Set("URI", uri)
	msg.Set("Message", message)

	return msg.Write(out)
}
