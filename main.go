package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strings"

	"github.com/mattn/go-colorable"
	"github.com/mattn/go-runewidth"
)

func main() {
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
	out := colorable.NewColorableStdout()
	result := ""

	out.Write([]byte("\x1b[?25l"))

	defer func() {
		tty.Close()
		out.Write([]byte("\x1b[?25h\x1b[0J"))
		if result != "" {
			out.Write([]byte(result + "\n"))
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
			return
		}
		n := 0
		for i, line := range lines[off:] {
			line = strings.Replace(line, "\t", "    ", -1)
			line = runewidth.Truncate(line, w, "")
			if dirty[off+i] {
				buf.Write([]byte("\x1b[0K"))
				if off+i == row {
					buf.Write([]byte("\x1b[30;47m" + line + "\x1b[0m\r"))
				} else {
					buf.Write([]byte(line + "\r"))
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

		var r rune
		for r == 0 {
			r, err = tty.readRune()
		}
		switch r {
		case 'j':
			if row < len(lines)-1 {
				dirty[row] = true
				row++
				dirty[row] = true
				if row-off >= h {
					off++
					for i := 0; i < len(dirty); i++ {
						dirty[i] = true
					}
				}
			}
		case 'k':
			if row > 0 {
				dirty[row] = true
				row--
				dirty[row] = true
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
