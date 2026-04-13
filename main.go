package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"unicode"

	"github.com/mattn/go-colorable"
	"github.com/mattn/go-runewidth"
	"github.com/mattn/go-tty"
)

const name = "cho"
const version = "0.0.17"

var revision = "HEAD"

type AnsiColor map[string]string

func (a AnsiColor) Get(name, fallback string) string {
	if c, ok := a[name]; ok {
		return c
	}
	return a[fallback]
}

var (
	cursorline    = flag.Bool("cl", false, "Cursor line")
	linefg        = flag.String("lf", "black", "Line foreground")
	linebg        = flag.String("lb", "white", "Line background")
	color         = flag.Bool("cc", false, "Handle colors")
	nocolorres    = flag.Bool("nc", false, "No colors for result")
	query         = flag.Bool("q", false, "Use query")
	ignoreCase    = flag.Bool("ic", false, "Ignore case match")
	multi         = flag.Bool("m", false, "Multi select")
	maxlines      = flag.Int("M", -1, "Max lines")
	sep           = flag.String("sep", "", "Separator for prefix")
	offset        = flag.Int("off", 0, "Header offset")
	resultPattern = flag.String("pat", "", "Result pattern")
	showVersion   = flag.Bool("v", false, "Print the version")
	truncate      = runewidth.Truncate

	fgcolor = AnsiColor{
		"gray":    "30",
		"black":   "30",
		"red":     "31",
		"green":   "32",
		"yellow":  "33",
		"blue":    "34",
		"magenta": "35",
		"cyan":    "36",
		"white":   "37",
	}
	bgcolor = AnsiColor{
		"black":   "40",
		"gray":    "40",
		"red":     "41",
		"green":   "42",
		"yellow":  "43",
		"blue":    "44",
		"magenta": "45",
		"cyan":    "46",
		"white":   "47",
	}
)

type item struct {
	result  string
	display string
	index   int
}

type uiState struct {
	items        []item
	view         []item
	selected     []bool
	dirty        []bool
	query        []rune
	row          int
	off          int
	offset       int
	maxVisible   int
	queryEnabled bool
	multi        bool
	matcher      func(string, string) int
}

type renderLine struct {
	index    int
	text     string
	selected bool
	current  bool
}

type renderState struct {
	lines       []renderLine
	cursorCol   int
	cursorUp    int
	showCursor  bool
	clearBelow  bool
	queryPrompt string
}

type ttyReader interface {
	ReadRune() (rune, error)
	Buffered() bool
}

type drawStyle struct {
	fillStart string
	fillEnd   string
	clearEnd  string
	fg        string
	bg        string
	multi     bool
}

func readLines(r io.Reader) ([]string, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, fmt.Errorf("no buffer to work with was available")
	}

	s := strings.ReplaceAll(string(b), "\r", "")
	s = strings.TrimSuffix(s, "\n")
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines, nil
}

func splitLine(line, sep string) (string, string) {
	if sep == "" {
		return line, line
	}
	tok := strings.SplitN(line, sep, 2)
	if len(tok) == 2 {
		return tok[0], tok[1]
	}
	return tok[0], ""
}

func buildItems(lines []string, sep string) []item {
	items := make([]item, 0, len(lines))
	for i, line := range lines {
		result, display := splitLine(line, sep)
		items = append(items, item{
			result:  result,
			display: display,
			index:   i,
		})
	}
	return items
}

func filterItems(items []item, query string, mf func(string, string) int) []item {
	if query == "" {
		out := make([]item, len(items))
		copy(out, items)
		return out
	}

	filtered := make([]item, 0, len(items))
	for _, it := range items {
		if mf(it.display, query) != -1 {
			filtered = append(filtered, it)
		}
	}
	return filtered
}

func resolveResults(view []item, selected []bool, row int, multi bool) []string {
	if len(view) == 0 {
		return nil
	}
	if !multi {
		if row < 0 || row >= len(view) {
			return nil
		}
		return []string{view[row].result}
	}

	results := make([]string, 0)
	for _, it := range view {
		if it.index >= 0 && it.index < len(selected) && selected[it.index] {
			results = append(results, it.result)
		}
	}
	return results
}

func markAllDirty(dirty []bool) {
	for i := range dirty {
		dirty[i] = true
	}
}

func clampViewState(view []item, offset, row, off int) (int, int) {
	if len(view) == 0 {
		return offset, off
	}
	if row >= len(view) {
		row = len(view) - 1
	}
	if row < offset {
		row = offset
	}
	if off > row {
		off = row
	}
	return row, off
}

func visibleItems(view []item, off int) []item {
	if off < 0 || off >= len(view) {
		return nil
	}
	return view[off:]
}

func newUIState(items []item, offset int, queryEnabled, multi bool, matcher func(string, string) int) *uiState {
	dirty := make([]bool, len(items))
	selected := make([]bool, len(items))
	markAllDirty(dirty)
	return &uiState{
		items:        items,
		view:         items,
		selected:     selected,
		dirty:        dirty,
		row:          offset,
		offset:       offset,
		queryEnabled: queryEnabled,
		multi:        multi,
		matcher:      matcher,
	}
}

func (s *uiState) queryString() string {
	return string(s.query)
}

func (s *uiState) refreshView() {
	if s.queryEnabled {
		s.view = filterItems(s.items, s.queryString(), s.matcher)
		markAllDirty(s.dirty)
		if s.off >= len(s.view) {
			s.off = 0
		}
	} else {
		s.view = s.items
	}
	s.row, s.off = clampViewState(s.view, s.offset, s.row, s.off)
}

func (s *uiState) moveDown() {
	if s.row >= len(s.view)-1 {
		return
	}
	s.dirty[s.view[s.row].index] = true
	s.dirty[s.view[s.row+1].index] = true
	s.row++
	if s.row-s.off >= s.maxVisible {
		s.off++
		markAllDirty(s.dirty)
	}
}

func (s *uiState) moveUp() {
	if s.row <= s.offset {
		return
	}
	s.dirty[s.view[s.row].index] = true
	s.dirty[s.view[s.row-1].index] = true
	s.row--
	if s.row < s.off {
		s.off--
		markAllDirty(s.dirty)
	}
}

func (s *uiState) resetToTop() {
	s.row = s.offset
	s.off = 0
}

func (s *uiState) clearQuery() {
	if !s.queryEnabled || len(s.query) == 0 {
		return
	}
	s.query = nil
	s.resetToTop()
	markAllDirty(s.dirty)
}

func (s *uiState) backspaceQuery() {
	if !s.queryEnabled || len(s.query) == 0 {
		return
	}
	s.query = s.query[:len(s.query)-1]
	s.resetToTop()
	if len(s.query) == 0 {
		markAllDirty(s.dirty)
	}
}

func (s *uiState) appendQuery(r rune) {
	if !s.queryEnabled || !unicode.IsPrint(r) {
		return
	}
	s.query = append(s.query, r)
	s.resetToTop()
}

func (s *uiState) toggleCurrentSelection() {
	if s.row < 0 || s.row >= len(s.view) {
		return
	}
	idx := s.view[s.row].index
	s.selected[idx] = !s.selected[idx]
	s.dirty[idx] = true
}

func (s *uiState) results() []string {
	return resolveResults(s.view, s.selected, s.row, s.multi)
}

func (s *uiState) currentView() []item {
	return visibleItems(s.view, s.off)
}

func buildRenderState(state *uiState, width, maxLines int, trunc func(string, int, string) string) renderState {
	render := renderState{
		showCursor: state.queryEnabled,
		cursorUp:   0,
		cursorCol:  0,
	}
	if state.queryEnabled {
		render.queryPrompt = "> " + state.queryString()
		render.cursorUp = 1
		render.cursorCol = runewidth.StringWidth(state.queryString()) + 2
		render.clearBelow = len(state.query) > 0
	}

	for i, it := range state.currentView() {
		if len(render.lines) >= maxLines {
			break
		}
		line := strings.Replace(it.display, "\t", "    ", -1)
		render.lines = append(render.lines, renderLine{
			index:    it.index,
			text:     trunc(line, width, ""),
			selected: state.selected[it.index],
			current:  state.off+i == state.row,
		})
		render.cursorUp++
	}
	return render
}

func normalizeKey(r rune, queryEnabled bool) rune {
	if queryEnabled {
		return r
	}
	switch r {
	case 'j':
		return 0x0E
	case 'k':
		return 0x10
	default:
		return r
	}
}

func readKey(reader ttyReader) (rune, error) {
	for {
		r, err := reader.ReadRune()
		if err != nil {
			return 0, err
		}
		if r != 0 {
			return r, nil
		}
	}
}

func readInputKey(reader ttyReader) (rune, bool, error) {
	r, err := readKey(reader)
	if err != nil {
		return 0, false, err
	}
	if r != 0x1B {
		return r, false, nil
	}
	if !reader.Buffered() {
		return r, true, nil
	}
	next, err := reader.ReadRune()
	if err != nil {
		return 0, false, err
	}
	if next != 0x5b {
		return r, true, nil
	}
	key, err := reader.ReadRune()
	if err != nil {
		return 0, false, err
	}
	switch key {
	case 'A', 'Z':
		return 0x10, false, nil
	case 'B':
		return 0x0E, false, nil
	default:
		return r, true, nil
	}
}

func transformResults(results []string, pattern string) ([]string, error) {
	if pattern == "" {
		out := make([]string, len(results))
		copy(out, results)
		return out, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(results))
	copy(out, results)
	for i, line := range out {
		if vv := re.FindStringSubmatch(line); len(vv) > 1 {
			out[i] = vv[1]
		}
	}
	return out, nil
}

func formatResults(results []string, stripColor bool) (string, error) {
	var buf bytes.Buffer
	var writer io.Writer = &buf
	if stripColor {
		writer = colorable.NewNonColorable(&buf)
	}
	for _, line := range results {
		if _, err := writer.Write([]byte(line + "\n")); err != nil {
			return "", err
		}
	}
	return buf.String(), nil
}

func draw(w io.Writer, render renderState, dirty []bool, style drawStyle) (int, error) {
	n := 0
	if render.showCursor {
		n++
		if _, err := io.WriteString(w, style.fillStart); err != nil {
			return 0, err
		}
		if _, err := io.WriteString(w, "\r"+style.clearEnd+render.queryPrompt+"\n"); err != nil {
			return 0, err
		}
		if render.clearBelow {
			if _, err := io.WriteString(w, "\x1b[0J"); err != nil {
				return 0, err
			}
		}
	}
	if _, err := io.WriteString(w, "\x1b[?25l"); err != nil {
		return 0, err
	}
	for _, line := range render.lines {
		if dirty[line.index] {
			if style.multi {
				prefix := " "
				if line.selected {
					prefix = "*"
				}
				if _, err := io.WriteString(w, prefix); err != nil {
					return 0, err
				}
			}
			if _, err := io.WriteString(w, style.fillStart); err != nil {
				return 0, err
			}
			if line.current {
				if _, err := io.WriteString(w, "\x1b["+style.fg+";"+style.bg+"m"+line.text+style.fillEnd+"\r"); err != nil {
					return 0, err
				}
			} else {
				if _, err := io.WriteString(w, line.text+style.clearEnd+"\r"); err != nil {
					return 0, err
				}
			}
			dirty[line.index] = false
		}
		if n >= len(render.lines) && len(render.lines) > 0 {
			break
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return 0, err
		}
		n++
	}
	if render.showCursor {
		if _, err := io.WriteString(w, "\x1b[?25h"); err != nil {
			return 0, err
		}
	}
	if render.cursorUp >= 1 {
		if _, err := io.WriteString(w, fmt.Sprintf("\x1b[%dA", render.cursorUp)); err != nil {
			return 0, err
		}
	}
	if render.showCursor {
		if _, err := io.WriteString(w, fmt.Sprintf("\x1b[%dC", render.cursorCol)); err != nil {
			return 0, err
		}
	}
	return n, nil
}

func (s *uiState) handleKey(r rune) (done bool) {
	switch r {
	case 0x09, 0x0E:
		s.moveDown()
	case 0x10:
		s.moveUp()
	case 0x15, 0x17:
		s.clearQuery()
	case 0x16:
		s.toggleCurrentSelection()
	case 0x0D, 0x1B:
		return true
	case 0x08, 0x7F:
		s.backspaceQuery()
	default:
		s.appendQuery(r)
	}
	return false
}

func truncateAnsi(line string, w int, _ string) string {
	r := []rune(line)
	out := []rune{}
	width := 0
	i := 0
	for ; i < len(r); i++ {
		if i < len(r)-1 && r[i] == '\x1b' && r[i+1] == '[' {
			j := i + 2
			for ; j < len(r); j++ {
				if ('a' <= r[j] && r[j] <= 'z') || ('A' <= r[j] && r[j] <= 'Z') {
					if r[j] == 'm' {
						s := ""
						for _, tok := range strings.Split(string(r[i+2:j]), ";") {
							n, _ := strconv.Atoi(tok)
							if n == 0 || n == 39 || (30 <= n && n <= 37) {
								if s != "" {
									s += ";"
								}
								if n == 0 {
									tok = "39"
								}
								s += tok
							}
						}
						s = "\x1b[" + s + "m"
						out = append(out, []rune(s)...)
					}
					break
				}
			}
			i = j
			continue
		}
		cw := runewidth.RuneWidth(r[i])
		if width+cw > w {
			break
		}
		width += cw
		out = append(out, r[i])
	}
	return string(out)
}

func main() {
	flag.Parse()
	if *showVersion {
		fmt.Printf("%s %s (rev: %s/%s)\n", name, version, revision, runtime.Version())
		return
	}

	fillstart := "\x1b[0K"
	fillend := "\x1b[0m"
	clearend := "\x1b[0K"
	if *cursorline {
		fillstart = ""
		fillend = "\x1b[0K\x1b[0m"
	}
	fg := fgcolor.Get(*linefg, "black")
	bg := bgcolor.Get(*linebg, "white")
	style := drawStyle{
		fillStart: fillstart,
		fillEnd:   fillend,
		clearEnd:  clearend,
		fg:        fg,
		bg:        bg,
		multi:     *multi,
	}

	if *color {
		truncate = truncateAnsi
	}

	lines, err := readLines(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	results := []string{}
	items := []item(nil)

	tty, err := tty.Open()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	clean, err := tty.Raw()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	defer colorable.EnableColorsStdout(nil)()
	out := colorable.NewColorable(tty.Output())

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)
	go func() {
		<-sc
		out.Write([]byte("\x1b[?25h\x1b[0J"))
		clean()
		tty.Close()
		os.Exit(1)
	}()

	if *sep == "TAB" {
		*sep = "\t"
	}
	items = buildItems(lines, *sep)
	if !*query {
		out.Write([]byte("\x1b[?25l"))
	}

	defer func() {
		e := recover()
		out.Write([]byte("\x1b[?25h\r\x1b[0J"))
		clean()
		tty.Close()
		if e != nil {
			panic(e)
		}
		if len(results) > 0 {
			results, err = transformResults(results, *resultPattern)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			output, err := formatResults(results, *nocolorres)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			fmt.Print(output)
		} else {
			os.Exit(1)
		}
	}()

	mf := strings.Index
	if *ignoreCase {
		mf = func(s, substr string) int {
			return strings.Index(strings.ToUpper(s), strings.ToUpper(substr))
		}
	}
	state := newUIState(items, *offset, *query, *multi, mf)
	ml := 0
	mh := 0
	for {
		w, h, err := tty.Size()
		if err != nil {
			w = 79
			h = 25
		}
		if *multi {
			w -= 2
		}
		if *maxlines > 0 && *maxlines < h-1 {
			ml = *maxlines
		} else {
			ml = h - 2
		}
		if *query {
			mh = ml
		} else {
			mh = ml + 1
		}
		state.maxVisible = mh
		state.refreshView()
		render := buildRenderState(state, w, ml, truncate)
		if _, err := draw(out, render, state.dirty, style); err != nil {
			panic(err)
		}

		r, done, err := readInputKey(tty)
		if err != nil {
			panic(err)
		}
		r = normalizeKey(r, *query)
		switch r {
		case 0x0D: // ENTER
			results = state.results()
			return
		case 0x1B:
			if done {
				return
			}
			state.handleKey(r)
		default:
			if done || state.handleKey(r) {
				if r == 0x1B {
					return
				}
			}
			if done {
				return
			}
		}
		if r == 0x1B {
			return
		}
	}
}
