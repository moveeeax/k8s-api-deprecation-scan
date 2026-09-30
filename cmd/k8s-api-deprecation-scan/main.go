// Command k8s-api-deprecation-scan reports Kubernetes manifests that use an
// apiVersion deprecated or removed at a target cluster version.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/moveeeax/k8s-api-deprecation-scan/internal/scan"
)

// version is the CLI version, overridable at build time with
// -ldflags "-X main.version=...".
var version = "0.1.0"

const usage = `k8s-api-deprecation-scan --target-version VERSION [flags] PATH...

Scans YAML manifests for apiVersions deprecated or removed at a target cluster
version. PATH may be a file, a directory (walked for .yaml/.yml) or "-" to read
a manifest stream from stdin.

Exit status: 1 if any document uses an apiVersion already removed at the target
version, 0 otherwise, 2 on a usage or I/O error.

Flags:
`

func main() {
	os.Exit(run())
}

func run() int {
	fs := flag.NewFlagSet("k8s-api-deprecation-scan", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		fs.PrintDefaults()
	}
	target := fs.String("target-version", "", "target cluster version, e.g. 1.29 (required)")
	format := fs.String("format", "text", "output format: text or json")
	listRules := fs.Bool("list-rules", false, "print the built-in deprecation table as JSON and exit")
	showVersion := fs.Bool("version", false, "print the version and exit")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Println(version)
		return 0
	}
	if *listRules {
		return dumpRules()
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(os.Stderr, "error: --format must be text or json, got %q\n", *format)
		return 2
	}
	if *target == "" {
		fmt.Fprintln(os.Stderr, "error: --target-version is required")
		fs.Usage()
		return 2
	}
	v, err := scan.ParseVersion(*target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	paths := fs.Args()
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "error: no paths given (use \"-\" to read stdin)")
		return 2
	}

	res, err := scan.New(v).ScanPaths(paths, os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}

	if *format == "json" {
		err = scan.WriteJSON(os.Stdout, res)
	} else {
		err = scan.WriteText(os.Stdout, res)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	if res.Removed() > 0 {
		return 1
	}
	return 0
}

func dumpRules() int {
	if err := scan.WriteRulesJSON(os.Stdout, scan.BuiltinTable()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	return 0
}
