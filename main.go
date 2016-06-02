package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strings"

	"github.com/mattn/go-colorable"
	"github.com/mattn/go-runewidth"
)

var (
	cursorline = flag.Bool("cl", false, "cursor line")
	linefg     = flag.String("lf", "black", "line foreground")
	linebg     = flag.String("lb", "white", "line background")

	fgcolor = map[string]string{
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
	bgcolor = map[string]string{
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

func main() {
	flag.Parse()

	fillstart := "\x1b[0K"
	fillend := "\x1b[0m"
	clearend := "\x1b[0K"
	if *cursorline {
		fillstart = ""
		fillend = "\x1b[0K\x1b[0m"
	}
	fg, ok := fgcolor[*linefg]
	if !ok {
		fg = fgcolor["black"]
	}
	bg, ok := bgcolor[*linebg]
	if !ok {
		bg = bgcolor["white"]
	}

	b, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	tty, err := newTTY()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	for i := 0; i < len(lines); i++ {
		lines[i] = strings.Trim(lines[i], "\r")
	}

	out := colorable.NewColorable(tty.out)
	result := ""

	out.Write([]byte("\x1b[?25l"))

	defer func() {
		e := recover()
		tty.Close()
		out.Write([]byte("\x1b[?25h\x1b[0J"))
		if e != nil {
			panic(e)
		}
		if result != "" {
			fmt.Println(result)
		} else {
			os.Exit(1)
		}
	}()

	buf := bufio.NewWriterSize(out, 8000)
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
		for i, line := range lines[off:] {
			line = strings.Replace(line, "\t", "    ", -1)
			line = runewidth.Truncate(line, w, "")
			if dirty[off+i] {
				buf.Write([]byte(fillstart))
				if off+i == row {
					buf.Write([]byte("\x1b[" + fg + ";" + bg + "m" + line + fillend + "\r"))
				} else {
					buf.Write([]byte(line + clearend + "\r"))
				}
				dirty[off+i] = false
			}
			n++
			if n >= h {
				if runtime.GOOS == "windows" {
					buf.Write([]byte("\n"))
				}
				break
			}
			buf.Write([]byte("\n"))
		}
		buf.Write([]byte(fmt.Sprintf("\x1b[%dA", n)))
		buf.Flush()

		r, err := tty.readRune()
		if err != nil {
			panic(err)
		}
		switch r {
		case 'j', 0x0E:
			if row < len(lines)-1 {
				dirty[row], dirty[row+1] = true, true
				row++
				if row-off >= h {
					off++
					for i := 0; i < len(dirty); i++ {
						dirty[i] = true
					}
				}
			}
		case 'k', 0x10:
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
			result = lines[row]
			return
		case 27:
			return
		}
	}
}
