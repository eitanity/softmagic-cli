//go:build unix

package main

import "syscall"

func mkfifo(name string) error { return syscall.Mkfifo(name, 0o600) }
