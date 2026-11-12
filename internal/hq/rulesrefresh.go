package hq

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// readManagedRules is bounded and never follows a link/special file or repairs
// the source permissions. Existing private IO also checks the owner and parents.
func readManagedRules() ([]byte, os.FileInfo, error) {
	f, err := state.OpenPrivateRead(hqInstructionsPath())
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	if info.Mode().Perm()&0200 == 0 {
		return nil, nil, errors.New(i18n.Tr("the managed HQ rules are read-only", "HQ 规则文件只读，未更新"))
	}
	const limit = 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, nil, err
	}
	if len(body) > limit {
		return nil, nil, errors.New(i18n.Tr("managed HQ rules exceed the 1 MiB refresh limit", "HQ 规则超过 1 MiB，未更新"))
	}
	version, lang := parsePlaybookVersion(string(body)), parsePlaybookLang(string(body))
	if version < 1 || (lang != "en" && lang != "zh") || !strings.HasPrefix(string(body), playbookMarker(version, lang)+"\n") {
		return nil, nil, errors.New(i18n.Tr("refresh requires existing managed HQ rules; missing or legacy rules need normal HQ setup", "只支持已有的受管规则；缺失或旧式档案请通过正常 HQ 启动流程处理"))
	}
	return body, info, nil
}

func refreshHQRulesCmd() int {
	fail := func(err error) int {
		i18n.Sae("gtmux hq --refresh-rules: "+err.Error(), "gtmux hq --refresh-rules："+err.Error())
		return 1
	}
	release, err := acquireMigrationApplyLock()
	if err != nil {
		return fail(err)
	}
	defer release()
	body, observed, err := readManagedRules()
	if err != nil {
		return fail(err)
	}
	publish := func(stem string, prior, replacement []byte) (string, error) {
		backup, err := writeHQBackup(hqInstructionsPath(), stem, prior)
		if err != nil {
			return "", err
		}
		current, info, err := readManagedRules()
		if err != nil {
			return backup, err
		}
		if !os.SameFile(observed, info) || !bytes.Equal(current, prior) {
			return backup, errors.New(i18n.Tr("HQ rules changed during refresh; review the current file before retrying", "规则文件在刷新期间发生变化，请核对后重试"))
		}
		// Cooperating writers hold the same lock. External editors do not; this
		// final observation is a conflict guard, not an external-editor CAS.
		if err := state.WritePrivateFile(hqInstructionsPath(), replacement); err != nil {
			return backup, err
		}
		return backup, nil
	}
	var result seedResult
	if err := upgradePlaybookBody(&result, "", body, publish); err != nil {
		return fail(err)
	}
	if result.Upgraded {
		printSeedNotice(result)
	} else {
		i18n.Say("HQ rules are already current. No session was changed.", "HQ 规则已是当前版本，没有操作会话。")
	}
	return 0
}
