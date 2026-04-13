package main

import (
	"fmt"
	"strings"
	"testing"
)

type fakeTTY struct {
	runes    []rune
	pos      int
	buffered bool
}

func (f *fakeTTY) ReadRune() (rune, error) {
	if f.pos >= len(f.runes) {
		return 0, fmt.Errorf("eof")
	}
	r := f.runes[f.pos]
	f.pos++
	return r, nil
}

func (f *fakeTTY) Buffered() bool {
	return f.buffered
}

func TestReadLinesPreservesLeadingAndTrailingSpaces(t *testing.T) {
	lines, err := readLines(strings.NewReader("  first  \nsecond \n"))
	if err != nil {
		t.Fatalf("readLines returned error: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("readLines returned %d lines, want 2", len(lines))
	}
	if lines[0] != "  first  " {
		t.Fatalf("first line = %q, want %q", lines[0], "  first  ")
	}
	if lines[1] != "second " {
		t.Fatalf("second line = %q, want %q", lines[1], "second ")
	}
}

func TestReadLinesPreservesBlankLineInsideInput(t *testing.T) {
	lines, err := readLines(strings.NewReader("alpha\n\nbeta\n"))
	if err != nil {
		t.Fatalf("readLines returned error: %v", err)
	}
	if len(lines) != 3 {
		t.Fatalf("readLines returned %d lines, want 3", len(lines))
	}
	if lines[1] != "" {
		t.Fatalf("middle line = %q, want blank line", lines[1])
	}
}

func TestBuildItemsSeparatesResultAndDisplay(t *testing.T) {
	items := buildItems([]string{"id-1\tAlpha", "id-2"}, "\t")
	if got, want := items[0].result, "id-1"; got != want {
		t.Fatalf("result = %q, want %q", got, want)
	}
	if got, want := items[0].display, "Alpha"; got != want {
		t.Fatalf("display = %q, want %q", got, want)
	}
	if got, want := items[1].result, "id-2"; got != want {
		t.Fatalf("fallback result = %q, want %q", got, want)
	}
	if got, want := items[1].display, ""; got != want {
		t.Fatalf("fallback display = %q, want empty display", got)
	}
}

func TestFilterItemsKeepsOriginalIndexes(t *testing.T) {
	items := buildItems([]string{"id-1\tAlpha", "id-2\tBeta", "id-3\tGamma"}, "\t")
	filtered := filterItems(items, "ma", strings.Index)
	if len(filtered) != 1 {
		t.Fatalf("filtered len = %d, want 1", len(filtered))
	}
	if got, want := filtered[0].index, 2; got != want {
		t.Fatalf("filtered index = %d, want %d", got, want)
	}
	if got, want := filtered[0].result, "id-3"; got != want {
		t.Fatalf("filtered result = %q, want %q", got, want)
	}
}

func TestResolveResultsReturnsVisibleSelectedResultsInViewOrder(t *testing.T) {
	items := buildItems([]string{
		"id-1\tAlpha",
		"id-2\tBeta",
		"id-3\tGamma",
		"id-4\tDelta",
	}, "\t")
	view := filterItems(items, "a", strings.Index)
	selected := make([]bool, len(items))
	selected[3] = true
	selected[0] = true

	results := resolveResults(view, selected, 0, true)
	if got, want := len(results), 2; got != want {
		t.Fatalf("results len = %d, want %d", got, want)
	}
	if got, want := results[0], "id-1"; got != want {
		t.Fatalf("results[0] = %q, want %q", got, want)
	}
	if got, want := results[1], "id-4"; got != want {
		t.Fatalf("results[1] = %q, want %q", got, want)
	}
}

func TestResolveResultsReturnsResultForSingleSelection(t *testing.T) {
	items := buildItems([]string{"id-1\tAlpha", "id-2\tBeta"}, "\t")
	view := filterItems(items, "et", strings.Index)

	results := resolveResults(view, nil, 0, false)
	if got, want := len(results), 1; got != want {
		t.Fatalf("results len = %d, want %d", got, want)
	}
	if got, want := results[0], "id-2"; got != want {
		t.Fatalf("result = %q, want %q", got, want)
	}
}

func newTestUIState(lines []string, queryEnabled, multi bool) *uiState {
	state := newUIState(buildItems(lines, "\t"), 0, queryEnabled, multi, strings.Index)
	state.maxVisible = 2
	state.refreshView()
	return state
}

func TestUIStateMoveDownScrollsAndMarksDirty(t *testing.T) {
	state := newTestUIState([]string{
		"id-1\tAlpha",
		"id-2\tBeta",
		"id-3\tGamma",
	}, false, false)

	for i := range state.dirty {
		state.dirty[i] = false
	}
	state.handleKey(0x0E)
	state.handleKey(0x0E)

	if got, want := state.row, 2; got != want {
		t.Fatalf("row = %d, want %d", got, want)
	}
	if got, want := state.off, 1; got != want {
		t.Fatalf("off = %d, want %d", got, want)
	}
	for i, dirty := range state.dirty {
		if !dirty {
			t.Fatalf("dirty[%d] = false, want true after scroll", i)
		}
	}
}

func TestUIStateBackspaceResetsToTop(t *testing.T) {
	state := newTestUIState([]string{
		"id-1\tAlpha",
		"id-2\tBeta",
		"id-3\tGamma",
	}, true, false)

	state.handleKey('a')
	state.refreshView()
	state.row = 1
	state.off = 1
	state.handleKey(0x08)

	if got, want := state.queryString(), ""; got != want {
		t.Fatalf("query = %q, want empty", got)
	}
	if got, want := state.row, 0; got != want {
		t.Fatalf("row = %d, want %d", got, want)
	}
	if got, want := state.off, 0; got != want {
		t.Fatalf("off = %d, want %d", got, want)
	}
	for i, dirty := range state.dirty {
		if !dirty {
			t.Fatalf("dirty[%d] = false, want true after clearing query", i)
		}
	}
}

func TestUIStateToggleSelectionUsesVisibleItemIndex(t *testing.T) {
	state := newTestUIState([]string{
		"id-1\tAlpha",
		"id-2\tBeta",
		"id-3\tGamma",
	}, true, true)

	state.handleKey('m')
	state.refreshView()
	if got, want := len(state.view), 1; got != want {
		t.Fatalf("view len = %d, want %d", got, want)
	}

	state.handleKey(0x16)

	if !state.selected[2] {
		t.Fatalf("selected[2] = false, want true")
	}
	if state.selected[0] || state.selected[1] {
		t.Fatalf("unexpected selection state: %+v", state.selected)
	}
}

func TestUIStateResultsFollowFilteredViewOrder(t *testing.T) {
	state := newTestUIState([]string{
		"id-1\tAlpha",
		"id-2\tBeta",
		"id-3\tGamma",
		"id-4\tDelta",
	}, true, true)

	state.handleKey('a')
	state.refreshView()
	state.selected[3] = true
	state.selected[0] = true

	results := state.results()
	if got, want := len(results), 2; got != want {
		t.Fatalf("results len = %d, want %d", got, want)
	}
	if got, want := results[0], "id-1"; got != want {
		t.Fatalf("results[0] = %q, want %q", got, want)
	}
	if got, want := results[1], "id-4"; got != want {
		t.Fatalf("results[1] = %q, want %q", got, want)
	}
}

func TestBuildRenderStateIncludesQueryCursorAndVisibleLines(t *testing.T) {
	state := newTestUIState([]string{
		"id-1\tAlpha",
		"id-2\tBeta\tTab",
		"id-3\tGamma",
	}, true, true)
	state.handleKey('a')
	state.refreshView()
	state.selected[0] = true

	render := buildRenderState(state, 20, 2, func(s string, w int, _ string) string {
		if len([]rune(s)) > w {
			return string([]rune(s)[:w])
		}
		return s
	})

	if got, want := render.queryPrompt, "> a"; got != want {
		t.Fatalf("queryPrompt = %q, want %q", got, want)
	}
	if got, want := render.cursorCol, 3; got != want {
		t.Fatalf("cursorCol = %d, want %d", got, want)
	}
	if got, want := render.cursorUp, 3; got != want {
		t.Fatalf("cursorUp = %d, want %d", got, want)
	}
	if !render.clearBelow {
		t.Fatalf("clearBelow = false, want true")
	}
	if got, want := len(render.lines), 2; got != want {
		t.Fatalf("lines len = %d, want %d", got, want)
	}
	if got, want := render.lines[0].text, "Alpha"; got != want {
		t.Fatalf("lines[0].text = %q, want %q", got, want)
	}
	if !render.lines[0].selected {
		t.Fatalf("lines[0].selected = false, want true")
	}
	if !render.lines[0].current {
		t.Fatalf("lines[0].current = false, want true")
	}
	if got, want := render.lines[1].text, "Beta    Tab"; got != want {
		t.Fatalf("lines[1].text = %q, want %q", got, want)
	}
}

func TestBuildRenderStateWithoutQueryHidesCursor(t *testing.T) {
	state := newTestUIState([]string{
		"id-1\tAlpha",
		"id-2\tBeta",
	}, false, false)

	render := buildRenderState(state, 20, 2, func(s string, _ int, _ string) string { return s })

	if render.showCursor {
		t.Fatalf("showCursor = true, want false")
	}
	if got, want := render.queryPrompt, ""; got != want {
		t.Fatalf("queryPrompt = %q, want empty", got)
	}
	if got, want := render.cursorUp, 2; got != want {
		t.Fatalf("cursorUp = %d, want %d", got, want)
	}
}

func TestNormalizeKeyMapsJKWhenQueryDisabled(t *testing.T) {
	if got, want := normalizeKey('j', false), rune(0x0E); got != want {
		t.Fatalf("normalizeKey('j') = %d, want %d", got, want)
	}
	if got, want := normalizeKey('k', false), rune(0x10); got != want {
		t.Fatalf("normalizeKey('k') = %d, want %d", got, want)
	}
	if got, want := normalizeKey('j', true), rune('j'); got != want {
		t.Fatalf("normalizeKey('j', true) = %d, want %d", got, want)
	}
}

func TestReadInputKeyTranslatesArrowSequence(t *testing.T) {
	reader := &fakeTTY{
		runes:    []rune{0x1B, 0x5b, 'B'},
		buffered: true,
	}

	r, done, err := readInputKey(reader)
	if err != nil {
		t.Fatalf("readInputKey returned error: %v", err)
	}
	if done {
		t.Fatalf("done = true, want false")
	}
	if got, want := r, rune(0x0E); got != want {
		t.Fatalf("r = %d, want %d", got, want)
	}
}

func TestReadInputKeyReturnsEscapeWhenStandalone(t *testing.T) {
	reader := &fakeTTY{
		runes:    []rune{0x1B},
		buffered: false,
	}

	r, done, err := readInputKey(reader)
	if err != nil {
		t.Fatalf("readInputKey returned error: %v", err)
	}
	if !done {
		t.Fatalf("done = false, want true")
	}
	if got, want := r, rune(0x1B); got != want {
		t.Fatalf("r = %d, want %d", got, want)
	}
}

func TestTransformResultsExtractsFirstCaptureGroup(t *testing.T) {
	results, err := transformResults([]string{"id=42 name=alpha"}, `id=(\d+)`)
	if err != nil {
		t.Fatalf("transformResults returned error: %v", err)
	}
	if got, want := results[0], "42"; got != want {
		t.Fatalf("results[0] = %q, want %q", got, want)
	}
}

func TestFormatResultsWritesLines(t *testing.T) {
	output, err := formatResults([]string{"alpha", "beta"}, false)
	if err != nil {
		t.Fatalf("formatResults returned error: %v", err)
	}
	if got, want := output, "alpha\nbeta\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestDrawRendersQueryAndCurrentLine(t *testing.T) {
	render := renderState{
		queryPrompt: "> abc",
		showCursor:  true,
		clearBelow:  true,
		cursorUp:    3,
		cursorCol:   5,
		lines: []renderLine{
			{index: 0, text: "Alpha", selected: true, current: true},
			{index: 1, text: "Beta", selected: false, current: false},
		},
	}
	dirty := []bool{true, true}
	style := drawStyle{
		fillStart: "<F>",
		fillEnd:   "<FE>",
		clearEnd:  "<C>",
		fg:        "30",
		bg:        "47",
		multi:     true,
	}

	var buf strings.Builder
	_, err := draw(&buf, render, dirty, style)
	if err != nil {
		t.Fatalf("draw returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "\r<C>> abc\n") {
		t.Fatalf("output missing query prompt: %q", out)
	}
	if !strings.Contains(out, "*<F>\x1b[30;47mAlpha<FE>\r") {
		t.Fatalf("output missing current selected line: %q", out)
	}
	if !strings.Contains(out, " <F>Beta<C>\r") {
		t.Fatalf("output missing plain line: %q", out)
	}
	if !strings.Contains(out, "\x1b[3A\x1b[5C") {
		t.Fatalf("output missing cursor restore: %q", out)
	}
	if dirty[0] || dirty[1] {
		t.Fatalf("dirty flags were not cleared: %+v", dirty)
	}
}
