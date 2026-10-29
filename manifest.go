package gazeta

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/araihu/margo/site"
)

// Margo owns exact-byte identities; the blog supplies its final files and routes.
func writePublicationManifest(render string, result site.Result) error {
	result.Artifacts = nil
	err := filepath.WalkDir(render, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(render, file)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(relative)
		if name == site.ManifestPath {
			return nil
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		result.Artifacts = append(result.Artifacts, site.Artifact{Path: name, Content: data})
		return nil
	})
	if err != nil {
		return err
	}
	data, err := site.MarshalManifest(result, "")
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(render, site.ManifestPath), data)
}
