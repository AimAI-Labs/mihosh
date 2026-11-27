//go:build !linux && !darwin

package config

func readProcessArguments(int) ([]string, string) { return nil, "" }
