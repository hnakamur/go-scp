package scp

import (
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// FileInfo represents a file or a directory information.
type FileInfo struct {
	name       string
	size       int64
	mode       os.FileMode
	modTime    time.Time
	accessTime time.Time
}

func newFileInfoFromOS(fi os.FileInfo, replaceName string) *FileInfo {
	var name string
	if replaceName == "" {
		name = fi.Name()
	} else {
		name = replaceName
	}

	modTime := fi.ModTime()

	var accessTime time.Time
	sysStat, ok := fi.Sys().(*syscall.Stat_t)
	if ok {
		sec, nsec := sysStat.Atim.Unix()
		accessTime = time.Unix(sec, nsec)
	}

	if fi.IsDir() {
		return newDirInfo(name, fi.Mode(), modTime, accessTime)
	}
	return newFileInfo(name, fi.Size(), fi.Mode(), modTime, accessTime)
}

func newFileInfo(name string, size int64, mode os.FileMode, modTime, accessTime time.Time) *FileInfo {
	return &FileInfo{
		name:       filepath.Base(name),
		size:       size,
		mode:       mode & os.ModePerm,
		modTime:    modTime,
		accessTime: accessTime,
	}
}

func newDirInfo(name string, mode os.FileMode, modTime, accessTime time.Time) *FileInfo {
	return &FileInfo{
		name:       filepath.Base(name),
		mode:       (mode & os.ModePerm) | os.ModeDir,
		modTime:    modTime,
		accessTime: accessTime,
	}
}

// Name returns base name of the file.
func (i *FileInfo) Name() string { return i.name }

// Size length in bytes for regular files; system-dependent for others.
func (i *FileInfo) Size() int64 { return i.size }

// Mode returns file mode bits.
func (i *FileInfo) Mode() os.FileMode { return i.mode }

// ModTime returns modification time.
func (i *FileInfo) ModTime() time.Time { return i.modTime }

// IsDir is abbreviation for Mode().IsDir().
func (i *FileInfo) IsDir() bool { return i.Mode().IsDir() }

// Sys returns underlying data source (can return nil).
func (i *FileInfo) Sys() interface{} { return i }

// AccessTime returns access time.
func (i *FileInfo) AccessTime() time.Time { return i.accessTime }
