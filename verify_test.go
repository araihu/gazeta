package gazeta

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func verificationFixture(t *testing.T, mode *atomic.Value) (string, *publicationVerification) {
	t.Helper()
	root := exampleRoot(t)
	var origin string
	var manifestRequests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		behavior, _ := mode.Load().(string)
		if strings.HasPrefix(r.URL.Path, "/.well-known/gazeta-missing-") {
			w.Header().Set("Content-Type", "text/html")
			if behavior == "soft 404" {
				w.WriteHeader(200)
			} else {
				w.WriteHeader(404)
			}
			fmt.Fprint(w, readTestFile(t, root, "public/404.html"))
			return
		}
		if r.URL.Path == "/posts/hello/" {
			target := origin + "/blogs/hello/"
			if behavior == "wrong alias" {
				target = origin + "/about/"
			}
			http.Redirect(w, r, target, 301)
			return
		}
		if r.URL.Path == "/404.html" && behavior == "clean URL" {
			http.Redirect(w, r, "/404", 307)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "404" {
			name = "404.html"
		}
		if name == "" || strings.HasSuffix(name, "/") {
			name += "index.html"
		}
		data, err := os.ReadFile(filepath.Join(root, "public", filepath.FromSlash(name)))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(name)))
		if name == "assets/editorial.css" && behavior == "wrong content type" {
			w.Header().Set("Content-Type", "text/plain")
		}
		if name == "margo-manifest.json" && behavior == "stale manifest once" && manifestRequests.Add(1) == 1 {
			w.WriteHeader(503)
			return
		}
		if name == "margo-manifest.json" && behavior == "stale manifest" {
			data = append([]byte("!"), data[1:]...)
		}
		if name == "assets/editorial.css" && behavior == "wrong asset" {
			data = []byte("body { display: none }")
		}
		if name == "blogs/hello/index.html" {
			text := string(data)
			switch behavior {
			case "wrong author":
				text = strings.ReplaceAll(text, "Alex Example", "Another Author")
			case "wrong canonical":
				text = strings.Replace(text, `rel="canonical" href="`+origin+`/blogs/hello/"`, `rel="canonical" href="`+origin+`/about/"`, 1)
			case "wrong social image":
				text = strings.Replace(text, `property="og:image" content="`, `property="og:image" content="https://other.example/`, 1)
			case "duplicate schema":
				text = strings.Replace(text, "</head>", `<script type="application/ld+json">{}</script></head>`, 1)
			case "duplicate JSON property":
				text = strings.Replace(text, `"@type":"BlogPosting"`, `"@type":"Article","@type":"BlogPosting"`, 1)
			case "stale body":
				text = strings.Replace(text, "Content stays in Git.", "Old article content.", 1)
			case "hosting injection":
				text = strings.Replace(text, "</body>", `<script>window.__CF$cv$params={}; '/cdn-cgi/challenge-platform/';</script></body>`, 1)
			case "unexpected redirect":
				http.Redirect(w, r, "https://other.example/", 302)
				return
			}
			data = []byte(text)
		}
		w.Write(data)
	}))
	t.Cleanup(server.Close)
	origin = server.URL
	replaceTestFile(t, root, "site.yaml", "https://notes.example", origin)
	if err := Build(context.Background(), root, buildTime); err != nil {
		t.Fatal(err)
	}
	verifier, err := loadPublicationVerification(root)
	if err != nil {
		t.Fatal(err)
	}
	client := server.Client()
	client.CheckRedirect = verifier.client.CheckRedirect
	verifier.client = client
	return root, verifier
}

func TestVerifyPublicationContract(t *testing.T) {
	var mode atomic.Value
	mode.Store("")
	_, verifier := verificationFixture(t, &mode)
	for _, tc := range []struct{ name, want string }{
		{"valid artifact", ""}, {"hosting injection", ""}, {"clean URL", ""},
		{"stale manifest", "manifest"}, {"wrong asset", "asset"}, {"wrong content type", "Content-Type"},
		{"wrong author", "JSON-LD"}, {"wrong canonical", "canonical"},
		{"wrong social image", "head metadata"}, {"duplicate schema", "JSON-LD"},
		{"duplicate JSON property", "duplicate"}, {"stale body", "HTML content"},
		{"soft 404", "instead of 404"}, {"wrong alias", "legacy redirect"},
		{"unexpected redirect", "HTTP 302"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mode.Store(tc.name)
			err := verifier.check(context.Background())
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q failure, got %v", tc.want, err)
			}
		})
	}
}

func TestVerifyRetriesAndCancellation(t *testing.T) {
	var mode atomic.Value
	mode.Store("")
	root, verifier := verificationFixture(t, &mode)
	mode.Store("stale manifest once")
	report, err := Verify(context.Background(), root, VerifyOptions{Client: verifier.client, Timeout: 5 * time.Second, Interval: time.Millisecond})
	if err != nil || report.Pages < 1 || report.Assets < 1 || report.Redirects != 1 || report.Attempts != 2 {
		t.Fatalf("invalid verification report: %+v, %v", report, err)
	}
	mode.Store("")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Verify(ctx, root, VerifyOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation ignored: %v", err)
	}
	if _, err := Verify(context.Background(), root, VerifyOptions{Timeout: -1}); err == nil {
		t.Fatal("negative deadline accepted")
	}
	if err := os.WriteFile(filepath.Join(root, "public/assets/editorial.css"), []byte("changed locally"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(context.Background(), root, VerifyOptions{}); err == nil || !strings.Contains(err.Error(), "local publication") {
		t.Fatalf("local artifact drift accepted: %v", err)
	}
}
