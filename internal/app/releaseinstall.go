package app

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
	"howett.net/plist"
)

const releaseAssetLimit = 256 << 20

var releaseTagPattern = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.-]+)?$`)
var releaseRepoPattern = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9_.-]*/[A-Za-z0-9_-][A-Za-z0-9_.-]*$`)

type releaseInstaller struct {
	repo, version, platform, arch, binDir, appDir string
	mirror                                        string
	cliOnly, skipCLI                              bool
	client                                        *http.Client
	activateApp                                   func(string) error
	replaceApp                                    func(string, string) error
}

type releaseInstallResult struct {
	CLIInstalled, AppInstalled bool
	AppError                   error
}

func newReleaseInstaller(cliOnly bool, version string) (*releaseInstaller, error) {
	if pin := os.Getenv("GTMUX_VERSION"); pin != "" {
		version = pin
	}
	if !releaseTagPattern.MatchString(version) {
		return nil, fmt.Errorf("invalid or unresolved release tag %q", version)
	}
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	repo := os.Getenv("GTMUX_REPO")
	if repo == "" {
		repo = "chenchaoyi/gtmux"
	}
	if !releaseRepoPattern.MatchString(repo) {
		return nil, fmt.Errorf("invalid release repository %q", repo)
	}
	binDir := os.Getenv("GTMUX_BIN_DIR")
	if binDir == "" {
		self, err := os.Executable()
		if err != nil {
			return nil, err
		}
		if real, err := filepath.EvalSymlinks(self); err == nil {
			self = real
		}
		binDir = filepath.Dir(self)
	}
	appDir := filepath.Join(state.Home(), "Applications")
	if apps := installedAppCandidates(state.Home(), "/Applications/Gtmux.app"); len(apps) > 0 {
		appDir = filepath.Dir(apps[0])
	}
	x := &releaseInstaller{repo: repo, version: version, platform: runtime.GOOS, arch: runtime.GOARCH,
		binDir: binDir, appDir: appDir, mirror: os.Getenv("GTMUX_INSTALL_MIRROR"),
		cliOnly:     cliOnly || os.Getenv("GTMUX_NO_APP") != "" || runtime.GOOS != "darwin",
		client:      &http.Client{Timeout: 3 * time.Minute},
		activateApp: activateReleaseApp,
	}
	x.selectComponents(Version, installedAppUpdateVersion(), os.Getenv("GTMUX_VERSION") != "")
	if (x.platform != "darwin" && x.platform != "linux") || (x.arch != "arm64" && x.arch != "amd64") {
		return nil, fmt.Errorf("unsupported platform %s/%s", x.platform, x.arch)
	}
	return x, nil
}

// Explicit version pins retain their existing reinstall/downgrade behavior.
func (x *releaseInstaller) selectComponents(cli, app string, pinned bool) {
	if pinned {
		return
	}
	x.skipCLI = !newerRelease(x.version, cli)
	if app != "" && !newerRelease(x.version, app) {
		x.cliOnly = true
	}
}

// officialReleaseURL bounds the checksum/app trust anchor, including redirects.
// A mirror may supply archive bytes ONLY after their GitHub digest is known.
func officialReleaseURL(u *url.URL) bool {
	if u.Scheme != "https" || u.User != nil || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	switch u.Hostname() {
	case "github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com":
		return true
	}
	return false
}

func (x *releaseInstaller) download(raw, dest string, limit int64, official bool) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if official && !officialReleaseURL(u) {
		return fmt.Errorf("untrusted release source: %s", raw)
	}
	client := *x.client
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many release redirects")
		}
		if official && !officialReleaseURL(req.URL) {
			return fmt.Errorf("untrusted release redirect: %s", req.URL)
		}
		return nil
	}
	resp, err := client.Get(raw)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", raw, resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, io.LimitReader(resp.Body, limit+1))
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if n > limit {
		return fmt.Errorf("release asset exceeds %d bytes", limit)
	}
	return nil
}

func releaseChecksum(body []byte, name string) (string, error) {
	var found string
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != name {
			continue
		}
		digest, err := hex.DecodeString(fields[0])
		if err != nil || len(digest) != sha256.Size || found != "" {
			return "", fmt.Errorf("invalid or duplicate checksum for %s", name)
		}
		found = strings.ToLower(fields[0])
	}
	if found == "" {
		return "", fmt.Errorf("missing GitHub checksum for %s", name)
	}
	return found, nil
}

func (x *releaseInstaller) archiveURLs(official string) ([]string, error) {
	proxies := []string{"https://ghfast.top/", "https://gh-proxy.com/", "https://ghproxy.net/"}
	urls := []string{official}
	switch x.mirror {
	case "github":
		return urls, nil
	case "", "auto":
	case "ghproxy":
		urls = nil
	default:
		u, err := url.Parse(x.mirror)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return nil, fmt.Errorf("invalid GTMUX_INSTALL_MIRROR")
		}
		proxies = []string{strings.TrimRight(x.mirror, "/") + "/"}
	}
	for _, proxy := range proxies {
		urls = append(urls, proxy+official)
	}
	return urls, nil
}

func (x *releaseInstaller) install() (releaseInstallResult, error) {
	var result releaseInstallResult
	tmp, err := os.MkdirTemp("", "gtmux-release-*")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(tmp)
	base := "https://github.com/" + x.repo + "/releases/download/" + x.version + "/"
	if !x.skipCLI {
		if err := x.installCLI(tmp, base); err != nil {
			return result, err
		}
		result.CLIInstalled = true
	}
	if !x.cliOnly {
		result.AppInstalled, result.AppError = x.installApp(tmp, base)
	}
	return result, nil
}

func (x *releaseInstaller) installCLI(tmp, base string) error {
	name := fmt.Sprintf("gtmux-%s-%s-%s.tar.gz", strings.TrimPrefix(x.version, "v"), x.platform, x.arch)
	sums := filepath.Join(tmp, "SHASUMS256.txt")
	// Never fall back to a mirror for the checksum. A proxy able to replace both
	// archive and checksum would otherwise still control the installed executable.
	if err := x.download(base+"SHASUMS256.txt", sums, 1<<20, true); err != nil {
		return fmt.Errorf("authoritative checksums unavailable: %w", err)
	}
	body, err := os.ReadFile(sums)
	if err != nil {
		return err
	}
	expected, err := releaseChecksum(body, name)
	if err != nil {
		return err
	}
	urls, err := x.archiveURLs(base + name)
	if err != nil {
		return err
	}
	archive := filepath.Join(tmp, "cli.tar.gz")
	var lastErr error
	verified := false
	for _, raw := range urls {
		if lastErr = x.download(raw, archive, releaseAssetLimit, raw == base+name); lastErr != nil {
			continue
		}
		f, err := os.Open(archive)
		if err != nil {
			return err
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return err
		}
		if hex.EncodeToString(h.Sum(nil)) != expected {
			lastErr = fmt.Errorf("sha256 mismatch for %s", name)
			continue
		}
		verified = true
		break
	}
	if !verified {
		return fmt.Errorf("release archive unavailable or unverified: %w", lastErr)
	}
	binary := filepath.Join(tmp, "gtmux")
	if err := extractReleaseCLI(archive, binary); err != nil {
		return err
	}
	if err := replaceReleaseCLI(binary, x.binDir); err != nil {
		return err
	}
	i18n.Say("CLI installed: "+filepath.Join(x.binDir, "gtmux")+" ("+x.version+", SHA256 verified)",
		"CLI 已安装："+filepath.Join(x.binDir, "gtmux")+"（"+x.version+"，SHA256 校验通过）")
	return nil
}

func (x *releaseInstaller) installApp(tmp, base string) (bool, error) {
	// The app is published by a separate job, outside SHASUMS256.txt. Fetch
	// it only over GitHub's authenticated transport; proxies cannot supply it.
	appZip := filepath.Join(tmp, "app.zip")
	if err := x.download(base+"Gtmux-"+strings.TrimPrefix(x.version, "v")+"-macos.zip", appZip, releaseAssetLimit, true); err != nil {
		return false, err
	}
	if err := validateReleaseApp(appZip); err != nil {
		return false, err
	}
	if err := os.MkdirAll(x.appDir, 0o755); err != nil {
		return false, err
	}
	stagedApp, err := os.MkdirTemp(x.appDir, ".gtmux-app-*")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(stagedApp)
	// ditto preserves the signed macOS bundle's metadata. This is a fixed
	// local tool with arguments, never a downloaded script or shell program.
	if err := exec.Command("/usr/bin/ditto", "-x", "-k", appZip, stagedApp).Run(); err != nil {
		return false, err
	}
	if _, err := os.Stat(filepath.Join(stagedApp, "Gtmux.app", "Contents", "Info.plist")); err != nil {
		return false, fmt.Errorf("incomplete app bundle: %w", err)
	}
	target := filepath.Join(x.appDir, "Gtmux.app")
	replace := x.replaceApp
	if replace == nil {
		replace = replaceReleaseApp
	}
	if err := replace(filepath.Join(stagedApp, "Gtmux.app"), target); err != nil {
		return false, err
	}
	i18n.Say("Menu-bar app installed: "+target+" ("+x.version+")", "菜单栏 app 已安装："+target+"（"+x.version+"）")
	if x.activateApp != nil {
		return true, x.activateApp(target)
	}
	return true, nil
}

// Extract ONLY the root regular binary. Archive paths and links never become
// filesystem paths, and duplicate binary entries are rejected before installation.
func extractReleaseCLI(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(io.LimitReader(gz, 512<<20))
	found := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Name != "gtmux" && hdr.Name != "./gtmux" {
			continue
		}
		if found || hdr.Typeflag != tar.TypeReg || hdr.Size <= 0 || hdr.Size > releaseAssetLimit || hdr.Mode&0o111 == 0 {
			return fmt.Errorf("invalid release binary entry")
		}
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, tr)
		closeErr := out.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		found = true
	}
	if !found {
		return fmt.Errorf("release archive missing gtmux binary")
	}
	return nil
}

func replaceReleaseCLI(source, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.CreateTemp(dir, ".gtmux-*")
	if err != nil {
		return err
	}
	defer os.Remove(out.Name())
	_, err = io.Copy(out, in)
	if err == nil {
		err = out.Chmod(0o755)
	}
	if err == nil {
		err = out.Sync()
	}
	closeErr := out.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(out.Name(), filepath.Join(dir, "gtmux"))
}

func validateReleaseApp(file string) error {
	z, err := zip.OpenReader(file)
	if err != nil {
		return err
	}
	defer z.Close()
	var total uint64
	for _, f := range z.File {
		name := strings.TrimSuffix(f.Name, "/")
		if !filepath.IsLocal(name) || path.Clean(name) != name || strings.Contains(name, "\\") {
			return fmt.Errorf("unsafe app archive path %q", f.Name)
		}
		if name != "Gtmux.app" && name != "__MACOSX" && !strings.HasPrefix(name, "Gtmux.app/") && !strings.HasPrefix(name, "__MACOSX/") {
			return fmt.Errorf("unexpected app archive path %q", f.Name)
		}
		// Current release bundles contain files/directories only. Refuse links
		// rather than let an extractor follow one while writing a later entry.
		if !f.Mode().IsRegular() && !f.Mode().IsDir() {
			return fmt.Errorf("unsupported app archive entry %q", f.Name)
		}
		if f.UncompressedSize64 > 512<<20 || total > (1<<30)-f.UncompressedSize64 {
			return fmt.Errorf("app archive too large")
		}
		total += f.UncompressedSize64
	}
	return nil
}

func replaceReleaseApp(staged, target string) error {
	// Reserve a unique rollback location on the same filesystem. If promotion
	// fails, restore the existing bundle; never delete it before a valid swap.
	backup, err := os.MkdirTemp(filepath.Dir(target), ".gtmux-app-old-*")
	if err != nil {
		return err
	}
	cleanupBackup := true
	defer func() {
		if cleanupBackup {
			_ = os.RemoveAll(backup)
		}
	}()
	old := filepath.Join(backup, "Gtmux.app")
	hadOld := false
	if _, err := os.Lstat(target); err == nil {
		if err := os.Rename(target, old); err != nil {
			return err
		}
		hadOld = true
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(staged, target); err != nil {
		if hadOld {
			if restoreErr := os.Rename(old, target); restoreErr != nil {
				cleanupBackup = false
				return fmt.Errorf("app swap failed: %v; restore failed: %v (old bundle at %s)", err, restoreErr, old)
			}
		}
		return err
	}
	return nil
}

func activateReleaseApp(target string) error {
	_ = runBounded(10*time.Second, "xattr", "-dr", "com.apple.quarantine", target)
	_ = runBounded(10*time.Second, "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister", "-f", target)
	if os.Getenv("GTMUX_APP_LOGIN") != "" {
		if err := setAppAutostart(target); err != nil {
			return err
		}
	}
	// Retain open -n and strip the one-shot version pin: the app's update
	// action must resolve a fresh release on its next run.
	cmd := exec.Command("open", "-n", target)
	cmd.Env = []string{}
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "GTMUX_VERSION=") {
			cmd.Env = append(cmd.Env, e)
		}
	}
	return cmd.Run()
}

func setAppAutostart(target string) error {
	body, err := plist.Marshal(map[string]any{
		"Label":            menubarBundleID,
		"ProgramArguments": []string{filepath.Join(target, "Contents", "MacOS", "GtmuxBar")},
		"RunAtLoad":        true, "KeepAlive": false,
	}, plist.XMLFormat)
	if err != nil {
		return err
	}
	file := launchAgentPath()
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(file, body, 0o600); err != nil {
		return err
	}
	manager := serviceManager()
	_ = manager.Stop(file)
	return manager.Start(file)
}
