//go:build !darwin && !linux

package app

func childPIDs(int) []int     { return nil }
func commandRunning(int) bool { return false }
