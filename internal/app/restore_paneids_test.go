package app

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

func idShape(name, raw string, count int) windowShape {
	return windowShape{Session: name, Index: "0", Panes: count, Layout: normalizeLayout(raw), PaneIDs: layoutPaneIDs(raw)}
}

func TestRestorePaneIDReceiptFollowsLayoutPositionNotNumericIDOrder(t *testing.T) {
	saved := idShape("example", "37e9,189x48,0,0{94x48,0,0,44,94x48,95,0,12}", 2)
	live := idShape("example", "b81c,189x48,0,0{94x48,0,0,7,94x48,95,0,81}", 2)
	if !reflect.DeepEqual(saved.PaneIDs, []string{"%44", "%12"}) {
		t.Fatal(saved.PaneIDs)
	}
	out := renderRestorePaneIDs([]windowShape{saved}, []windowShape{live}, 80)
	for _, want := range []string{"example:0", "%44 → %7", "%12 → %81"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q: %s", want, out)
		}
	}
	if strings.Contains(out, "%12 → %7") {
		t.Fatal("receipt sorted IDs instead of matching spatial positions")
	}
}

func TestRestoreReconciliationActuallyPrintsPaneIDReceipt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("RESTORE_TEST_WINDOWS", "example\t0\t2\tb81c,189x48,0,0{94x48,0,0,7,94x48,95,0,81}\n")
	useDoctorTmuxStub(t, "[ \"$1\" = -u ] || exit 8\n[ \"$2\" = list-windows ] || exit 9\nprintf '%s' \"$RESTORE_TEST_WINDOWS\"\n")
	save := writeShapeSave(t,
		shapeWin("example", "0", "37e9,189x48,0,0{94x48,0,0,44,94x48,95,0,12}"),
		shapePane("example", "0", "0"), shapePane("example", "0", "1"))
	out := captureStdout(t, func() { reportLayoutDrift(save) })
	for _, want := range []string{"example:0", "%44 → %7", "%12 → %81"} {
		if !strings.Contains(out, want) {
			t.Fatalf("restore path omitted %q: %s", want, out)
		}
	}
	t.Logf("Synthetic ID receipt:\n%s", out)
}

func TestRestorePaneIDReceiptDoesNotGuessAcrossLayoutDrift(t *testing.T) {
	saved := idShape("example", "37e9,189x48,0,0{94x48,0,0,44,94x48,95,0,12}", 2)
	for _, live := range []windowShape{
		idShape("example", "b81c,189x48,0,0[189x24,0,0,7,189x23,0,25,81]", 2),
		idShape("example", "b81c,189x48,0,0{62x48,0,0,7,62x48,63,0,81,63x48,126,0,90}", 3),
	} {
		if correspondingPaneIDs(saved, live) {
			t.Fatal("different layout was treated as certain correspondence")
		}
		out := renderRestorePaneIDs([]windowShape{saved}, []windowShape{live}, 80)
		for _, id := range append(saved.PaneIDs, live.PaneIDs...) {
			if !strings.Contains(out, id) {
				t.Fatalf("receipt omitted %s: %s", id, out)
			}
		}
		if strings.Contains(out, "%44 → %7") {
			t.Fatal("guessed an individual match")
		}
	}
	// A single pane has no ambiguity even when the terminal was resized.
	if !correspondingPaneIDs(idShape("one", "1234,80x24,0,0,44", 1), idShape("one", "4321,120x40,0,0,7", 1)) {
		t.Fatal("single-pane window lost its unambiguous correspondence")
	}
}

func TestRestorePaneIDsHandleLegacyMissingAndDuplicateEvidence(t *testing.T) {
	if layoutPaneIDs("80x24,0,0,1") != nil {
		t.Fatal("accepted an unrecognized layout field")
	}
	if layoutPaneIDs("1234,80x24,0,0{40x24,0,0,1,39x24,41,0,1}") != nil {
		t.Fatal("accepted duplicate IDs")
	}
	saved := windowShape{Session: "legacy", Index: "0", Panes: 1}
	live := idShape("legacy", "1234,80x24,0,0,7", 1)
	if correspondingPaneIDs(saved, live) {
		t.Fatal("invented a saved ID")
	}
	out := renderRestorePaneIDs([]windowShape{saved}, []windowShape{live}, 80)
	if !strings.Contains(out, "%7") {
		t.Fatal("legacy backup hid the current ID")
	}
	missing := idShape("missing", "1234,80x24,0,0,44", 1)
	out = renderRestorePaneIDs([]windowShape{missing}, []windowShape{live}, 80)
	if !strings.Contains(out, "%44") || strings.Contains(out, "%7") {
		t.Fatal("missing window borrowed an unrelated live pane")
	}
	if renderRestorePaneIDs([]windowShape{missing}, nil, 80) != "" {
		t.Fatal("failed live read was treated as a missing window")
	}
}

func TestRestorePaneIDReceiptWrapsWithoutDroppingIDsOrTerminalControls(t *testing.T) {
	saved := idShape("example\n\x1b[31m\u009b", "37e9,189x48,0,0{94x48,0,0,44,94x48,95,0,12}", 2)
	live := saved
	live.PaneIDs = []string{"%7", "%81"}
	for _, lang := range []string{"en", "zh"} {
		old := i18n.Lang()
		i18n.SetLang(lang)
		out := renderRestorePaneIDs([]windowShape{saved}, []windowShape{live}, 20)
		i18n.SetLang(old)
		for _, id := range []string{"%44", "%12", "%7", "%81"} {
			if !strings.Contains(out, id) {
				t.Fatal(id, out)
			}
		}
		if strings.ContainsAny(out, "\x1b\u009b\r\t") {
			t.Fatal("terminal control characters escaped sanitation")
		}
		for _, line := range strings.Split(out, "\n") {
			if i18n.DispWidth(line) > 20 {
				t.Fatal("overflow", line)
			}
		}
	}
}

func TestRestoreWarningExplainsCountDifferenceAndGivesSafeNextStep(t *testing.T) {
	old := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(old) })
	for _, lang := range []string{"en", "zh"} {
		i18n.SetLang(lang)
		saved := idShape("example", "1234,80x24,0,0,44", 2)
		live := idShape("example", "4321,80x24,0,0,7", 3)
		out := renderLayoutDrift(19, layoutDrift([]windowShape{saved}, []windowShape{live}), 80)
		for _, want := range []string{i18n.Tr("saved 2, now 3", "备份 2 个，现在 3 个"), i18n.Tr("1 extra", "多了 1 个"), i18n.Tr("before closing", "确认不需要后再关闭"), "gtmux focus %7", restoreTraceHint} {
			if !strings.Contains(strings.Join(strings.Fields(out), ""), strings.Join(strings.Fields(want), "")) {
				t.Fatalf("missing %q: %s", want, out)
			}
		}
		if strings.Contains(out, "kill-pane") || strings.Contains(out, "default arrangement") {
			t.Fatal("unsafe or unsupported advice", out)
		}
		t.Logf("%s synthetic warning:\n%s", lang, out)
	}
}

func TestRestoreSaveAgeNoteUsesNaturalChineseUnits(t *testing.T) {
	old := i18n.Lang()
	i18n.SetLang("zh")
	t.Cleanup(func() { i18n.SetLang(old) })
	save := writeSaveWithAge(t, 6*time.Minute)
	out := saveAgeNote(save, time.Now())
	for _, want := range []string{"本次使用", "布局备份", "6 分钟前保存", "保存之后的改动不会恢复"} {
		if !strings.Contains(out, want) {
			t.Fatal(want, out)
		}
	}
	if strings.Contains(out, "6m") || strings.Contains(out, "不在里面") {
		t.Fatal(out)
	}
}
