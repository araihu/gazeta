package gazeta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/araihu/margo/site"
)

// VerifyOptions bounds publication propagation checks. Zero values use defaults.
type VerifyOptions struct {
	// Client optionally supplies a transport, for example a private CA. Redirects remain disabled.
	Client *http.Client
	// Timeout is the total verification deadline. The default is two minutes.
	Timeout time.Duration
	// Interval is the delay between failed passes. The default is five seconds.
	Interval time.Duration
}

// Verification identifies the validated artifact observed on the public origin.
type Verification struct {
	Origin         string `json:"origin"`
	ManifestDigest string `json:"manifestDigest"`
	Pages          int    `json:"pages"`
	Assets         int    `json:"assets"`
	Redirects      int    `json:"redirects"`
	Attempts       int    `json:"attempts"`
}

type publicationManifest struct {
	SchemaVersion string `json:"schemaVersion"`
	Digest        string `json:"digest"`
	Policy        string `json:"policy,omitempty"`
	site.SiteManifest
	Entries []struct{ Path, Digest string } `json:"entries"`
}
type publishedFile struct {
	name string
	data []byte
}
type publicationVerification struct {
	engine   *engine
	manifest []byte
	notFound []byte
	files    []publishedFile
	report   Verification
	client   *http.Client
}

// Verify checks an already-built root/public against the configured public origin.
// It never builds or publishes. Non-HTML assets must match exactly; HTML must
// retain content, canonical, locale, JSON-LD and head metadata despite hosting injections.
// The remote manifest must match before and after each complete pass.
func Verify(ctx context.Context, root string, options VerifyOptions) (Verification, error) {
	if err := ctx.Err(); err != nil {
		return Verification{}, err
	}
	if options.Timeout < 0 || options.Interval < 0 {
		return Verification{}, fmt.Errorf("verification durations must not be negative")
	}
	if options.Timeout == 0 {
		options.Timeout = 2 * time.Minute
	}
	if options.Interval == 0 {
		options.Interval = 5 * time.Second
	}
	verifier, err := loadPublicationVerification(root)
	if err != nil {
		return Verification{}, err
	}
	if options.Client != nil {
		client := *options.Client
		client.CheckRedirect = verifier.client.CheckRedirect
		if client.Timeout <= 0 {
			client.Timeout = 15 * time.Second
		}
		verifier.client = &client
	}
	ctx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	var last error
	for {
		if err := ctx.Err(); err != nil {
			return verifier.report, fmt.Errorf("publication verification stopped: %w; last failure: %v", err, last)
		}
		verifier.report.Attempts++
		if err := verifier.check(ctx); err == nil {
			return verifier.report, nil
		} else {
			last = err
		}
		timer := time.NewTimer(options.Interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return verifier.report, fmt.Errorf("publication verification stopped: %w; last failure: %v", ctx.Err(), last)
		case <-timer.C:
		}
	}
}

func loadPublicationVerification(root string) (*publicationVerification, error) {
	e, err := loadEngine(root)
	if err != nil {
		return nil, err
	}
	public, err := os.OpenRoot(filepath.Join(root, "public"))
	if err != nil {
		return nil, err
	}
	defer public.Close()
	raw, err := public.ReadFile(site.ManifestPath)
	if err != nil {
		return nil, err
	}
	var manifest publicationManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("invalid publication manifest: %w", err)
	}
	if manifest.SchemaVersion != "margo-site-manifest/v1" || manifest.BaseURL != e.baseURL || len(manifest.Entries) == 0 || len(manifest.Routes) == 0 {
		return nil, fmt.Errorf("publication manifest does not describe this site")
	}
	result := site.Result{Site: manifest.SiteManifest}
	verifier := &publicationVerification{engine: e, manifest: raw, report: Verification{Origin: e.baseURL, ManifestDigest: manifest.Digest}, client: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	for _, entry := range manifest.Entries {
		if !fs.ValidPath(entry.Path) || entry.Path == site.ManifestPath {
			return nil, fmt.Errorf("invalid publication artifact path %q", entry.Path)
		}
		data, err := public.ReadFile(entry.Path)
		if err != nil {
			return nil, err
		}
		result.Artifacts = append(result.Artifacts, site.Artifact{Path: entry.Path, Content: data})
		if entry.Path == "_headers" || entry.Path == "_redirects" || entry.Path == ".assetsignore" {
			continue
		}
		verifier.files = append(verifier.files, publishedFile{entry.Path, data})
		if entry.Path == "404.html" {
			verifier.notFound = data
		}
		if strings.HasSuffix(entry.Path, ".html") {
			page, err := readHTML(data)
			if err != nil {
				return nil, err
			}
			if page.Canonical != e.baseURL+publicRoute(entry.Path) && len(page.Schemas) == 0 {
				verifier.report.Redirects++
			} else {
				verifier.report.Pages++
			}
		} else {
			verifier.report.Assets++
		}
	}
	// Reuse Margo's manifest serializer and digest algorithm for local integrity.
	canonical, err := site.MarshalManifest(result, manifest.Policy)
	if err != nil {
		return nil, err
	}
	var recomputed publicationManifest
	if err := json.Unmarshal(canonical, &recomputed); err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(manifest, recomputed) {
		return nil, fmt.Errorf("local publication artifacts differ from their manifest")
	}
	if len(verifier.notFound) == 0 {
		return nil, fmt.Errorf("publication has no 404 artifact")
	}
	return verifier, nil
}

func (v *publicationVerification) fetch(ctx context.Context, route string, limit int64) ([]byte, *http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, v.engine.baseURL+route, nil)
	if err != nil {
		return nil, nil, err
	}
	request.Header.Set("User-Agent", "Gazeta publication verifier")
	request.Header.Set("Cache-Control", "no-cache")
	response, err := v.client.Do(request)
	if err != nil {
		return nil, nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, response, err
	}
	if int64(len(data)) > limit {
		return nil, response, fmt.Errorf("response exceeds expected artifact size")
	}
	return data, response, nil
}
func (v *publicationVerification) checkManifest(ctx context.Context) error {
	data, response, err := v.fetch(ctx, "/"+site.ManifestPath, int64(len(v.manifest)))
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK || !bytes.Equal(data, v.manifest) {
		return fmt.Errorf("public manifest does not match the validated artifact")
	}
	return nil
}
func (v *publicationVerification) check(ctx context.Context) error {
	if err := v.checkManifest(ctx); err != nil {
		return err
	}
	// A bounded worker group checks the entire artifact without unbounded requests.
	jobs := make(chan publishedFile)
	var workers sync.WaitGroup
	var failure error
	var once sync.Once
	for range 6 {
		workers.Go(func() {
			for file := range jobs {
				if err := v.checkFile(ctx, file); err != nil {
					once.Do(func() { failure = fmt.Errorf("%s: %w", file.name, err) })
				}
			}
		})
	}
	for _, file := range v.files {
		select {
		case jobs <- file:
		case <-ctx.Done():
			close(jobs)
			workers.Wait()
			return ctx.Err()
		}
	}
	close(jobs)
	workers.Wait()
	if failure != nil {
		return failure
	}
	// Test the HTTP status separately from the directly addressable 404 artifact.
	data, response, err := v.fetch(ctx, "/.well-known/gazeta-missing-"+v.report.ManifestDigest, int64(2<<20))
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusNotFound {
		return fmt.Errorf("missing route returned HTTP %d instead of 404", response.StatusCode)
	}
	expected, err := readHTML(v.notFound)
	if err != nil {
		return err
	}
	page, err := readHTML(data)
	if err != nil {
		return err
	}
	if err := samePublicationHead(expected, page); err != nil {
		return fmt.Errorf("missing route: %w", err)
	}
	if err := samePublishedHTML(v.notFound, data); err != nil {
		return fmt.Errorf("missing route: %w", err)
	}
	return v.checkManifest(ctx)
}
func (v *publicationVerification) checkFile(ctx context.Context, file publishedFile) error {
	route := "/" + file.name
	isHTML := strings.HasSuffix(file.name, ".html")
	if isHTML {
		route = publicRoute(file.name)
	}
	data, response, err := v.fetch(ctx, route, int64(len(file.data))+(1<<20))
	if err != nil {
		return err
	}
	if isHTML {
		expected, err := readHTML(file.data)
		if err != nil {
			return err
		}
		if expected.Canonical != v.engine.baseURL+route && len(expected.Schemas) == 0 {
			location, err := response.Location()
			if err != nil || (response.StatusCode != 301 && response.StatusCode != 308) || location.String() != expected.Canonical {
				return fmt.Errorf("legacy redirect differs from its configured target")
			}
			return nil
		}
		// Static hosts may remove .html from standalone-file URLs. Follow only
		// that exact same-origin normalization, never arbitrary redirects.
		if strings.HasSuffix(route, ".html") && (response.StatusCode == 301 || response.StatusCode == 302 || response.StatusCode == 307 || response.StatusCode == 308) {
			location, locationErr := response.Location()
			clean := strings.TrimSuffix(route, ".html")
			if locationErr == nil && location.String() == v.engine.baseURL+clean {
				data, response, err = v.fetch(ctx, clean, int64(len(file.data))+(1<<20))
				if err != nil {
					return err
				}
			}
		}
		if response.StatusCode != 200 {
			return fmt.Errorf("HTML returned HTTP %d", response.StatusCode)
		}
		media, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
		if media != "text/html" {
			return fmt.Errorf("HTML has incorrect Content-Type %q", media)
		}
		actual, err := readHTML(data)
		if err != nil {
			return err
		}
		if err := samePublicationHead(expected, actual); err != nil {
			return err
		}
		return samePublishedHTML(file.data, data)
	}
	if response.StatusCode != 200 || !bytes.Equal(data, file.data) {
		return fmt.Errorf("public asset differs from the validated artifact (HTTP %d)", response.StatusCode)
	}
	media, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	switch strings.ToLower(filepath.Ext(file.name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ico":
		if !strings.HasPrefix(media, "image/") {
			return fmt.Errorf("image has incorrect Content-Type %q", media)
		}
	case ".css":
		if media != "text/css" {
			return fmt.Errorf("CSS has incorrect Content-Type %q", media)
		}
	case ".js":
		if media != "text/javascript" && media != "application/javascript" {
			return fmt.Errorf("JavaScript has incorrect Content-Type %q", media)
		}
	}
	return nil
}
func samePublicationHead(expected, actual htmlPage) error {
	if expected.Canonical != actual.Canonical || expected.CanonicalCount != actual.CanonicalCount || actual.CanonicalCount != 1 {
		return fmt.Errorf("canonical differs from the validated page")
	}
	if expected.Language != actual.Language {
		return fmt.Errorf("document language differs from the validated page")
	}
	if len(expected.Schemas) != len(actual.Schemas) || expected.HeadSchemas != actual.HeadSchemas {
		return fmt.Errorf("JSON-LD count or head placement differs from the validated page")
	}
	for i := range expected.Schemas {
		var want, got any
		if err := json.Unmarshal(expected.Schemas[i], &want); err != nil {
			return err
		}
		if err := json.Unmarshal(actual.Schemas[i], &got); err != nil {
			return fmt.Errorf("invalid public JSON-LD: %w", err)
		}
		if !reflect.DeepEqual(want, got) {
			return fmt.Errorf("public JSON-LD differs from the validated page")
		}
	}
	if !reflect.DeepEqual(expected.HeadMetadata, actual.HeadMetadata) {
		return fmt.Errorf("head metadata differs from the validated page")
	}
	return nil
}
