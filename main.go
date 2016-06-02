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

	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	w := colorable.NewColorableStdout()
	result := ""

	w.Write([]byte("\x1b[?25l"))

	defer func() {
		tty.Close()
		w.Write([]byte("\x1b[?25h\x1b[0J"))
		if result != "" {
			w.Write([]byte(result + "\n"))
		} else {
			os.Exit(1)
		}
	}()

	row := 0
	for {
		pos := 0
		for i, line := range lines {
			if i == row {
				w.Write([]byte("\x1b[30;47m" + line + "\x1b[0m\x1b[0K\r\n"))
			} else {
				w.Write([]byte(line + "\x1b[0K\r\n"))
			}
			pos++
		}
		w.Write([]byte(fmt.Sprintf("\x1b[%dA", pos)))

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
