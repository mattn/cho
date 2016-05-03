package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/mattn/go-colorable"
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
	defer tty.Close()

	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	w := colorable.NewColorableStdout()
	row := 0
	result := ""

	w.Write([]byte("\x1b[?25h"))
	defer func() {
		w.Write([]byte("\x1b[?25l\x1b[0J"))
		if result != "" {
			w.Write([]byte(result + "\n"))
		}
	}()

	for {
		for i, line := range lines {
			if i == row {
				w.Write([]byte("\x1b[47m" + line + "\x1b[0m\x1b[0K\r\n"))
			} else {
				w.Write([]byte(line + "\x1b[0K\r\n"))
			}
		}
		w.Write([]byte(fmt.Sprintf("\x1b[%dA", len(lines))))

		r, _ := tty.readRune()
		switch r {
		case 'j':
			if row < len(lines)-1 {
				row++
			}
		case 'k':
			if row > 0 {
				row--
			}
		case 13:
			result = lines[row]
			return
		case 27:
			return
		}
	}
}
