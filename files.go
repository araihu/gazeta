package gazeta

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func copyOptionalTree(source, target string) error {
	if _, err := os.Lstat(source); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	return copyTree(source, target)
}
func (e *engine) copyFiles(root, render string) error {
	for _, f := range e.config.Files {
		source := filepath.Join(root, filepath.FromSlash(f.Source))
		info, err := os.Lstat(source)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("cannot copy symlink %s", source)
		}
		if !info.IsDir() {
			if err := copyPublicationFile(source, filepath.Join(render, f.Target)); err != nil {
				return err
			}
			continue
		}
		err = filepath.WalkDir(source, func(file string, entry fs.DirEntry, err error) error {
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
			return copyPublicationFile(file, filepath.Join(render, f.Target, relative))
		})
		if err != nil {
			return err
		}
	}
	return nil
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
