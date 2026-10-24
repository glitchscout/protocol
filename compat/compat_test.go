package compat

import (
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/glitchscout/protocol"
)

// snapshot copies protocol.FS so tests can edit a "head" version.
func snapshot(t *testing.T) fstest.MapFS {
	t.Helper()
	m := fstest.MapFS{}
	err := fs.WalkDir(protocol.FS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(protocol.FS, p)
		m[p] = &fstest.MapFile{Data: b}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// edit decodes the JSON file at p, applies f, and writes it back.
func edit(t *testing.T, m fstest.MapFS, p string, f func(map[string]any)) {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(m[p].Data, &doc); err != nil {
		t.Fatal(err)
	}
	f(doc)
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	m[p] = &fstest.MapFile{Data: b}
}

func obj(m map[string]any, path ...string) map[string]any {
	for _, k := range path {
		m = m[k].(map[string]any)
	}
	return m
}

func mustCheck(t *testing.T, base, head fs.FS) []string {
	t.Helper()
	problems, err := Check(base, head)
	if err != nil {
		t.Fatal(err)
	}
	return problems
}

func wantProblem(t *testing.T, problems []string, substr string) {
	t.Helper()
	for _, p := range problems {
		if strings.Contains(p, substr) {
			return
		}
	}
	t.Fatalf("problems %q: want one mentioning %q", problems, substr)
}

func TestIdenticalIsCompatible(t *testing.T) {
	if p := mustCheck(t, snapshot(t), snapshot(t)); len(p) != 0 {
		t.Fatalf("problems = %q, want none", p)
	}
}

func TestNarrowingIsIncompatible(t *testing.T) {
	head := snapshot(t)
	edit(t, head, "schemas/messages/step.schema.json", func(d map[string]any) {
		obj(d, "$defs", "payload", "properties", "ticks")["maximum"] = 10
	})
	wantProblem(t, mustCheck(t, snapshot(t), head), "fixtures/valid/STEP/basic.json")
}

func TestRemovedMessageTypeIsIncompatible(t *testing.T) {
	head := snapshot(t)
	edit(t, head, "schemas/envelope.schema.json", func(d map[string]any) {
		mt := obj(d, "properties", "message_type")
		var kept []any
		for _, v := range mt["enum"].([]any) {
			if v != "GOODBYE" {
				kept = append(kept, v)
			}
		}
		mt["enum"] = kept
	})
	delete(head, "schemas/messages/goodbye.schema.json")
	edit(t, head, "schemas/message.schema.json", func(d map[string]any) {
		var kept []any
		for _, r := range d["oneOf"].([]any) {
			if !strings.Contains(r.(map[string]any)["$ref"].(string), "goodbye") {
				kept = append(kept, r)
			}
		}
		d["oneOf"] = kept
	})
	wantProblem(t, mustCheck(t, snapshot(t), head), "GOODBYE")
}

func TestVersionBumpAllowsBreakingChanges(t *testing.T) {
	head := snapshot(t)
	edit(t, head, "schemas/envelope.schema.json", func(d map[string]any) {
		obj(d, "properties", "protocol")["const"] = "glitchscout/2"
	})
	edit(t, head, "schemas/messages/step.schema.json", func(d map[string]any) {
		obj(d, "$defs", "payload", "properties", "ticks")["maximum"] = 10
	})
	if p := mustCheck(t, snapshot(t), head); len(p) != 0 {
		t.Fatalf("problems = %q, want none after a version bump", p)
	}
}

func TestWideningIsCompatible(t *testing.T) {
	head := snapshot(t)
	edit(t, head, "schemas/messages/step.schema.json", func(d map[string]any) {
		obj(d, "$defs", "payload", "properties")["reason"] = map[string]any{"type": "string"}
	})
	if p := mustCheck(t, snapshot(t), head); len(p) != 0 {
		t.Fatalf("problems = %q, want none", p)
	}
}
