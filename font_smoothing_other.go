//go:build !darwin

package main

// disableFontSmoothing does nothing: font smoothing follows AppleFontSmoothing
// on macOS only.
func disableFontSmoothing() {}
