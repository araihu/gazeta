package gazeta

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func copyOptionalTree(source, target string) error {
	if _, err := os.Lstat(source); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	return copyPublicationTree(source, target)
}
func (e *engine) copyFiles(root, render string) error {
	for _, f := range e.config.Files {
		if err := rejectSymlinkPath(root, f.Source); err != nil {
			return err
		}
		if err := copyPublicationTree(filepath.Join(root, filepath.FromSlash(f.Source)), filepath.Join(render, filepath.FromSlash(f.Target))); err != nil {
			return err
		}
	}
	return nil
}
func rejectSymlinkPath(root, relative string) error {
	current := root
	for _, part := range strings.Split(filepath.ToSlash(relative), "/") {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("cannot copy symlink %s", current)
		}
	}
	return nil
}
func copyPublicationTree(source, target string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("cannot copy symlink %s", source)
	}
	if !info.IsDir() {
		return copyPublicationFile(source, target)
	}
	return filepath.WalkDir(source, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 || (!entry.IsDir() && !entry.Type().IsRegular()) {
			return fmt.Errorf("unsupported publication file: %s", file)
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(source, file)
		if err != nil {
			return err
		}
		return copyPublicationFile(file, filepath.Join(target, relative))
	})
}
func copyPublicationFile(source, target string) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file: %s", source)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	existing, err := os.ReadFile(target)
	if err == nil {
		if !bytes.Equal(existing, data) {
			return fmt.Errorf("publication file collision: %s", target)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	return writeFile(target, data)
}
