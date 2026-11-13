//go:build !darwin

package main

// configureFontSmoothing does nothing: font smoothing follows
// AppleFontSmoothing on macOS only.
func configureFontSmoothing() {}
