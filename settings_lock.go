package main

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

func withSettingsLock(path string, update func() error) error {
	if path == "" {
		return errors.New("settings path is unavailable")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	// Lock a separate, stable file because saving replaces the JSON file.
	// Do not unlink the lock file: other processes may already hold it open.
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	deadline := time.Now().Add(time.Second)
	for {
		locked, err := trySettingsLock(lock)
		if err != nil {
			return err
		}
		if locked {
			return update()
		}
		if !time.Now().Before(deadline) {
			return errors.New("settings are locked by another instance")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
