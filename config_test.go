package gazeta

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigRejectsInvalidContracts(t *testing.T) {
	for _, tc := range []struct{ old, new string }{
		{"version: 1", "version: 2"}, {"home_limit: 4", "home_limit: 0"},
		{"version: 1", "version: 1\nunknown: true"},
		{"url: /about/", "url: javascript:alert(1)"},
		{"version: 1", "version: 1\nfiles: [{source: content, target: leaked}]"},
		{"version: 1", "version: 1\nfiles: [{source: ../secret, target: leaked}]"},
	} {
		t.Run(tc.new, func(t *testing.T) {
			root := exampleRoot(t)
			replaceTestFile(t, root, "blog.yaml", tc.old, tc.new)
			if _, err := loadEngine(root); err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
	root := exampleRoot(t)
	replaceTestFile(t, root, "site.yaml", "source: .build/content", "source: content")
	if _, err := loadEngine(root); err == nil {
		t.Fatal("unsafe staging source accepted")
	}
}
func TestBuildLockAndCollisionPreservePublication(t *testing.T) {
	root := exampleRoot(t)
	if err := Build(context.Background(), root, buildTime); err != nil {
		t.Fatal(err)
	}
	before := readTestFile(t, root, "public/margo-manifest.json")
	lock := filepath.Join(root, ".build/build.lock")
	if err := os.WriteFile(lock, []byte("held"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Build(context.Background(), root, buildTime); err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("concurrent build accepted: %v", err)
	}
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(filepath.Join(root, "static/blogs/hello/index.html"), []byte("shadowed")); err != nil {
		t.Fatal(err)
	}
	if err := Build(context.Background(), root, buildTime); err == nil || !strings.Contains(err.Error(), "collision") {
		t.Fatalf("static override accepted: %v", err)
	}
	if readTestFile(t, root, "public/margo-manifest.json") != before {
		t.Fatal("failed build replaced publication")
	}
}
func TestFilesRejectSymlinkAncestor(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := writeFile(filepath.Join(outside, "secret.txt"), []byte("private")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "assets/linked")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	e := &engine{config: config{Files: []fileCopy{{Source: "assets/linked/secret.txt", Target: "secret.txt"}}}}
	if err := e.copyFiles(root, t.TempDir()); err == nil {
		t.Fatal("followed symlink ancestor")
	}
}
