//go:build !windows

package remote

func restrictPrivateKey(string) error { return nil }
