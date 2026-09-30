package hq

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/knowledge"
)

func memoryWriteAllowed() error {
	return memoryWriteAllowedFor(hqpane.Find(), hqSessionRef)
}
func memoryWriteAllowedFor(pane string, sessionRef func(string) (string, string)) error {
	if pane != "" {
		if agent, _ := sessionRef(pane); agent != "" {
			return errors.New(i18n.Tr("Stop HQ before restoring or applying migrated records. Preview and staging are still available.", "请先退出 HQ，再恢复或应用迁移内容；预览和暂存不受影响。"))
		}
	}
	return nil
}
func migrationCmd(args []string) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		i18n.Say("usage: gtmux hq migrate --from ARCHIVE [--stage --include knowledge,local,tools]", "用法：gtmux hq migrate --from 档案 [--stage --include knowledge,local,tools]")
		i18n.Say("  Preview is read-only. Staging needs an explicit selection; plain archives need --allow-plain.", "  预览只读；暂存需明确选择内容，未加密档案需 --allow-plain。")
		i18n.Say("  --passphrase-stdin reads the passphrase from stdin; --include-sensitive opts in to sensitive histories.", "  --passphrase-stdin 从标准输入读取口令；--include-sensitive 明确包含敏感历史。")
		i18n.Say("  --list | --review ID | --apply ID --entries ID,ID --reviewed", "  --list | --review 暂存ID | --apply 暂存ID --entries 条目ID,条目ID --reviewed")
		i18n.Say("  --apply ID --apply-local --expect-local HASH --reviewed replaces personal requirements separately.", "  --apply 暂存ID --apply-local --expect-local 哈希 --reviewed 单独替换个人要求。")
		i18n.Say("  Stop HQ before apply. All results are JSON; knowledge conflicts never overwrite local history.", "  应用前请退出 HQ。结果为 JSON；知识历史冲突不会覆盖本机内容。")
		return 0
	}

	if len(args) > 0 && args[0] == "--list" {
		if len(args) > 2 || len(args) == 2 && args[1] != "--json" {
			i18n.Sae("use migrate --list [--json]", "请用 migrate --list [--json]")
			return 2
		}
		list, err := listMigrations()
		if err != nil {
			i18n.Sae(err.Error(), err.Error())
			return 1
		}
		if err := json.NewEncoder(os.Stdout).Encode(list); err != nil {
			i18n.Sae(err.Error(), err.Error())
			return 1
		}
		return 0
	}

	var from, review, apply, include, ids, expected, expectedSource string
	stage, passStdin, sensitive, plain, applyLocal, reviewed, restorePreview := false, false, false, false, false, false, false
	fail := func(err error) int {
		if errors.Is(err, ErrWrongPassphrase) {
			i18n.Sae("Wrong archive passphrase; no records were changed.", "档案口令不正确，当前内容未修改。")
			return 1
		}
		i18n.Sae("gtmux hq migrate: "+err.Error(), "gtmux hq migrate："+err.Error())
		return 1
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--from", "--review", "--apply", "--include", "--entries", "--expect-local", "--expect-source":
			flag := args[i]
			i++
			if i >= len(args) {
				return fail(fmt.Errorf("%s requires a value", flag))
			}
			switch flag {
			case "--from":
				from = args[i]
			case "--review":
				review = args[i]
			case "--apply":
				apply = args[i]
			case "--include":
				include = args[i]
			case "--entries":
				ids = args[i]
			case "--expect-local":
				expected = args[i]
			case "--expect-source":
				expectedSource = args[i]
			}
		case "--restore-preview":
			restorePreview = true
		case "--stage":
			stage = true
		case "--passphrase-stdin":
			passStdin = true
		case "--include-sensitive":
			sensitive = true
		case "--allow-plain":
			plain = true
		case "--apply-local":
			applyLocal = true
		case "--reviewed":
			reviewed = true
		case "--json":
		default:
			return fail(fmt.Errorf("unknown option %q", args[i]))
		}
	}
	count := 0
	for _, v := range []string{from, review, apply} {
		if v != "" {
			count++
		}
	}
	if count != 1 {
		return fail(errors.New("choose exactly one of --from, --review, --apply"))
	}
	if from != "" && (applyLocal || reviewed || ids != "" || expected != "") || review != "" && (stage || passStdin || sensitive || plain || applyLocal || reviewed || include != "" || ids != "" || expected != "" || expectedSource != "") || apply != "" && (stage || passStdin || sensitive || plain || include != "" || expectedSource != "") || from != "" && !stage && (include != "" || plain) || apply != "" && !applyLocal && expected != "" {
		return fail(errors.New("options do not match preview, staging, review or apply mode; see migrate --help"))
	}
	if restorePreview && (from == "" || stage || sensitive) {
		return fail(errors.New("--restore-preview requires a read-only --from preview"))
	}
	encode := func(v any) int {
		if err := json.NewEncoder(os.Stdout).Encode(v); err != nil {
			return fail(err)
		}
		return 0
	}
	if from != "" {
		pass := ""
		if IsEncryptedArchive(from) {
			var err error
			pass, err = askPassphrase(false, passStdin)
			if err != nil {
				return fail(err)
			}
		}
		a, err := readMemoryArchive(from, pass)
		if err != nil {
			return fail(err)
		}
		if expectedSource != "" && a.digest != expectedSource {
			return fail(errors.New("archive changed since preview; preview it again"))
		}
		if !stage {
			var p migrationPreview
			if restorePreview {
				p, err = previewRestore(a)
			} else {
				p, err = previewMigration(a, sensitive)
			}
			if err != nil {
				return fail(err)
			}
			return encode(p)
		}
		opts := migrationOptions{Sensitive: sensitive, AllowPlain: plain}
		for _, item := range strings.Split(include, ",") {
			switch item {
			case "knowledge":
				opts.Knowledge = true
			case "local":
				opts.Local = true
			case "tools":
				opts.Tools = true
			default:
				return fail(fmt.Errorf("unknown migration selection %q", item))
			}
		}
		m, err := stageMigration(a, opts)
		if err != nil {
			return fail(err)
		}
		diag.Did("act.hq.migrate", m.ID, diag.OK, "staged selected HQ records", "files", len(m.Files), "source", m.Source)
		return encode(m)
	}
	id := review
	if apply != "" {
		id = apply
	}
	m, a, err := readMigration(id)
	if err != nil {
		return fail(err)
	}
	if review != "" {
		p, err := previewMigration(a, true)
		if err != nil {
			return fail(err)
		}
		return encode(p)
	}
	if !reviewed {
		return fail(errors.New("review staged content first, then pass --reviewed"))
	}
	release, lockErr := acquireMigrationApplyLock()
	if lockErr != nil {
		return fail(lockErr)
	}
	defer release()
	if err := memoryWriteAllowed(); err != nil {
		return fail(err)
	}
	if applyLocal && ids != "" || !applyLocal && ids == "" {
		return fail(errors.New("apply knowledge entries or personal requirements separately"))
	}
	r := migrationReceipt{StageID: id, Kind: "knowledge"}
	if applyLocal {
		r, err = applyMigrationLocalLocked(id, a, expected)
	} else {
		data, ok := a.files[knowledge.MigrationLedgerFile]
		if !ok {
			return fail(errors.New("knowledge was not selected"))
		}
		var result knowledge.MigrationResult
		result, err = knowledge.ApplyMigration(data.data, strings.Split(ids, ","), m.Source)
		r.OperationID, r.Imported, r.Skipped, r.Committed, r.Backup = result.OperationID, result.Imported, result.Skipped, result.Committed, result.Backup
	}
	if err != nil {
		r.Error = err.Error()
	}
	if applyLocal {
		outcome := "committed"
		if err != nil {
			outcome = "failed"
			if r.Committed {
				outcome = "committed-view-failed"
			}
		}
		events.AuditKnowledgeOutcome("migrate-local "+id, time.Now().Unix(), r.OperationID, outcome, "local")
	}
	diag.Did("act.hq.migrate", id, diag.Outcome(err), "migration application result", "op_id", r.OperationID, "kind", r.Kind, "imported", r.Imported, "skipped", r.Skipped, "committed", r.Committed)
	if r.OperationID != "" {
		b, _ := json.Marshal(r)
		if _, receiptErr := publishMigrationFile(filepath.Join(migrationDir(), id, "receipt-"+r.OperationID+".json"), b); receiptErr != nil {
			if err == nil {
				r.Error = "application finished but receipt file could not be saved: " + receiptErr.Error()
				err = receiptErr
			}
		}
	}
	rc := encode(r)
	if rc != 0 {
		return rc
	}
	if err != nil {
		return 1
	}
	return 0
}
