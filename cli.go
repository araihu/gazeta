package gazeta

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"time"
)

// Run implements build, check and serve for a thin consumer command.
// An optional second argument selects the publication root.
func Run(args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("usage: blog build|check|serve [root]")
	}
	root := "."
	if len(args) == 2 {
		root = args[1]
	}
	switch args[0] {
	case "build":
		return Build(context.Background(), root, time.Now())
	case "check":
		return Check(root, time.Now())
	case "serve":
		if _, err := loadEngine(root); err != nil {
			return err
		}
		if err := mime.AddExtensionType(".webmanifest", "application/manifest+json"); err != nil {
			return err
		}
		fmt.Println("Preview: http://127.0.0.1:1313")
		server := &http.Server{Addr: "127.0.0.1:1313", Handler: http.FileServer(http.Dir(root + "/public")), ReadHeaderTimeout: 5 * time.Second}
		return server.ListenAndServe()
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
