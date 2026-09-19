package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/hq"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// doctor's Logs section (openspec change `diagnostics`, design 8): the log store and
// everything else gtmux lets grow, and whether gtmux's files are private. Each row that
// can be flagged has a --fix step.

func logsChecks(now time.Time) []dcheck {
	rows := []dcheck{rowLogStore(now), rowLogRunaway(now), rowLogErrors(now), rowFileModes(), rowGrowth()}
	if r, ok := rowCredentialBackups(); ok {
		rows = append(rows, r)
	}
	return rows
}

func rowLogStore(now time.Time) dcheck {
	label := i18n.Tr("log store", "日志库")
	dir := state.LogsDir()
	lim := diag.LoadLimits()
	size, oldest := diag.Size(dir), diag.OldestDay(dir)
	if oldest == "" {
		return dcheck{stInfo, label, i18n.Tr("empty", "空"),
			i18n.Tr("every gtmux process writes here; read it with `gtmux logs`", "所有 gtmux 进程都写在这里；用 `gtmux logs` 查看")}
	}
	val := fmt.Sprintf("%s · %s", humanBytes(size), i18n.Tr("since ", "最早 ")+oldest)
	note := fmt.Sprintf(i18n.Tr("keeps %d days or %s; `gtmux logs` reads it", "保留 %d 天或 %s；`gtmux logs` 查看"),
		lim.RetainDays, humanBytes(lim.MaxBytes))
	stale := now.AddDate(0, 0, -(lim.RetainDays + 2)).Format("2006-01-02")
	switch {
	case size > lim.MaxBytes:
		return dcheck{stRec, label, val, i18n.Tr("over its cap: cleanup is not keeping up. `gtmux doctor --fix` runs it",
			"超出上限：清理没跟上。`gtmux doctor --fix` 可以立刻清理")}
	case oldest < stale:
		return dcheck{stRec, label, val, i18n.Tr("holds days past its retention: cleanup is not running. `gtmux doctor --fix` runs it",
			"留着超过保留期的日子：清理没在运行。`gtmux doctor --fix` 可以立刻清理")}
	}
	return dcheck{stOK, label, val, note}
}

// storeEntries reads the store's entries from the last `window`, keeping those keep
// accepts.
func storeEntries(now time.Time, window time.Duration, keep func(diag.Entry) bool) []diag.Entry {
	since := now.Add(-window)
	from := since.Format("2006-01-02")
	var out []diag.Entry
	for _, sf := range diag.Files(state.LogsDir()) {
		if sf.Day < from {
			continue
		}
		f, err := os.Open(sf.Path)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, diag.MaxEntry*2), diag.MaxEntry*2)
		for sc.Scan() {
			var e diag.Entry
			if json.Unmarshal(sc.Bytes(), &e) == nil && !e.Time().Before(since) && keep(e) {
				out = append(out, e)
			}
		}
		_ = f.Close()
	}
	return out
}

func rowLogRunaway(now time.Time) dcheck {
	label := i18n.Tr("runaway writer", "刷屏的组件")
	hits := storeEntries(now, 7*24*time.Hour, func(e diag.Entry) bool { return e.Event == "log.runaway" })
	if len(hits) == 0 {
		return dcheck{stOK, label, i18n.Tr("none in 7 days", "7 天内没有"),
			i18n.Tr("a day past 20 MB would name who filled it", "某天超过 20MB 时，会点名是谁写满的")}
	}
	last := hits[len(hits)-1]
	val := fmt.Sprintf("%s %v %v", last.Time().Format("01-02"), last.Attrs["component"], last.Attrs["event"])
	return dcheck{stRec, label, val, i18n.Tr("something looped and filled a day of logs; `gtmux logs --event "+fmt.Sprint(last.Attrs["event"])+"` shows it",
		"有东西在循环，写满了一整天的日志；`gtmux logs --event "+fmt.Sprint(last.Attrs["event"])+"` 可以看到")}
}

func rowLogErrors(now time.Time) dcheck {
	label := i18n.Tr("recent errors", "近期报错")
	errs := storeEntries(now, 24*time.Hour, func(e diag.Entry) bool { return e.Level == "error" })
	if len(errs) == 0 {
		return dcheck{stOK, label, i18n.Tr("none in 24 hours", "24 小时内没有"), ""}
	}
	counts := map[string]int{}
	for _, e := range errs {
		counts[e.Component+" "+e.Event]++
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if counts[keys[i]] != counts[keys[j]] {
			return counts[keys[i]] > counts[keys[j]]
		}
		return keys[i] < keys[j]
	})
	if len(keys) > 3 {
		keys = keys[:3]
	}
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s ×%d", k, counts[k])
	}
	return dcheck{stRec, label, fmt.Sprintf("%d · %s", len(errs), strings.Join(parts, ", ")),
		i18n.Tr("`gtmux logs --since 1d --level error` shows them", "`gtmux logs --since 1d --level error` 可以看到")}
}

func rowFileModes() dcheck {
	label := i18n.Tr("file modes", "文件权限")
	n, example := 0, ""
	for _, root := range []string{state.Dir(), state.ConfigDir()} {
		c, ex := state.WideModes(root)
		if n == 0 {
			example = ex
		}
		n += c
	}
	if n == 0 {
		return dcheck{stOK, label, i18n.Tr("owner only", "只有本人可读"),
			i18n.Tr("nothing gtmux keeps is readable by another account on this Mac", "gtmux 存的东西，这台 Mac 上别的账号都读不到")}
	}
	return dcheck{stRec, label,
		fmt.Sprintf(i18n.Tr("%d readable by other accounts", "%d 个别的账号也能读"), n),
		i18n.Tr("e.g. ", "例如 ") + example + i18n.Tr(". `gtmux doctor --fix` narrows them; serve also does on every sweep",
			"。`gtmux doctor --fix` 可以收紧；serve 每次清理时也会收紧")}
}

// rowGrowth checks the stores besides the log that grow, each against its written bound,
// and reports retired files still on disk.
func rowGrowth() dcheck {
	label := i18n.Tr("other stores", "其他数据")
	base := state.Dir()
	var over []string
	if fi, err := os.Stat(filepath.Join(base, "events.jsonl")); err == nil && fi.Size() > 40<<20 {
		over = append(over, "events.jsonl "+humanBytes(fi.Size()))
	}
	for _, p := range hq.LaunchdCaptures() {
		if fi, err := os.Stat(p); err == nil && fi.Size() > 8<<20 {
			over = append(over, filepath.Base(p)+" "+humanBytes(fi.Size()))
		}
	}
	retired := hq.RetiredPresent()
	if len(over) == 0 && len(retired) == 0 {
		return dcheck{stOK, label, i18n.Tr("within their bounds", "都在上限之内"),
			i18n.Tr("the journal, launchd captures, uploads and caches", "事件流、launchd 日志、上传和缓存")}
	}
	var val []string
	if len(over) > 0 {
		val = append(val, i18n.Tr("over: ", "超出：")+strings.Join(over, ", "))
	}
	if len(retired) > 0 {
		val = append(val, fmt.Sprintf(i18n.Tr("%d retired files", "%d 个已下线的文件"), len(retired)))
	}
	return dcheck{stRec, label, strings.Join(val, " · "),
		i18n.Tr("serve's hygiene sweep trims these; is serve running? `gtmux doctor --fix` does it now",
			"serve 的清理会处理这些；serve 在跑吗？`gtmux doctor --fix` 可以立刻处理")}
}

// credentialBackups are the copies of credential files earlier migrations left in the
// config root. They hold the same tokens as the live files.
func credentialBackups() []string {
	m, _ := filepath.Glob(filepath.Join(state.ConfigDir(), "*.bak-*"))
	return m
}

func rowCredentialBackups() (dcheck, bool) {
	b := credentialBackups()
	if len(b) == 0 {
		return dcheck{}, false
	}
	names := make([]string, len(b))
	for i, p := range b {
		names[i] = filepath.Base(p)
	}
	return dcheck{stRec, i18n.Tr("credential backups", "凭证备份"),
		fmt.Sprintf(i18n.Tr("%d left by earlier migrations", "%d 份是早先迁移留下的"), len(b)),
		strings.Join(names, ", ") + i18n.Tr(". `gtmux doctor --fix` offers to remove them", "。`gtmux doctor --fix` 会询问后删除")}, true
}

// stepHousekeep runs the log cleanup, narrows modes and removes retired files, when any
// of the rows above says one is needed. It asks first, like every step.
func (s *fixState) stepHousekeep() int {
	now := time.Now()
	need := rowLogStore(now).status == stRec || rowFileModes().status == stRec || rowGrowth().status == stRec
	if !need {
		return 0
	}
	if !s.ask(i18n.Tr("Tidy gtmux's files", "整理 gtmux 的文件"),
		i18n.Tr("Runs the log store's retention, removes files nothing reads any more, and makes everything gtmux keeps readable by you only. Each change is recorded in `gtmux logs`.",
			"执行日志库的保留规则，删掉已经没人读的文件，并把 gtmux 存的所有东西改成只有你能读。每一处改动都会记进 `gtmux logs`。")) {
		return 0
	}
	hq.Housekeep()
	i18n.Say("  done.", "  完成。")
	return 1
}

// stepCredentialBackups offers to remove the credential copies earlier migrations left.
func (s *fixState) stepCredentialBackups() int {
	b := credentialBackups()
	if len(b) == 0 {
		return 0
	}
	if !s.ask(fmt.Sprintf(i18n.Tr("Remove %d old credential backups", "删除 %d 份旧的凭证备份"), len(b)),
		i18n.Tr("Earlier migrations left copies of the device roster and push tokens in "+state.ConfigDir()+". The live files are current, so the copies only keep old tokens around.",
			"早先的迁移在 "+state.ConfigDir()+" 里留下了设备名册和推送 token 的副本。现用的文件是最新的，这些副本只是让旧 token 一直留在盘上。")) {
		return 0
	}
	lg := diag.For("cli")
	for _, p := range b {
		if os.Remove(p) == nil {
			lg.Act("act.cleanup", "user", filepath.Base(p), diag.OK, "removed an old credential backup", "reason", "doctor-fix")
		}
	}
	i18n.Say(fmt.Sprintf("  removed %d.", len(b)), fmt.Sprintf("  已删除 %d 份。", len(b)))
	return 1
}
