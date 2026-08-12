//go:build !windows

package config

import "errors"

func protect([]byte) ([]byte, error) {
	return nil, errors.New("当前系统不支持 Windows DPAPI")
}

func unprotect([]byte) ([]byte, error) {
	return nil, errors.New("当前系统不支持 Windows DPAPI")
}

func Protect(plaintext []byte) ([]byte, error)    { return protect(plaintext) }
func Unprotect(ciphertext []byte) ([]byte, error) { return unprotect(ciphertext) }
