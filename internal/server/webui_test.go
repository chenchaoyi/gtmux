package server

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"unicode"
)

// The browser-mirror web UI is served at "/" (unauthenticated static page), its
// vendored assets resolve, and the /api/* routes stay token-guarded.
func TestWebUIRouting(t *testing.T) {
	s := New(Config{Addr: "x", Token: "master"}, Deps{})
	h := s.Handler()

	get := func(path string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		return rr
	}

	if rr := get("/"); rr.Code != http.StatusOK {
		t.Errorf("GET / = %d, want 200", rr.Code)
	} else if !strings.Contains(rr.Body.String(), "app.js") {
		t.Errorf("GET / did not serve index.html (no app.js reference)")
	}

	if rr := get("/app.js"); rr.Code != http.StatusOK {
		t.Errorf("GET /app.js = %d, want 200", rr.Code)
	} else if body := rr.Body.String(); !strings.Contains(body, "openPanes") || !strings.Contains(body, "/api/panes") {
		// the all-panes browser (tiered-pane-control): the radar entry + the fetch
		t.Errorf("app.js missing the all-panes browser wiring (openPanes / /api/panes)")
	}
	// index.html carries the browser's entry button + its own section.
	if rr := get("/"); !strings.Contains(rr.Body.String(), "panes-btn") || !strings.Contains(rr.Body.String(), "id=\"panes\"") {
		t.Errorf("index.html missing the all-panes browser markup (panes-btn / #panes)")
	}
	if rr := get("/vendor/xterm.js"); rr.Code != http.StatusOK {
		t.Errorf("GET /vendor/xterm.js = %d, want 200", rr.Code)
	}

	// The web "/" route must NOT shadow the guarded API.
	if rr := get("/api/agents"); rr.Code != http.StatusUnauthorized {
		t.Errorf("GET /api/agents (no token) = %d, want 401", rr.Code)
	}
}

// The browser UI must defeat stale CDN/tunnel edge caching: index.html is served
// uncached with content-hashed asset URLs, and static assets carry no-cache. This
// is what makes `gtmux update` actually show up in the browser.
func TestWebHandlerCacheBusting(t *testing.T) {
	if len(assetTag) != 12 {
		t.Fatalf("assetTag should be a 12-char content hash, got %q", assetTag)
	}

	h := webHandler()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET / = %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "app.js?v="+assetTag) {
		t.Errorf("index.html app.js not cache-busted with assetTag")
	}
	if !strings.Contains(body, "style.css?v="+assetTag) {
		t.Errorf("index.html style.css not cache-busted with assetTag")
	}
	if cc := rr.Header().Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
		t.Errorf("index.html Cache-Control missing no-cache: %q", cc)
	}

	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/app.js", nil))
	if rr2.Code != http.StatusOK {
		t.Fatalf("GET /app.js = %d", rr2.Code)
	}
	if cc := rr2.Header().Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
		t.Errorf("app.js Cache-Control missing no-cache: %q", cc)
	}
}

// The browser mirror is the one surface you hand to SOMEONE ELSE.
//
// A share link goes to a guest, who is the person least likely to read the host's
// language — and the page had drifted into Chinese-only chrome anyway: the gate screen
// and the connection tooltip were bilingual, the rest was not. This pins that every
// visible label has an English half, so the next label added in one language is a red
// build rather than a page a guest cannot operate.
func TestWebChromeIsNotChineseOnly(t *testing.T) {
	js, err := webFS.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	// Every Chinese string a reader sees must be reachable through the language switch:
	// either T(en, zh), or an explicit ZH ? … : … branch, or the GATE table (which shows
	// BOTH languages on purpose — it is the screen you photograph and send on).
	if !bytes.Contains(js, []byte("function T(en, zh)")) {
		t.Fatal("the language helper is gone; every label would follow whatever the markup happens to say")
	}
	html, err := webFS.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	// Markup carries the Chinese half; app.js relabels at boot. A control whose id never
	// appears in app.js is one nobody translates.
	for _, id := range []string{"panes-title", "panes-search", "cmdk-input", "jump", "pane-ro", "wb-snap", "wb-surface", "wb-preset"} {
		if !bytes.Contains(html, []byte(`id="`+id+`"`)) {
			continue // the control was removed; nothing to translate
		}
		if !bytes.Contains(js, []byte("'"+id+"'")) {
			t.Errorf("%s is in the page but never relabelled — an English reader sees the Chinese markup", id)
		}
	}
	if n := chineseWithoutASwitch(js); len(n) > 0 {
		t.Errorf("%d runtime strings are Chinese-only:\n%s", len(n), strings.Join(n, "\n"))
	}
}

// chineseWithoutASwitch finds Chinese string literals in app.js that no language switch
// reaches, and is the reason this is a test rather than a habit.
//
// The page was translated by hand once (2026-09-07) and 24 strings were missed — every
// one of them deeper in the file than the visible chrome someone thinks to look at: the
// composer's errors, the approval bar, the workbench's tile buttons, the ⌘K empty state.
// A reader who does not read Chinese meets those only when something goes wrong, which is
// the worst moment to meet them.
//
// Deliberately crude: it looks for a switch (T(...), a ZH ternary, or one of the two
// tables that carry both languages) within a few lines. A new string in one language is a
// red build; where the check is wrong, the fix is to route the string through T().
func chineseWithoutASwitch(js []byte) []string {
	lines := strings.Split(string(js), "\n")
	hasSwitch := func(from, to int) bool {
		for i := from; i < to && i < len(lines); i++ {
			if i < 0 {
				continue
			}
			l := lines[i]
			// A bare "ZH" counts: the ternary is often split across lines, and ZH is
			// this file's only language flag, so its presence nearby IS the switch.
			if strings.Contains(l, "T(") || strings.Contains(l, "ZH") ||
				strings.Contains(l, "zh:") || strings.Contains(l, "GATE") || strings.Contains(l, "CONN_TITLE") {
				return true
			}
		}
		return false
	}
	var out []string
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "//") || strings.HasPrefix(t, "*") {
			continue
		}
		if !hasCJKInLiteral(l) || hasSwitch(i-3, i+4) {
			continue
		}
		out = append(out, fmt.Sprintf("  app.js:%d  %s", i+1, firstN(t, 90)))
	}
	return out
}

// hasCJKInLiteral reports whether a line carries CJK inside a quoted string (not a
// comment, which this caller has already excluded).
func hasCJKInLiteral(line string) bool {
	inStr := rune(0)
	var esc bool
	for _, r := range line {
		switch {
		case esc:
			esc = false
		case r == '\\':
			esc = true
		case inStr != 0 && r == inStr:
			inStr = 0
		case inStr == 0 && (r == '\'' || r == '"' || r == '`'):
			inStr = r
		case inStr != 0 && unicode.Is(unicode.Han, r):
			return true
		}
	}
	return false
}

func firstN(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// Every list that groups agents by status must have a bucket for every section the page
// renders, and this is a test because the workbench rail shipped without one.
//
// `ORDER` names five sections; the rail's buckets named four, leaving `errored` with no
// array. The render loop then read `.length` off nothing on that pass and threw, so the
// rail drew "needs you" and stopped — working, idle and running panes were missing from
// the session tree — and the same throw reached the poll's catch, which parked the
// connection dot on "reconnecting" while the board beside it kept updating fine. Nothing
// logged, and the one section that did render is the one a screenshot tends to show.
func TestWebStatusBucketsCoverEverySection(t *testing.T) {
	js, err := webFS.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	order := regexp.MustCompile(`var ORDER = \[([^\]]*)\]`).FindSubmatch(js)
	if order == nil {
		t.Fatal("app.js no longer declares ORDER; the sections a reader sees are now unpinned")
	}
	var sections []string
	for _, s := range regexp.MustCompile(`'([a-z]+)'`).FindAllSubmatch(order[1], -1) {
		sections = append(sections, string(s[1]))
	}
	if len(sections) < 2 {
		t.Fatalf("parsed %d sections out of ORDER; the pattern no longer matches", len(sections))
	}
	buckets := regexp.MustCompile(`var by = \{[^}]*\}`).FindAll(js, -1)
	if len(buckets) == 0 {
		t.Fatal("no status buckets found in app.js; this test no longer checks anything")
	}
	for _, b := range buckets {
		for _, st := range sections {
			if !bytes.Contains(b, []byte(st+": [")) {
				t.Errorf("a status bucket has no %q array, so that section throws when it is rendered:\n%s", st, b)
			}
		}
	}
}

// Every word the markup shows, in its text, title or placeholder, is relabelled by app.js
// for the reader's language. TestWebChromeIsNotChineseOnly checks the Chinese side; this
// is the other direction, which it could not see: the focus bar's tooltips, the
// appearance panel (Font, Size, Match terminal, System) and the workbench rail (Sessions,
// search) stayed English on a Chinese page (%12, 2026-10-06). An element counts as
// relabelled when app.js names its id, one of its classes as a selector, or (the mode
// switch) its data-mode buttons. Brand and font names are not translated.
func TestWebMarkupIsRelabelledForTheReader(t *testing.T) {
	js, err := webFS.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := webFS.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	// Only the relabelling code counts: an id app.js merely binds a click to is not
	// translated (the tooltips above all had ids app.js used for behaviour).
	from, to := bytes.Index(js, []byte("var CHROME = [")), bytes.Index(js, []byte("// ---- helpers"))
	if from < 0 || to < from {
		t.Fatal("cannot find the CHROME table and labelChrome in app.js")
	}
	js = js[from:to]
	html := regexp.MustCompile(`(?s)<!--.*?-->|<script.*?</script>`).ReplaceAllString(string(raw), "")
	brand := map[string]bool{"gtmux": true}
	words := regexp.MustCompile(`[A-Za-z]{3,}|\p{Han}`)
	attr := regexp.MustCompile(`(?:title|placeholder|aria-label)="([^"]*)"`)
	idOf := regexp.MustCompile(`\bid="([^"]+)"`)
	classOf := regexp.MustCompile(`\bclass="([^"]+)"`)
	valueOf := regexp.MustCompile(`\bvalue="([^"]+)"`)
	for _, m := range regexp.MustCompile(`<(\w+)([^>]*)>([^<]*)`).FindAllStringSubmatch(html, -1) {
		tag, attrs, text := m[1], m[2], strings.TrimSpace(m[3])
		if tag == "title" || brand[text] {
			continue
		}
		if v := valueOf.FindStringSubmatch(attrs); tag == "option" && v != nil && v[1] == text {
			continue // a font option: the font's own name
		}
		shown := []string{text}
		for _, a := range attr.FindAllStringSubmatch(attrs, -1) {
			shown = append(shown, a[1])
		}
		needs := false
		for _, s := range shown {
			needs = needs || words.MatchString(s)
		}
		if !needs {
			continue
		}
		relabelled := false
		if id := idOf.FindStringSubmatch(attrs); id != nil {
			relabelled = bytes.Contains(js, []byte("'"+id[1]+"'"))
		}
		if c := classOf.FindStringSubmatch(attrs); c != nil && !relabelled {
			for _, cls := range strings.Fields(c[1]) {
				relabelled = relabelled || bytes.Contains(js, []byte("."+cls+"'"))
			}
		}
		if strings.Contains(attrs, "data-mode=") {
			relabelled = bytes.Contains(js, []byte("'#mode button'"))
		}
		if !relabelled {
			t.Errorf("<%s%s> shows %q, and app.js never relabels it for the reader's language", tag, attrs, shown)
		}
	}
	if !bytes.Contains(js, []byte("document.documentElement.lang")) {
		t.Error("the page's lang never follows the reader's language")
	}
}

// The status words the radar, the rail and ⌘K build at runtime follow the reader too:
// LABEL was a plain English table, so a Chinese page showed NEEDS YOU / WORKING / IDLE
// over every section (%12, 2026-10-06). Each state's word must go through T, and LABEL
// must be built after ZH is known (T at the top of the file would always read English).
func TestWebStatusWordsFollowTheReader(t *testing.T) {
	js, err := webFS.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^\s*LABEL = \{([^\n]*)\};`).FindSubmatch(js)
	if m == nil {
		t.Fatal("LABEL is never built")
	}
	for _, st := range []string{"waiting", "errored", "working", "idle", "running"} {
		if !regexp.MustCompile(st + `: T\('[^']+', '[^']+'\)`).Match(m[1]) {
			t.Errorf("%s's word does not go through T: %s", st, m[1])
		}
	}
	if bytes.Index(js, m[0]) < bytes.Index(js, []byte("var ZH = ")) {
		t.Error("LABEL is built before ZH is set, so T would always answer in English")
	}
}
