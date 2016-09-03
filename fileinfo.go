package scp

import (
	"os"
	"syscall"
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

func NewFileInfoFromOS(fi os.FileInfo, setTime bool, replaceName string) FileInfo {
	var name string
	if replaceName == "" {
		name = fi.Name()
	} else {
		name = replaceName
	}

	mode := fi.Mode() & os.ModePerm

	var modTime time.Time
	var accessTime time.Time
	if setTime {
		modTime = fi.ModTime()

		sysStat, ok := fi.Sys().(*syscall.Stat_t)
		if ok {
			sec, nsec := sysStat.Atim.Unix()
			accessTime = time.Unix(sec, nsec)
		}
	}

	if fi.IsDir() {
		return NewDirInfo(name, mode, modTime, accessTime)
	} else {
		return NewFileInfo(name, fi.Size(), mode, modTime, accessTime)
	}
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

func (i *FileInfo) Name() string          { return i.name }
func (i *FileInfo) Size() int64           { return i.size }
func (i *FileInfo) Mode() os.FileMode     { return i.mode }
func (i *FileInfo) ModTime() time.Time    { return i.modTime }
func (i *FileInfo) IsDir() bool           { return i.isDir }
func (i *FileInfo) Sys() interface{}      { return i.sys }
func (i *FileInfo) AccessTime() time.Time { return i.sys.AccessTime }
