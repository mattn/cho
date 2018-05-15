package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"unicode"

	"github.com/mattn/go-colorable"
	"github.com/mattn/go-runewidth"
	"github.com/mattn/go-tty"
)

type AnsiColor map[string]string

func (a AnsiColor) Get(name, fallback string) string {
	if c, ok := a[name]; ok {
		return c
	}
	return a[fallback]
}

var (
	cursorline = flag.Bool("cl", false, "cursor line")
	linefg     = flag.String("lf", "black", "line foreground")
	linebg     = flag.String("lb", "white", "line background")
	color      = flag.Bool("cc", false, "handle colors")
	query      = flag.Bool("q", false, "use query")
	sep        = flag.String("sep", "", "separator for prefix")
	truncate   = runewidth.Truncate

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

	fillstart := "\x1b[0K"
	fillend := "\x1b[0m"
	clearend := "\x1b[0K"
	if *cursorline {
		fillstart = ""
		fillend = "\x1b[0K\x1b[0m"
	}
	fg := fgcolor.Get(*linefg, "black")
	bg := bgcolor.Get(*linebg, "white")

	if *color {
		truncate = truncateAnsi
	}

	b, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(b) == 0 {
		fmt.Fprintln(os.Stderr, "no buffer to work with was available")
		os.Exit(1)
	}
	lines := strings.Split(strings.Replace(strings.TrimSpace(string(b)), "\r", "", -1), "\n")
	result := ""
	var qlines, rlines []string

	if *sep != "" {
		if *sep == "TAB" {
			*sep = "\t"
		}
		rlines = make([]string, len(lines))
		for i, line := range lines {
			tok := strings.SplitN(line, *sep, 2)
			if len(tok) == 2 {
				rlines[i] = tok[0]
				lines[i] = tok[1]
			} else {
				rlines[i] = tok[0]
				lines[i] = ""
			}
		}
	}

	tty, err := tty.Open()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	out := colorable.NewColorable(tty.Output())

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)
	go func() {
		<-sc
		out.Write([]byte("\x1b[?25h\x1b[0J"))
		tty.Close()
		os.Exit(1)
	}()

	if !*query {
		out.Write([]byte("\x1b[?25l"))
	}

	defer func() {
		e := recover()
		out.Write([]byte("\x1b[?25h\r\x1b[0J"))
		tty.Close()
		if e != nil {
			panic(e)
		}
		if result != "" {
			fmt.Println(result)
		} else {
			os.Exit(1)
		}
	}()

	var rs []rune
	off := 0
	row := 0
	dirty := make([]bool, len(lines))
	for i := 0; i < len(dirty); i++ {
		dirty[i] = true
	}
	for {
		w, h, err := tty.Size()
		if err != nil {
			w = 80
			h = 25
		}
		n := 0

		if *query {
			out.Write([]byte(fillstart))
			out.Write([]byte("\r" + clearend + "> " + string(rs) + "\n"))
			n++
			qlines = nil
			if len(rs) > 0 {
				for i, line := range lines {
					if strings.Index(line, string(rs)) != -1 {
						qlines = append(qlines, line)
					}
					dirty[off+i] = true
				}
				out.Write([]byte("\x1b[0J"))
			} else {
				qlines = lines[off:]
			}
		} else {
			qlines = lines[off:]
		}
		for i, line := range qlines {
			line = strings.Replace(line, "\t", "    ", -1)
			line = truncate(line, w, "")
			if dirty[off+i] {
				out.Write([]byte(fillstart))
				if off+i == row {
					out.Write([]byte("\x1b[" + fg + ";" + bg + "m" + line + fillend + "\r"))
				} else {
					out.Write([]byte(line + clearend + "\r"))
				}
				dirty[off+i] = false
			}
			n++
			if n >= h {
				if runtime.GOOS == "windows" {
					out.Write([]byte("\n"))
				}
				break
			}
			out.Write([]byte("\n"))
		}
		if *query {
			out.Write([]byte(fmt.Sprintf("\x1b[%dA\x1b[%dC", n, runewidth.StringWidth(string(rs))+2)))
		} else {
			out.Write([]byte(fmt.Sprintf("\x1b[%dA", n)))
		}

		r, err := tty.ReadRune()
		if err != nil {
			panic(err)
		}

	retry:
		switch r {
		case '\t', 0x0E:
			if row < len(qlines)-1 {
				dirty[row], dirty[row+1] = true, true
				row++
				if row-off >= h {
					off++
					for i := 0; i < len(dirty); i++ {
						dirty[i] = true
					}
				}
			}
		case 0x10:
			if row > 0 {
				dirty[row], dirty[row-1] = true, true
				row--
				if row < off {
					off--
					for i := 0; i < len(dirty); i++ {
						dirty[i] = true
					}
				}
			}
		case 13:
			if *sep != "" {
				result = rlines[off+row]
			} else {
				result = qlines[row]
			}
			return
		case 27:
			if !tty.Buffered() {
				return
			}
			r, err = tty.ReadRune()
			if err == nil && r == 0x5b {
				r, err = tty.ReadRune()
				if err != nil {
					panic(err)
				}
				switch r {
				case 'A':
					r = 'k'
					goto retry
				case 'B':
					r = 'j'
					goto retry
				}
			}
		case 8:
			if *query && len(rs) > 0 {
				rs = rs[:len(rs)-1]
				row = 0
				off = 0
				if len(rs) == 0 {
					for i := 0; i < len(dirty); i++ {
						dirty[i] = true
					}
				}
			}
		default:
			if !*query {
				switch r {
				case 'j':
					r = 0x0E
					goto retry
				case 'k':
					r = 0x10
					goto retry
				}
			}
			if *query && unicode.IsPrint(r) {
				rs = append(rs, r)
				row = 0
				off = 0
			}
		}
	}
}
