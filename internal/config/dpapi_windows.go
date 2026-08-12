//go:build windows

package config

import (
	"errors"
	"syscall"
	"unsafe"
)

const cryptprotectUIForbidden = 0x1

var (
	crypt32                = syscall.NewLazyDLL("crypt32.dll")
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procCryptProtectData   = crypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
	procLocalFree          = kernel32.NewProc("LocalFree")
)

type dataBlob struct {
	size uint32
	data *byte
}

func protect(plaintext []byte) ([]byte, error) {
	input := makeBlob(plaintext)
	var output dataBlob
	result, _, callErr := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(&input)), 0, 0, 0, 0,
		cryptprotectUIForbidden, uintptr(unsafe.Pointer(&output)),
	)
	if result == 0 {
		return nil, normalizeWindowsError(callErr)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(output.data)))
	return copyBlob(output), nil
}

// Protect 使用当前 Windows 用户的 DPAPI 加密敏感数据。
func Protect(plaintext []byte) ([]byte, error) {
	return protect(plaintext)
}

func unprotect(ciphertext []byte) ([]byte, error) {
	input := makeBlob(ciphertext)
	var output dataBlob
	result, _, callErr := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&input)), 0, 0, 0, 0,
		cryptprotectUIForbidden, uintptr(unsafe.Pointer(&output)),
	)
	if result == 0 {
		return nil, normalizeWindowsError(callErr)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(output.data)))
	return copyBlob(output), nil
}

// Unprotect 使用当前 Windows 用户的 DPAPI 解密敏感数据。
func Unprotect(ciphertext []byte) ([]byte, error) {
	return unprotect(ciphertext)
}

func makeBlob(data []byte) dataBlob {
	if len(data) == 0 {
		return dataBlob{}
	}
	return dataBlob{size: uint32(len(data)), data: &data[0]}
}

func copyBlob(blob dataBlob) []byte {
	if blob.size == 0 || blob.data == nil {
		return nil
	}
	result := make([]byte, int(blob.size))
	copy(result, unsafe.Slice(blob.data, int(blob.size)))
	return result
}

func normalizeWindowsError(err error) error {
	if err == nil || errors.Is(err, syscall.Errno(0)) {
		return errors.New("Windows DPAPI 调用失败")
	}
	return err
}
