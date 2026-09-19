package app

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/hq"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// `gtmux doctor --bundle` packs what someone needs to see why gtmux misbehaved on this
// Mac, and nothing else (openspec change `diagnostics`, design 9): the log store, the
// status files, the launchd captures' tails, the doctor report and the versions. The
// journal holds prompt heads and wake payloads, so it goes in only with --with-events.
// Nothing is uploaded; the person decides where the file goes.
//
// Every text in the archive passes the log store's redaction again, with every
// credential gtmux keeps registered first: the store's entries were redacted when they
// were written, but a launchd capture and a status file were not written through it.

const captureTail = 256 << 10 // the last 256 KB of each launchd capture

type bundleFile struct {
	name string
	data []byte
}

func doctorBundle(path string, withEvents bool, secs []dsection) int {
	if path == "" {
		path = "gtmux-diagnostics-" + time.Now().Format("20060102-1504") + ".tgz"
	}
	if _, err := os.Stat(path); err == nil {
		i18n.Sae("gtmux doctor --bundle: "+path+" already exists; name another file",
			"gtmux doctor --bundle："+path+" 已存在，换一个文件名")
		return 1
	}
	registerKeptSecrets()
	files := bundleFiles(withEvents, secs)
	err := writeBundle(path, files)
	diag.Did("act.doctor.bundle", filepath.Base(path), diag.Outcome(err), "packed a diagnostics bundle",
		"files", len(files), "withEvents", withEvents, "error", err)
	if err != nil {
		i18n.Sae("gtmux doctor --bundle: "+err.Error(), "gtmux doctor --bundle："+err.Error())
		return 1
	}
	i18n.Say("Packed "+path+":", "已打包 "+path+"：")
	for _, f := range files {
		fmt.Printf("  %-44s %s\n", f.name, humanBytes(int64(len(f.data))))
	}
	if !withEvents {
		i18n.Say("Not included: the event journal (it holds prompt heads; add --with-events to include it) and anything you uploaded.",
			"没有包含：事件流（里面有提示词开头，要带上就加 --with-events），以及你上传过的文件。")
	}
	i18n.Say("Tokens and pairing codes are replaced. Nothing was sent anywhere.",
		"token 和配对码都已替换掉。没有发送到任何地方。")
	return 0
}

// registerKeptSecrets registers every credential gtmux keeps on this Mac with the
// redactor, so the bundle's second pass knows them: the serve token, the Direct secret,
// and every credential-named value in the device roster, the push tokens and the config.
func registerKeptSecrets() {
	if b, err := os.ReadFile(filepath.Join(state.ConfigDir(), "serve-token")); err == nil {
		diag.RegisterSecret(strings.TrimSpace(string(b)))
	}
	_, secret := readSelfTunnelConf()
	diag.RegisterSecret(secret)
	diag.RegisterSecret(os.Getenv("GTMUX_RELAY_TOKEN"))
	for _, name := range []string{"devices.json", "push-tokens.json", "config.json", "share.json"} {
		b, err := os.ReadFile(filepath.Join(state.ConfigDir(), name))
		if err != nil {
			continue
		}
		var v any
		if json.Unmarshal(b, &v) == nil {
			registerCredentialValues(v, "")
		}
	}
}

func registerCredentialValues(v any, key string) {
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			registerCredentialValues(x, k)
		}
	case []any:
		for _, x := range t {
			registerCredentialValues(x, key)
		}
	case string:
		if diag.IsCredentialKey(key) {
			diag.RegisterSecret(t)
		}
	}
}

// bundleFiles gathers the archive's contents, each already redacted.
func bundleFiles(withEvents bool, secs []dsection) []bundleFile {
	var out []bundleFile
	add := func(name string, data []byte) {
		out = append(out, bundleFile{name, redactLines(data)})
	}
	for _, sf := range diag.Files(state.LogsDir()) {
		if b, err := os.ReadFile(sf.Path); err == nil {
			add("logs/"+filepath.Base(sf.Path), b)
		}
	}
	if ents, err := os.ReadDir(state.StatusDir()); err == nil {
		for _, e := range ents {
			if strings.HasSuffix(e.Name(), ".json") {
				if b, err := os.ReadFile(filepath.Join(state.StatusDir(), e.Name())); err == nil {
					add("status/"+e.Name(), b)
				}
			}
		}
	}
	for _, p := range hq.LaunchdCaptures() {
		if b := fileTail(p, captureTail); len(b) > 0 {
			add("captures/"+filepath.Base(p), b)
		}
	}
	add("doctor.txt", []byte(doctorReportText(secs)))
	add("versions.txt", []byte(versionsText()))
	if withEvents {
		if b, err := os.ReadFile(filepath.Join(state.Dir(), "events.jsonl")); err == nil {
			add("events.jsonl", b)
		}
	}
	return out
}

// redactLines runs the log store's redaction over a text line by line.
func redactLines(b []byte) []byte {
	var out bytes.Buffer
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 64<<10), 4<<20)
	for sc.Scan() {
		out.WriteString(diag.Redact(sc.Text()))
		out.WriteByte('\n')
	}
	return out.Bytes()
}

// fileTail returns the last n bytes of a file, starting on a whole line.
func fileTail(path string, n int64) []byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil
	}
	if fi.Size() > n {
		if _, err := f.Seek(-n, io.SeekEnd); err != nil {
			return nil
		}
	}
	b, _ := io.ReadAll(f)
	if fi.Size() > n {
		if i := bytes.IndexByte(b, '\n'); i >= 0 {
			b = b[i+1:]
		}
	}
	return b
}

// doctorReportText is doctor's report as plain text: the same sections and rows the
// screen shows, without color, for a reader who is not at this Mac.
func doctorReportText(secs []dsection) string {
	mark := map[int]string{stOK: "ok", stRec: "to improve", stMiss: "blocking", stInfo: "info"}
	var b strings.Builder
	for _, s := range secs {
		fmt.Fprintf(&b, "%s\n", s.title)
		for _, r := range s.rows {
			fmt.Fprintf(&b, "  [%s] %s: %s", mark[r.status], r.label, r.value)
			if r.note != "" {
				fmt.Fprintf(&b, " (%s)", r.note)
			}
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// versionsText names what was running: gtmux, the menu bar app, macOS, tmux.
func versionsText() string {
	lines := []string{
		"gtmux " + Version,
		"app " + orUnknown(installedAppVersion()),
		"os " + runtime.GOOS + "/" + runtime.GOARCH,
	}
	if out, err := exec.Command("sw_vers", "-productVersion").Output(); err == nil {
		lines = append(lines, "macOS "+strings.TrimSpace(string(out)))
	}
	if tmux.Bin != "" {
		if out, err := exec.Command(tmux.Bin, "-V").Output(); err == nil {
			lines = append(lines, strings.TrimSpace(string(out)))
		}
	}
	lines = append(lines, "packed "+time.Now().Format(time.RFC3339))
	return strings.Join(lines, "\n") + "\n"
}

func orUnknown(s string) string {
	if s == "" {
		return "not installed"
	}
	return s
}

// writeBundle writes the files as a gzipped tar, readable by the owner only, through a
// temporary file so a failed write leaves nothing behind.
func writeBundle(path string, files []bundleFile) error {
	sort.SliceStable(files, func(i, j int) bool { return files[i].name < files[j].name })
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".gtmux-bundle-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	zw := gzip.NewWriter(tmp)
	tw := tar.NewWriter(zw)
	now := time.Now()
	for _, f := range files {
		hdr := &tar.Header{Name: "gtmux-diagnostics/" + f.name, Mode: 0o600, Size: int64(len(f.data)), ModTime: now}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := tw.Write(f.data); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
