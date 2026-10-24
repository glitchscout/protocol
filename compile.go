package protocol

import (
	"bytes"
	"fmt"
	"io/fs"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// SchemaURL returns the $id of the schema at path p in FS, e.g. "schemas/messages/step.schema.json".
func SchemaURL(p string) string { return SchemaBaseURL + strings.TrimPrefix(p, "schemas/") }

// MessageSchemaPath returns the FS path of a message type's schema, e.g. STEP → schemas/messages/step.schema.json.
func MessageSchemaPath(messageType string) string {
	return "schemas/messages/" + strings.ToLower(messageType) + ".schema.json"
}

// Compiler returns a JSON Schema compiler with every schemas/**/*.schema.json in fsys registered
// under its $id, so $refs resolve without network access. Compile a schema with c.Compile(SchemaURL(p)).
func Compiler(fsys fs.FS) (*jsonschema.Compiler, error) {
	c := jsonschema.NewCompiler()
	err := fs.WalkDir(fsys, "schemas", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".schema.json") {
			return err
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		if err := c.AddResource(SchemaURL(p), doc); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return c, nil
}
