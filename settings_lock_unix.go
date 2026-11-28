//go:build unix

package main

import (
	"os"
	"syscall"
)

func trySettingsLock(lock *os.File) (bool, error) {
	err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == syscall.EWOULDBLOCK || err == syscall.EINTR {
		return false, nil
	}
	return err == nil, err
}
