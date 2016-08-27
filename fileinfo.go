package scp

import (
	"os"
	"time"
)

type FileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
	sys     SysFileInfo
}

type SysFileInfo struct {
	AccessTime time.Time
}

func NewFileInfo(name string, size int64, mode os.FileMode, modTime, accessTime time.Time) FileInfo {
	return FileInfo{
		name:    name,
		size:    size,
		mode:    mode,
		modTime: modTime,
		sys:     SysFileInfo{AccessTime: accessTime},
	}
}

func NewDirInfo(name string, mode os.FileMode, modTime, accessTime time.Time) FileInfo {
	return FileInfo{
		name:    name,
		mode:    mode,
		modTime: modTime,
		isDir:   true,
		sys:     SysFileInfo{AccessTime: accessTime},
	}
}

func (i *FileInfo) Name() string       { return i.name }
func (i *FileInfo) Size() int64        { return i.size }
func (i *FileInfo) Mode() os.FileMode  { return i.mode }
func (i *FileInfo) ModTime() time.Time { return i.modTime }
func (i *FileInfo) IsDir() bool        { return i.isDir }
func (i *FileInfo) Sys() interface{}   { return i.sys }
