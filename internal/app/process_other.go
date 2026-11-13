//go:build !darwin && !linux

package app

func childPIDs(int) []int           { return nil }
func processDir(int) (string, bool) { return "", false }
func processArgs(int) []string      { return nil }
func foregroundPID(int) int         { return 0 }
