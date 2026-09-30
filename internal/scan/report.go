package scan

import (
	"encoding/json"
	"fmt"
	"io"
)

// WriteText prints a human-readable report.
func WriteText(w io.Writer, res *Result) error {
	for _, f := range res.Findings {
		target := f.Kind
		if f.Name != "" {
			target += "/" + f.Name
		}
		to := f.Replacement
		if to == "" {
			to = "no replacement"
		}
		when := "deprecated " + f.DeprecatedIn
		if f.RemovedIn != "" {
			when += ", removed " + f.RemovedIn
		}
		if _, err := fmt.Fprintf(w, "%-10s %s:%d %s %s -> %s (%s)\n",
			f.Level, f.File, f.DocIndex, target, f.APIVersion, to, when); err != nil {
			return err
		}
	}
	removed, deprecated := res.Removed(), res.Deprecated()
	scanned := fmt.Sprintf("%s in %s", plural(res.DocsScanned, "document"), plural(res.FilesScanned, "file"))
	if removed == 0 && deprecated == 0 {
		_, err := fmt.Fprintf(w, "ok: %s, nothing retiring at %s\n", scanned, res.TargetVersion)
		return err
	}
	_, err := fmt.Fprintf(w, "\n%d removed, %d deprecated at %s (%s)\n",
		removed, deprecated, res.TargetVersion, scanned)
	return err
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// WriteJSON prints the machine-readable report.
func WriteJSON(w io.Writer, res *Result) error {
	payload := struct {
		*Result
		Summary struct {
			Removed    int `json:"removed"`
			Deprecated int `json:"deprecated"`
		} `json:"summary"`
	}{Result: res}
	payload.Summary.Removed = res.Removed()
	payload.Summary.Deprecated = res.Deprecated()

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

// WriteRulesJSON dumps the deprecation table itself, so the check is auditable.
func WriteRulesJSON(w io.Writer, t *Table) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(t.Rules())
}
