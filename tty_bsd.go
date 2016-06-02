// +build darwin

package main

const (
	ioctlReadTermios  = 0x40487413 // syscall.TIOCGETA
	ioctlWriteTermios = 0x80487414 // syscall.TIOCSETA
)
