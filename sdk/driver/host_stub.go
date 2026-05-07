//go:build !tinygo.wasm

package driver

import "fmt"

// Stub implementations for non-WASM builds.
// These functions are only used when compiling to WASM with TinyGo.

func CallResourceCall(call *ResourceCall) (*ResourceResult, error) {
	return nil, fmt.Errorf("CallResourceCall is only available in WASM builds")
}

func CheckAuthorization(uri, mode string, permissions *PermissionContext) (bool, error) {
	return false, fmt.Errorf("CheckAuthorization is only available in WASM builds")
}

func ResolvePackage(name string) (string, error) {
	return "", fmt.Errorf("ResolvePackage is only available in WASM builds")
}
