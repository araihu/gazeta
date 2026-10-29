package gazeta

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/araihu/margo/site"
	"os"
	"path/filepath"
	"testing"
)

func TestBrandManifestHashesFinalBytes(t *testing.T) {
	root := t.TempDir()
	if err := writeFile(filepath.Join(root, "index.html"), []byte("final page")); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(filepath.Join(root, "margo-manifest.json"), []byte(`{"schemaVersion":"margo-site-manifest/v1","digest":"old","entries":[]}`)); err != nil {
		t.Fatal(err)
	}
	if err := writePublicationManifest(root, site.Result{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "margo-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Digest  string
		Entries []struct{ Path, Digest string }
	}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("final page"))
	if len(m.Entries) != 1 || m.Entries[0].Digest != hex.EncodeToString(digest[:]) {
		t.Fatal("artifact hash differs from output bytes")
	}
	if err := writePublicationManifest(root, site.Result{}); err != nil {
		t.Fatal(err)
	}
	again, err := os.ReadFile(filepath.Join(root, "margo-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, again) {
		t.Fatal("manifest is not deterministic")
	}
}
