package protocol

import (
	"bytes"
	"io/fs"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// evidenceSchemas are the evidence formats of protocol v1 (schemas/evidence/<name>.schema.json).
var evidenceSchemas = []string{"run", "result", "trace_line", "evidence_index"}

func evidenceExpectations(t *testing.T) map[string]string {
	t.Helper()
	doc := readJSON(t, "fixtures/evidence/invalid/expectations.json").(map[string]any)
	out := map[string]string{}
	for k, v := range doc["fixtures"].(map[string]any) {
		out[k] = v.(string)
	}
	return out
}

func TestEvidenceFixtures(t *testing.T) {
	c := newCompiler(t)
	exp := evidenceExpectations(t)
	seen := map[string]bool{}
	for _, name := range evidenceSchemas {
		sch := compile(t, c, "schemas/evidence/"+name+".schema.json")
		valid := fixtureFiles(t, "evidence/valid", name)
		if len(valid) == 0 {
			t.Errorf("%s: no valid fixtures", name)
		}
		for _, f := range valid {
			if err := sch.Validate(readJSON(t, f)); err != nil {
				t.Errorf("%s rejected: %v", f, err)
			}
		}
		lines, _ := fs.Glob(FS, path.Join("fixtures/evidence/valid", name, "*.jsonl"))
		for _, f := range lines {
			b, err := FS.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			for i, line := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
				doc, err := jsonschema.UnmarshalJSON(bytes.NewReader([]byte(line)))
				if err != nil {
					t.Fatalf("%s:%d: %v", f, i+1, err)
				}
				if err := sch.Validate(doc); err != nil {
					t.Errorf("%s:%d rejected: %v", f, i+1, err)
				}
			}
		}
		invalid := fixtureFiles(t, "evidence/invalid", name)
		if len(invalid) == 0 {
			t.Errorf("%s: no invalid fixtures", name)
		}
		for _, f := range invalid {
			key := strings.TrimPrefix(f, "fixtures/evidence/invalid/")
			seen[key] = true
			want, ok := exp[key]
			if !ok {
				t.Errorf("%s: no entry in fixtures/evidence/invalid/expectations.json", f)
				continue
			}
			err := sch.Validate(readJSON(t, f))
			if err == nil {
				t.Errorf("%s accepted", f)
				continue
			}
			if locs := leafLocations(err); !slices.Contains(locs, want) {
				t.Errorf("%s rejected at %q, want at %q", f, locs, want)
			}
		}
	}
	for key := range exp {
		if !seen[key] {
			t.Errorf("expectations.json lists %s, which isn't an invalid fixture", key)
		}
	}
}

// Every schema under schemas/evidence is in evidenceSchemas, so none goes untested.
func TestEveryEvidenceSchemaIsTested(t *testing.T) {
	files, err := fs.Glob(FS, "schemas/evidence/*.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if name := strings.TrimSuffix(path.Base(f), ".schema.json"); !slices.Contains(evidenceSchemas, name) {
			t.Errorf("%s isn't in evidenceSchemas", f)
		}
	}
	if len(files) != len(evidenceSchemas) {
		t.Errorf("%d evidence schemas, %d listed", len(files), len(evidenceSchemas))
	}
}
