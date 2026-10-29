package gazeta

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSocialCoverDeclarationAndValidation(t *testing.T) {
	root := t.TempDir()
	post := testDocument(t, "cover = './assets/a capa.png'\ncover_alt = 'Código & dados <juntos>'")
	var cover bytes.Buffer
	if err := png.Encode(&cover, image.NewRGBA(image.Rect(0, 0, 120, 63))); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "content", filepath.Dir(post.Source), post.Meta.Cover)
	if err := writeFile(file, cover.Bytes()); err != nil {
		t.Fatal(err)
	}
	values, err := testBlog.coverMetadata(root, post)
	if err != nil {
		t.Fatal(err)
	}
	if values["og:image"] != "https://journal.example/blogs/test/assets/a%20capa.png" || values["og:image:width"] != "120" || values["og:image:height"] != "63" || values["og:image:alt"] != post.Meta.CoverAlt {
		t.Fatalf("wrong source metadata: %v", values)
	}
	stage := filepath.Join(root, "stage")
	if err := testBlog.writePage(stage, post); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(stage, post.Source))
	if err != nil {
		t.Fatal(err)
	}
	var front struct {
		SocialImage struct{ Path, Alt string } `json:"social_image"`
	}
	if err := json.Unmarshal([]byte(strings.SplitN(string(data), "\n", 3)[1]), &front); err != nil {
		t.Fatal(err)
	}
	if front.SocialImage.Path != post.Meta.Cover || front.SocialImage.Alt != post.Meta.CoverAlt {
		t.Fatalf("wrong native declaration: %+v", front.SocialImage)
	}

	post.Meta.CoverAlt = ""
	values, err = testBlog.coverMetadata(root, post)
	if err != nil || values["og:image:alt"] != post.Meta.Title {
		t.Fatal("missing title alternative fallback")
	}
	if err := os.WriteFile(file, []byte("not an image"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := testBlog.coverMetadata(root, post); err == nil {
		t.Fatal("accepted an invalid source image")
	}
}
