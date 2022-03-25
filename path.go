//go:build !windows
// +build !windows

package scp

func realPath(path string) string {
	return path
}
