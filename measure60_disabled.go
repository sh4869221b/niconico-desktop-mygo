//go:build !mygo_measure60

package main

func observeLibrary60(_ string, action func() error) error { return action() }
