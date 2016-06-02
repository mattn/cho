// +build linux

package main

import (
	"bufio"
	"os"
	"syscall"
	"unsafe"
)

const (
	ioctlReadTermios  = 0x5401 // syscall.TCGETS
	ioctlWriteTermios = 0x5402 // syscall.TCSETS
)

type TTY struct {
	in      *os.File
	out     *os.File
	st      uint32
	w       int
	h       int
	termios syscall.Termios
}

func (tty *TTY) readRune() (rune, error) {
	in := bufio.NewReader(tty.in)
	r, _, err := in.ReadRune()
	return r, err
}

func newTTY() (*TTY, error) {
	tty := new(TTY)
	//tty.in = os.Stdin

	in, err := os.Open("/dev/tty")
	if err != nil {
		return nil, err
	}
	tty.in = in
	tty.out = os.Stdout

	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(tty.in.Fd()), ioctlReadTermios, uintptr(unsafe.Pointer(&tty.termios)), 0, 0, 0); err != 0 {
		return nil, err
	}
	newios := tty.termios
	newios.Iflag &^= syscall.ISTRIP | syscall.INLCR | syscall.ICRNL | syscall.IGNCR | syscall.IXON | syscall.IXOFF
	newios.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.ISIG
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(tty.in.Fd()), ioctlWriteTermios, uintptr(unsafe.Pointer(&newios)), 0, 0, 0); err != 0 {
		return nil, err
	}
	return tty, nil
}

func (tty *TTY) Close() error {
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(tty.in.Fd()), ioctlWriteTermios, uintptr(unsafe.Pointer(&tty.termios)), 0, 0, 0)
	return err
}
