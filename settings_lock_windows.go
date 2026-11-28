package main

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

var lockSettingsFile = syscall.NewLazyDLL("kernel32.dll").NewProc("LockFileEx")

func trySettingsLock(lock *os.File) (bool, error) {
	const (
		failImmediately = 1
		exclusive       = 2
		lockViolation   = syscall.Errno(33)
	)
	// Lock one byte from offset zero. Closing the handle releases the lock,
	// including when a process exits unexpectedly.
	var region syscall.Overlapped
	ok, _, err := lockSettingsFile.Call(lock.Fd(), failImmediately|exclusive, 0, 1, 0, uintptr(unsafe.Pointer(&region)))
	runtime.KeepAlive(lock)
	if ok != 0 {
		return true, nil
	}
	if err == lockViolation {
		return false, nil
	}
	return false, err
}
