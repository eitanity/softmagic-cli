//go:build windows

package main

import "errors"

func mkfifo(string) error { return errors.New("no fifos on Windows") }
