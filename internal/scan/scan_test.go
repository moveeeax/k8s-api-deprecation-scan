package scan

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func mustVersion(t *testing.T, s string) Version {
	t.Helper()
	v, err := ParseVersion(s)
	if err != nil {
		t.Fatalf("ParseVersion(%q): %v", s, err)
	}
	return v
}

func scanPath(t *testing.T, target, path string) *Result {
	t.Helper()
	res, err := New(mustVersion(t, target)).ScanPaths([]string{path}, nil)
	if err != nil {
		t.Fatalf("ScanPaths(%q): %v", path, err)
	}
	return res
}

// The acceptance criterion from the spec: the same PodDisruptionBudget is a
// hard failure on 1.25 and only a warning on 1.20.
func TestPodDisruptionBudgetLevelDependsOnTarget(t *testing.T) {
	path := filepath.Join("testdata", "pdb.yaml")

	at125 := scanPath(t, "1.25", path)
	if len(at125.Findings) != 1 {
		t.Fatalf("target 1.25: got %d findings, want 1", len(at125.Findings))
	}
	got := at125.Findings[0]
	if got.Level != LevelRemoved {
		t.Errorf("target 1.25: level = %s, want %s", got.Level, LevelRemoved)
	}
	if got.Replacement != "policy/v1" {
		t.Errorf("replacement = %q, want policy/v1", got.Replacement)
	}
	if got.Name != "web" || got.Kind != "PodDisruptionBudget" {
		t.Errorf("got %s/%s, want PodDisruptionBudget/web", got.Kind, got.Name)
	}

	at120 := scanPath(t, "1.20", path)
	if len(at120.Findings) != 1 {
		t.Fatalf("target 1.20: got %d findings, want 1", len(at120.Findings))
	}
	if at120.Findings[0].Level != LevelDeprecated {
		t.Errorf("target 1.20: level = %s, want %s", at120.Findings[0].Level, LevelDeprecated)
	}
}

// A later target than the removal release is still REMOVED.
func TestRemovalStaysRemovedOnNewerTargets(t *testing.T) {
	res := scanPath(t, "1.31", filepath.Join("testdata", "pdb.yaml"))
	if res.Removed() != 1 {
		t.Fatalf("target 1.31: removed = %d, want 1", res.Removed())
	}
}

func TestMultiDocumentReportsEachOffender(t *testing.T) {
	res := scanPath(t, "1.25", filepath.Join("testdata", "bundle.yaml"))
	if len(res.Findings) != 2 {
		t.Fatalf("got %d findings, want 2: %+v", len(res.Findings), res.Findings)
	}
	// apps/v1 Deployment at index 0 is fine and must not be reported.
	cronjob, ingress := res.Findings[0], res.Findings[1]
	if cronjob.DocIndex != 1 || cronjob.Kind != "CronJob" {
		t.Errorf("first finding = doc %d %s, want doc 1 CronJob", cronjob.DocIndex, cronjob.Kind)
	}
	if cronjob.Level != LevelRemoved {
		t.Errorf("batch/v1beta1 CronJob at 1.25: level = %s, want REMOVED", cronjob.Level)
	}
	if ingress.DocIndex != 2 || ingress.Kind != "Ingress" {
		t.Errorf("second finding = doc %d %s, want doc 2 Ingress", ingress.DocIndex, ingress.Kind)
	}
	for _, f := range res.Findings {
		if !strings.HasSuffix(f.File, "bundle.yaml") {
			t.Errorf("file = %q, want it to name the fixture", f.File)
		}
	}
}

func TestNonKubernetesYAMLIsIgnored(t *testing.T) {
	res := scanPath(t, "1.29", filepath.Join("testdata", "not-kubernetes.yaml"))
	if len(res.Findings) != 0 {
		t.Fatalf("got %d findings, want 0: %+v", len(res.Findings), res.Findings)
	}
	if res.DocsScanned != 0 {
		t.Errorf("DocsScanned = %d, want 0 (nothing in the file is a k8s object)", res.DocsScanned)
	}
}

func TestDirectoryWalkCollectsEveryFile(t *testing.T) {
	res := scanPath(t, "1.29", "testdata")
	if res.FilesScanned != 3 {
		t.Errorf("FilesScanned = %d, want 3", res.FilesScanned)
	}
	if res.Removed() != 3 {
		t.Errorf("removed = %d, want 3 (pdb + cronjob + ingress)", res.Removed())
	}
	// Findings must come back sorted by file, then document index.
	for i := 1; i < len(res.Findings); i++ {
		a, b := res.Findings[i-1], res.Findings[i]
		if a.File > b.File || (a.File == b.File && a.DocIndex > b.DocIndex) {
			t.Fatalf("findings not sorted: %v before %v", a, b)
		}
	}
}

func TestStdinStream(t *testing.T) {
	in := strings.NewReader("apiVersion: autoscaling/v2beta1\nkind: HorizontalPodAutoscaler\nmetadata:\n  name: web\n")
	res, err := New(mustVersion(t, "1.26")).ScanPaths([]string{"-"}, in)
	if err != nil {
		t.Fatalf("ScanPaths: %v", err)
	}
	if len(res.Findings) != 1 || res.Findings[0].Level != LevelRemoved {
		t.Fatalf("got %+v, want one REMOVED finding", res.Findings)
	}
	if res.Findings[0].File != "<stdin>" {
		t.Errorf("file = %q, want <stdin>", res.Findings[0].File)
	}
}

func TestCleanManifestProducesNothing(t *testing.T) {
	in := strings.NewReader("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: app\ndata:\n  k: v\n")
	res, err := New(mustVersion(t, "1.30")).ScanPaths([]string{"-"}, in)
	if err != nil {
		t.Fatalf("ScanPaths: %v", err)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("got %+v, want no findings", res.Findings)
	}
	if res.DocsScanned != 1 {
		t.Errorf("DocsScanned = %d, want 1", res.DocsScanned)
	}
}

func TestMissingPathIsAnError(t *testing.T) {
	if _, err := New(mustVersion(t, "1.29")).ScanPaths([]string{"testdata/nope.yaml"}, nil); err == nil {
		t.Fatal("want an error for a missing path")
	}
}

func TestTextReport(t *testing.T) {
	res := scanPath(t, "1.25", filepath.Join("testdata", "pdb.yaml"))
	var buf bytes.Buffer
	if err := WriteText(&buf, res); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"REMOVED", "PodDisruptionBudget/web", "policy/v1beta1", "-> policy/v1", "removed 1.25"} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q:\n%s", want, out)
		}
	}
}

func TestTextReportWhenClean(t *testing.T) {
	res := scanPath(t, "1.20", filepath.Join("testdata", "not-kubernetes.yaml"))
	var buf bytes.Buffer
	if err := WriteText(&buf, res); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	if !strings.HasPrefix(buf.String(), "ok:") {
		t.Errorf("want an ok: line, got %q", buf.String())
	}
}

func TestJSONReport(t *testing.T) {
	res := scanPath(t, "1.22", filepath.Join("testdata", "bundle.yaml"))
	var buf bytes.Buffer
	if err := WriteJSON(&buf, res); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	var got struct {
		TargetVersion string    `json:"targetVersion"`
		Findings      []Finding `json:"findings"`
		Summary       struct {
			Removed    int `json:"removed"`
			Deprecated int `json:"deprecated"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, buf.String())
	}
	if got.TargetVersion != "1.22" {
		t.Errorf("targetVersion = %q, want 1.22", got.TargetVersion)
	}
	// At 1.22 the Ingress is gone but the CronJob (removed 1.25) is only a warning.
	if got.Summary.Removed != 1 || got.Summary.Deprecated != 1 {
		t.Errorf("summary = %+v, want 1 removed / 1 deprecated", got.Summary)
	}
}

func TestBuiltinTableIsConsistent(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range BuiltinTable().Rules() {
		key := r.APIVersion + "|" + r.Kind
		if seen[key] {
			t.Errorf("duplicate rule for %s", key)
		}
		seen[key] = true

		dep, err := ParseVersion(r.DeprecatedIn)
		if err != nil {
			t.Errorf("%s: bad deprecatedIn %q", key, r.DeprecatedIn)
			continue
		}
		if r.RemovedIn == "" {
			continue
		}
		rem, err := ParseVersion(r.RemovedIn)
		if err != nil {
			t.Errorf("%s: bad removedIn %q", key, r.RemovedIn)
			continue
		}
		if !rem.AtLeast(dep) {
			t.Errorf("%s: removed %s is before deprecated %s", key, r.RemovedIn, r.DeprecatedIn)
		}
		if r.Replacement == r.APIVersion {
			t.Errorf("%s: replacement points at itself", key)
		}
	}
	if len(seen) < 40 {
		t.Errorf("table has only %d rules, that cannot be right", len(seen))
	}
}

func TestLookupMissesUnknownAPI(t *testing.T) {
	if _, ok := BuiltinTable().Lookup("apps/v1", "Deployment"); ok {
		t.Error("apps/v1 Deployment must not be in the table")
	}
	if _, ok := BuiltinTable().Lookup("policy/v1beta1", "PodDisruptionBudget"); !ok {
		t.Error("policy/v1beta1 PodDisruptionBudget must be in the table")
	}
}
