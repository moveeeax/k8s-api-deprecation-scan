package scan

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Level classifies a finding relative to the target cluster version.
type Level string

const (
	// LevelRemoved means the API server at the target version no longer serves
	// this apiVersion. Applying the manifest fails.
	LevelRemoved Level = "REMOVED"
	// LevelDeprecated means the API still works at the target version but is on
	// its way out.
	LevelDeprecated Level = "DEPRECATED"
)

// Finding is one document that uses a retiring apiVersion.
type Finding struct {
	File         string `json:"file"`
	DocIndex     int    `json:"docIndex"`
	Kind         string `json:"kind"`
	Name         string `json:"name,omitempty"`
	APIVersion   string `json:"apiVersion"`
	Level        Level  `json:"level"`
	DeprecatedIn string `json:"deprecatedIn"`
	RemovedIn    string `json:"removedIn,omitempty"`
	Replacement  string `json:"replacement,omitempty"`
}

// Result is the whole scan.
type Result struct {
	TargetVersion string    `json:"targetVersion"`
	Findings      []Finding `json:"findings"`
	FilesScanned  int       `json:"filesScanned"`
	DocsScanned   int       `json:"docsScanned"`
}

// Removed counts findings the target version will reject outright.
func (r *Result) Removed() int { return r.count(LevelRemoved) }

// Deprecated counts findings that still work at the target version.
func (r *Result) Deprecated() int { return r.count(LevelDeprecated) }

func (r *Result) count(l Level) int {
	n := 0
	for _, f := range r.Findings {
		if f.Level == l {
			n++
		}
	}
	return n
}

// Scanner walks manifests and matches them against a deprecation table.
type Scanner struct {
	Table  *Table
	Target Version
}

// New builds a Scanner against the built-in table.
func New(target Version) *Scanner {
	return &Scanner{Table: BuiltinTable(), Target: target}
}

// stdinName is the file label used for documents read from standard input.
const stdinName = "<stdin>"

// ScanPaths scans every path. A path may be a file, a directory (walked
// recursively for .yaml/.yml) or "-" for standard input.
func (s *Scanner) ScanPaths(paths []string, stdin io.Reader) (*Result, error) {
	res := &Result{TargetVersion: s.Target.String(), Findings: []Finding{}}
	for _, p := range paths {
		if p == "-" {
			if err := s.scanReader(stdinName, stdin, res); err != nil {
				return nil, err
			}
			continue
		}
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			if err := s.scanFile(p, res); err != nil {
				return nil, err
			}
			continue
		}
		err = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if isIgnoredDir(d.Name()) && path != p {
					return fs.SkipDir
				}
				return nil
			}
			if !hasYAMLExt(path) {
				return nil
			}
			return s.scanFile(path, res)
		})
		if err != nil {
			return nil, err
		}
	}
	sort.SliceStable(res.Findings, func(i, j int) bool {
		if res.Findings[i].File != res.Findings[j].File {
			return res.Findings[i].File < res.Findings[j].File
		}
		return res.Findings[i].DocIndex < res.Findings[j].DocIndex
	})
	return res, nil
}

func isIgnoredDir(name string) bool {
	return name == ".git" || name == "vendor" || name == "node_modules"
}

func hasYAMLExt(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return true
	}
	return false
}

func (s *Scanner) scanFile(path string, res *Result) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return s.scanReader(path, f, res)
}

// header is the minimum every Kubernetes object carries.
type header struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
}

// scanReader decodes a multi-document YAML stream. Documents that are not
// Kubernetes objects are skipped without complaint: the whole point is to be
// able to point the tool at a directory that also holds CI config and charts.
func (s *Scanner) scanReader(name string, r io.Reader, res *Result) error {
	res.FilesScanned++
	dec := yaml.NewDecoder(r)
	for idx := 0; ; idx++ {
		var node yaml.Node
		err := dec.Decode(&node)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			// Malformed YAML somewhere in the stream. Everything after it is
			// unreadable too, so stop on this file rather than fail the run.
			return nil
		}
		var h header
		if node.Decode(&h) != nil {
			continue
		}
		if h.APIVersion == "" || h.Kind == "" {
			continue
		}
		res.DocsScanned++
		rule, ok := s.Table.Lookup(h.APIVersion, h.Kind)
		if !ok {
			continue
		}
		level, ok := s.classify(rule)
		if !ok {
			continue
		}
		res.Findings = append(res.Findings, Finding{
			File:         name,
			DocIndex:     idx,
			Kind:         h.Kind,
			Name:         h.Metadata.Name,
			APIVersion:   h.APIVersion,
			Level:        level,
			DeprecatedIn: rule.DeprecatedIn,
			RemovedIn:    rule.RemovedIn,
			Replacement:  rule.Replacement,
		})
	}
}

// classify decides how bad a rule is at the target version.
//
// Anything in the table is reported: if an API is scheduled for removal it is
// worth knowing about even on a cluster old enough that the deprecation has not
// landed yet, because the manifest is what you carry into the next upgrade.
// Only the reached-removal case is an error.
func (s *Scanner) classify(r Rule) (Level, bool) {
	if r.RemovedIn != "" {
		if removed, err := ParseVersion(r.RemovedIn); err == nil && s.Target.AtLeast(removed) {
			return LevelRemoved, true
		}
	}
	return LevelDeprecated, true
}
