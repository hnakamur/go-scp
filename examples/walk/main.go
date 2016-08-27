package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	err := run()
	if err != nil {
		panic(err)
	}
}

func run() error {
	baseDir := "/tmp"
	prevDir := baseDir

	walkFn := func(path string, info os.FileInfo, err error) error {
		isDir := info.IsDir()
		var dir string
		if isDir {
			dir = path
		} else {
			dir = filepath.Dir(path)
		}
		fmt.Printf("path=%s, isDir=%v\n", path, isDir)
		defer func() {
			prevDir = dir
		}()

		err = processDirectories(prevDir, dir)
		if err != nil {
			return err
		}

		if !isDir {
			fmt.Printf("  writeFile path=%s\n", path)
		}
		return nil
	}
	err := filepath.Walk(baseDir, walkFn)
	if err != nil {
		return err
	}

	err = processDirectories(prevDir, baseDir)
	if err != nil {
		return err
	}

	return nil
}

func processDirectories(prevDir, dir string) error {
	rel, err := filepath.Rel(prevDir, dir)
	if err != nil {
		return err
	}
	fmt.Printf("  rel=%s\n", rel)
	for _, comp := range strings.Split(rel, string([]rune{filepath.Separator})) {
		if comp == ".." {
			fmt.Printf("  endDirectory\n")
		} else if comp == "." {
			continue
		} else {
			fmt.Printf("  startDirectory dir=%s\n", filepath.Base(dir))
		}
	}
	return nil
}
