// +build darwin dragonfly freebsd netbsd openbsd

package main

import (
	"syscall"
)

const (
	ioctlReadTermios  = syscall.TIOCGETA
	ioctlWriteTermios = syscall.TIOCSETA
)
