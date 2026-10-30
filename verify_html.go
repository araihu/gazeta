package gazeta

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// Normalize only the known Cloudflare response injection and JSON object formatting.
// This changes comparison trees, never publication files or served HTML.
func samePublishedHTML(expected, actual []byte) error {
	normalize := func(data []byte) (string, error) {
		root, err := html.Parse(bytes.NewReader(data))
		if err != nil {
			return "", err
		}
		var walk func(*html.Node) error
		walk = func(node *html.Node) error {
			for child := node.FirstChild; child != nil; {
				next := child.NextSibling
				if child.Type == html.ElementNode && child.Data == "script" && child.FirstChild != nil {
					text := child.FirstChild.Data
					if node.Data == "body" && len(child.Attr) == 0 && strings.Contains(text, "__CF$cv$params") && strings.Contains(text, "/cdn-cgi/challenge-platform/") {
						node.RemoveChild(child)
						child = next
						continue
					}
					for _, attr := range child.Attr {
						if attr.Key == "type" && attr.Val == "application/ld+json" {
							value, err := uniqueJSON([]byte(text))
							if err != nil {
								return err
							}
							normalized, err := json.Marshal(value)
							if err != nil {
								return err
							}
							child.FirstChild.Data = string(normalized)
						}
					}
				}
				if err := walk(child); err != nil {
					return err
				}
				child = next
			}
			return nil
		}
		if err := walk(root); err != nil {
			return "", err
		}
		var output bytes.Buffer
		if err := html.Render(&output, root); err != nil {
			return "", err
		}
		return output.String(), nil
	}
	want, err := normalize(expected)
	if err != nil {
		return err
	}
	got, err := normalize(actual)
	if err != nil {
		return err
	}
	if want != got {
		return fmt.Errorf("public HTML content differs from the validated artifact")
	}
	return nil
}

func uniqueJSON(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var read func() error
	read = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch token {
		case json.Delim('{'):
			keys := map[string]bool{}
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || keys[name] {
					return fmt.Errorf("duplicate or invalid JSON-LD property %q", key)
				}
				keys[name] = true
				if err := read(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		case json.Delim('['):
			for decoder.More() {
				if err := read(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		}
		return nil
	}
	if err := read(); err != nil {
		return nil, err
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return value, nil
}
