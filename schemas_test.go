package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const metaSchemaURL = "https://json-schema.org/draft/2020-12/schema"

// messageTypes is every message type in protocol v1.
var messageTypes = []string{
	"HELLO", "HELLO_ACK", "OBSERVE", "OBSERVATION", "STATE_UPDATE", "EVENT", "ACTION_INJECT",
	"UI_ACTION", "STEP", "COMMAND", "STATE_CTRL", "QUERY", "QUERY_REPLY", "CAPTURE", "ACK",
	"HEARTBEAT", "ERROR", "GOODBYE",
}

func schemaPaths(t *testing.T) []string {
	t.Helper()
	var paths []string
	err := fs.WalkDir(FS, "schemas", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".schema.json") {
			paths = append(paths, p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

func readJSON(t *testing.T, p string) any {
	t.Helper()
	b, err := FS.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("%s: %v", p, err)
	}
	return doc
}

func schemaURL(p string) string { return SchemaURL(p) }

func messageSchemaPath(messageType string) string { return MessageSchemaPath(messageType) }

func newCompiler(t *testing.T) *jsonschema.Compiler {
	t.Helper()
	c, err := Compiler(FS)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func compile(t *testing.T, c *jsonschema.Compiler, p string) *jsonschema.Schema {
	t.Helper()
	sch, err := c.Compile(schemaURL(p))
	if err != nil {
		t.Fatalf("compile %s: %v", p, err)
	}
	return sch
}

func fixtureFiles(t *testing.T, kind, dir string) []string {
	t.Helper()
	entries, err := fs.ReadDir(FS, path.Join("fixtures", kind, dir))
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			files = append(files, path.Join("fixtures", kind, dir, e.Name()))
		}
	}
	return files
}

func TestSchemasConformToMetaschema(t *testing.T) {
	paths := schemaPaths(t)
	if len(paths) == 0 {
		t.Fatal("no schemas found")
	}
	meta, err := jsonschema.NewCompiler().Compile(metaSchemaURL)
	if err != nil {
		t.Fatal(err)
	}
	c := newCompiler(t)
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			doc := readJSON(t, p)
			if err := meta.Validate(doc); err != nil {
				t.Fatalf("not a valid Draft 2020-12 schema: %v", err)
			}
			m, _ := doc.(map[string]any)
			if m["$schema"] != metaSchemaURL {
				t.Errorf("$schema = %v, want %s", m["$schema"], metaSchemaURL)
			}
			if m["$id"] != schemaURL(p) {
				t.Errorf("$id = %v, want %s", m["$id"], schemaURL(p))
			}
			compile(t, c, p)
		})
	}
}

func TestEveryMessageTypeHasSchemaAndFixtures(t *testing.T) {
	for _, mt := range messageTypes {
		t.Run(mt, func(t *testing.T) {
			if _, err := fs.Stat(FS, messageSchemaPath(mt)); err != nil {
				t.Errorf("missing schema %s", messageSchemaPath(mt))
			}
			if len(fixtureFiles(t, "valid", mt)) == 0 {
				t.Errorf("no valid fixtures in fixtures/valid/%s", mt)
			}
			if len(fixtureFiles(t, "invalid", mt)) == 0 {
				t.Errorf("no invalid fixtures in fixtures/invalid/%s", mt)
			}
		})
	}
}

func TestEnvelopeEnumListsEveryMessageType(t *testing.T) {
	doc := readJSON(t, "schemas/envelope.schema.json").(map[string]any)
	props := doc["properties"].(map[string]any)
	var got []string
	for _, v := range props["message_type"].(map[string]any)["enum"].([]any) {
		got = append(got, v.(string))
	}
	want := slices.Clone(messageTypes)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("envelope message_type enum = %v, want %v", got, want)
	}
}

func TestValidFixtures(t *testing.T) {
	c := newCompiler(t)
	dispatch := compile(t, c, "schemas/message.schema.json")
	for _, mt := range messageTypes {
		own := compile(t, c, messageSchemaPath(mt))
		for _, f := range fixtureFiles(t, "valid", mt) {
			t.Run(f, func(t *testing.T) {
				doc := readJSON(t, f)
				if got := doc.(map[string]any)["message_type"]; got != mt {
					t.Fatalf("message_type = %v, want %s (fixture directory)", got, mt)
				}
				if err := own.Validate(doc); err != nil {
					t.Fatalf("rejected by %s: %v", messageSchemaPath(mt), err)
				}
				if err := dispatch.Validate(doc); err != nil {
					t.Fatalf("rejected by message.schema.json: %v", err)
				}
			})
		}
	}
}

// expectations maps fixtures/invalid/<dir>/<name>.json (relative to fixtures/invalid) to the JSON
// Pointer of the instance location the fixture must be rejected at.
func expectations(t *testing.T) map[string]string {
	t.Helper()
	b, err := FS.ReadFile("fixtures/invalid/expectations.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Fixtures map[string]string `json:"fixtures"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Fixtures
}

// leafLocations returns the instance locations, as JSON Pointers, of the innermost validation errors.
func leafLocations(err error) []string {
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return nil
	}
	var out []string
	var walk func(*jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) == 0 {
			out = append(out, "/"+strings.Join(e.InstanceLocation, "/"))
			if len(e.InstanceLocation) == 0 {
				out[len(out)-1] = ""
			}
			return
		}
		for _, c := range e.Causes {
			walk(c)
		}
	}
	walk(ve)
	return out
}

func TestInvalidFixtures(t *testing.T) {
	c := newCompiler(t)
	dispatch := compile(t, c, "schemas/message.schema.json")
	want := expectations(t)
	dirs := append([]string{"envelope"}, messageTypes...)
	seen := map[string]bool{}
	for _, dir := range dirs {
		for _, f := range fixtureFiles(t, "invalid", dir) {
			rel := strings.TrimPrefix(f, "fixtures/invalid/")
			seen[rel] = true
			t.Run(f, func(t *testing.T) {
				doc := readJSON(t, f)
				ptr, ok := want[rel]
				if !ok {
					t.Fatalf("no entry for %s in fixtures/invalid/expectations.json", rel)
				}
				sch := dispatch
				if dir != "envelope" {
					sch = compile(t, c, messageSchemaPath(dir))
				}
				err := sch.Validate(doc)
				if err == nil {
					t.Fatal("accepted, want rejection")
				}
				if locs := leafLocations(err); !slices.Contains(locs, ptr) {
					t.Errorf("rejected at %q, want a failure at %q:\n%v", locs, ptr, err)
				}
				if err := dispatch.Validate(doc); err == nil {
					t.Error("accepted by message.schema.json, want rejection")
				}
			})
		}
	}
	for rel := range want {
		if !seen[rel] {
			t.Errorf("expectations.json lists %s, which isn't an invalid fixture", rel)
		}
	}
}

func TestFixturesFitTextFrameCap(t *testing.T) {
	const maxTextFrame = 256 * 1024
	checked := 0
	err := fs.WalkDir(FS, "fixtures", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".json") {
			return err
		}
		b, err := FS.ReadFile(p)
		if err != nil {
			return err
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, b); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		if compact.Len() > maxTextFrame {
			t.Errorf("%s is %d bytes compacted, over the %d-byte text-frame cap", p, compact.Len(), maxTextFrame)
		}
		checked++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("no fixtures checked")
	}
}

// Code generators (go-jsonschema) need typed roots and no anyOf for nullable envelope fields.
func TestSchemasAreCodegenFriendly(t *testing.T) {
	env := readJSON(t, "schemas/envelope.schema.json").(map[string]any)
	for name, prop := range env["properties"].(map[string]any) {
		p := prop.(map[string]any)
		if _, ok := p["anyOf"]; ok {
			t.Errorf("envelope.%s uses anyOf; use a type array", name)
		}
		if _, ok := p["type"]; !ok {
			if _, isRef := p["$ref"]; !isRef {
				t.Errorf("envelope.%s has no type", name)
			}
		}
	}
	for _, mt := range messageTypes {
		t.Run(mt, func(t *testing.T) {
			doc := readJSON(t, messageSchemaPath(mt)).(map[string]any)
			if doc["type"] != "object" {
				t.Errorf("root type = %v, want object", doc["type"])
			}
			props := doc["properties"].(map[string]any)
			if ref := props["payload"].(map[string]any)["$ref"]; ref != "#/$defs/payload" {
				t.Errorf(`properties.payload.$ref = %v, want "#/$defs/payload"`, ref)
			}
			defs, _ := doc["$defs"].(map[string]any)
			if _, ok := defs["payload"]; !ok {
				t.Error("no $defs.payload")
			}
			if typ := props["message_type"].(map[string]any)["type"]; typ != "string" {
				t.Errorf("message_type type = %v, want string", typ)
			}
		})
	}
}
