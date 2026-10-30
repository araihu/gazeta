package gazeta

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"mime"
	"net/http"
	"os"
	"time"
)

// Run implements build, check, verify and serve for a thin consumer command.
// An optional second argument selects the publication root.
func Run(args []string) error {
	if len(args) > 0 && args[0] == "verify" {
		flags := flag.NewFlagSet("verify", flag.ContinueOnError)
		timeout := flags.Duration("timeout", 2*time.Minute, "total publication verification deadline")
		interval := flags.Duration("interval", 5*time.Second, "delay between failed verification passes")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() > 1 {
			return fmt.Errorf("usage: gazeta verify [-timeout 2m] [-interval 5s] [root]")
		}
		root := "."
		if flags.NArg() == 1 {
			root = flags.Arg(0)
		}
		report, err := Verify(context.Background(), root, VerifyOptions{Timeout: *timeout, Interval: *interval})
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(report)
	}
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("usage: gazeta build|check|serve [root], or gazeta verify [flags] [root]")
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
