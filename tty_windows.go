// +build windows

package main

import (
	"os"
	"syscall"
	"unsafe"

	"github.com/mattn/go-isatty"
)

const (
	enableEchoInput      = 0x4
	enableInsertMode     = 0x20
	enableLineInput      = 0x2
	enableMouseInput     = 0x10
	enableProcessedInput = 0x1
	enableQuickEditMode  = 0x40
	enableWindowInput    = 0x8

	keyEvent              = 0x1
	mouseEvent            = 0x2
	windowBufferSizeEvent = 0x4
)

var kernel32 = syscall.NewLazyDLL("kernel32.dll")

var (
	procSetStdHandle                = kernel32.NewProc("SetStdHandle")
	procGetStdHandle                = kernel32.NewProc("GetStdHandle")
	procSetConsoleScreenBufferSize  = kernel32.NewProc("SetConsoleScreenBufferSize")
	procCreateConsoleScreenBuffer   = kernel32.NewProc("CreateConsoleScreenBuffer")
	procGetConsoleScreenBufferInfo  = kernel32.NewProc("GetConsoleScreenBufferInfo")
	procWriteConsoleOutputCharacter = kernel32.NewProc("WriteConsoleOutputCharacterW")
	procWriteConsoleOutputAttribute = kernel32.NewProc("WriteConsoleOutputAttribute")
	procGetConsoleCursorInfo        = kernel32.NewProc("GetConsoleCursorInfo")
	procSetConsoleCursorInfo        = kernel32.NewProc("SetConsoleCursorInfo")
	procSetConsoleCursorPosition    = kernel32.NewProc("SetConsoleCursorPosition")
	procReadConsoleInput            = kernel32.NewProc("ReadConsoleInputW")
	procGetConsoleMode              = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode              = kernel32.NewProc("SetConsoleMode")
	procFillConsoleOutputCharacter  = kernel32.NewProc("FillConsoleOutputCharacterW")
	procFillConsoleOutputAttribute  = kernel32.NewProc("FillConsoleOutputAttribute")
	procScrollConsoleScreenBuffer   = kernel32.NewProc("ScrollConsoleScreenBufferW")
)

type wchar uint16
type short int16
type dword uint32
type word uint16

type coord struct {
	x short
	y short
}

type smallRect struct {
	left   short
	top    short
	right  short
	bottom short
}

type consoleScreenBufferInfo struct {
	size              coord
	cursorPosition    coord
	attributes        word
	window            smallRect
	maximumWindowSize coord
}

type consoleCursorInfo struct {
	size    dword
	visible int32
}

type inputRecord struct {
	eventType word
	_         [2]byte
	event     [16]byte
}

type keyEventRecord struct {
	keyDown         int32
	repeatCount     word
	virtualKeyCode  word
	virtualScanCode word
	unicodeChar     wchar
	controlKeyState dword
}

type windowBufferSizeRecord struct {
	size coord
}

type mouseEventRecord struct {
	mousePos        coord
	buttonState     dword
	controlKeyState dword
	eventFlags      dword
}

type charInfo struct {
	unicodeChar wchar
	attributes  word
}

type TTY struct {
	in  uintptr
	out uintptr
	st  uint32
	w   int
	h   int
}

func readConsoleInput(fd uintptr, record *inputRecord) (err error) {
	var w uint32
	r1, _, err := procReadConsoleInput.Call(fd, uintptr(unsafe.Pointer(record)), 1, uintptr(unsafe.Pointer(&w)))
	if r1 == 0 {
		return err
	}
	return nil
}

func (tty *TTY) readRune() (rune, error) {
	var ir inputRecord
	err := readConsoleInput(tty.in, &ir)
	if err != nil {
		return 0, err
	}

	if ir.eventType == keyEvent {
		kr := (*keyEventRecord)(unsafe.Pointer(&ir.event))
		if kr.keyDown != 0 {
			return rune(kr.unicodeChar), nil
		}
	}
	return 0, nil
}

func newTTY() (*TTY, error) {
	tty := new(TTY)
	if isatty.IsTerminal(os.Stdin.Fd()) {
		tty.in = os.Stdin.Fd()
	} else {
		conin, err := os.Open("CONIN$")
		if err != nil {
			return nil, err
		}
		tty.in = conin.Fd()
	}

	if isatty.IsTerminal(os.Stdout.Fd()) {
		tty.out = os.Stdout.Fd()
	} else {
		conout, err := os.Open("CONOUT$")
		if err != nil {
			return nil, err
		}
		tty.out = conout.Fd()
	}

	var st uint32
	r1, _, err := procGetConsoleMode.Call(tty.in, uintptr(unsafe.Pointer(&st)))
	if r1 == 0 {
		return nil, err
	}
	tty.st = st

	st &^= enableEchoInput
	st &^= enableInsertMode
	st &^= enableLineInput
	st &^= enableMouseInput
	st |= enableWindowInput

	// ignore error
	procSetConsoleMode.Call(tty.in, uintptr(st))

	var csbi consoleScreenBufferInfo

	r1, _, err = procGetConsoleScreenBufferInfo.Call(tty.out, uintptr(unsafe.Pointer(&csbi)))
	if r1 == 0 {
		return nil, err
	}
	tty.w = int(csbi.size.x)
	tty.h = int(csbi.size.y)

	return tty, nil
}

func (tty *TTY) Close() {
	procSetConsoleMode.Call(tty.in, uintptr(tty.st))
}
