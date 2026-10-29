package gazeta

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strconv"
)

// coverMetadata independently verifies native output against the post cover.
func (e *engine) coverMetadata(root string, post document) (map[string]string, error) {
	f, err := os.Open(filepath.Join(root, "content", filepath.Dir(post.Source), post.Meta.Cover))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return nil, fmt.Errorf("%s: decode social cover: %w", post.Source, err)
	}
	alt := post.Meta.CoverAlt
	if alt == "" {
		alt = post.Meta.Title
	}
	url := e.coverURL(post)
	return map[string]string{
		"og:image": url, "og:image:alt": alt, "og:image:type": "image/" + format,
		"og:image:width": strconv.Itoa(cfg.Width), "og:image:height": strconv.Itoa(cfg.Height),
		"twitter:image": url, "twitter:image:alt": alt,
	}, nil
}
