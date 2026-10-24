// Package compat checks that a protocol schema change is backward compatible.
//
// Within one protocol version (the envelope's "protocol" const), a change must keep accepting
// everything the base accepted: every base message type keeps a schema, and every base golden
// fixture (fixtures/valid) still validates against the head schemas. A version bump allows anything.
package compat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/glitchscout/protocol"
)

const envelopePath = "schemas/envelope.schema.json"

type envelope struct {
	Properties struct {
		Protocol struct {
			Const string `json:"const"`
		} `json:"protocol"`
		MessageType struct {
			Enum []string `json:"enum"`
		} `json:"message_type"`
	} `json:"properties"`
}

func readEnvelope(fsys fs.FS) (envelope, error) {
	var e envelope
	b, err := fs.ReadFile(fsys, envelopePath)
	if err != nil {
		return e, err
	}
	if err := json.Unmarshal(b, &e); err != nil {
		return e, fmt.Errorf("%s: %w", envelopePath, err)
	}
	return e, nil
}

// Check compares head against base and returns one problem per incompatibility.
// An error means the check itself couldn't run (unreadable or uncompilable schemas).
func Check(base, head fs.FS) ([]string, error) {
	be, err := readEnvelope(base)
	if err != nil {
		return nil, fmt.Errorf("base: %w", err)
	}
	he, err := readEnvelope(head)
	if err != nil {
		return nil, fmt.Errorf("head: %w", err)
	}
	if be.Properties.Protocol.Const != he.Properties.Protocol.Const {
		return nil, nil
	}

	var problems []string
	for _, mt := range be.Properties.MessageType.Enum {
		if !slices.Contains(he.Properties.MessageType.Enum, mt) {
			problems = append(problems, fmt.Sprintf("message type %s removed without a protocol version bump", mt))
		}
	}

	c, err := protocol.Compiler(head)
	if err != nil {
		return nil, fmt.Errorf("head: %w", err)
	}
	sch, err := c.Compile(protocol.SchemaURL("schemas/message.schema.json"))
	if err != nil {
		return nil, fmt.Errorf("head: %w", err)
	}
	err = fs.WalkDir(base, "fixtures/valid", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(p) != ".json" {
			return err
		}
		b, err := fs.ReadFile(base, p)
		if err != nil {
			return err
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("base %s: %w", p, err)
		}
		if err := sch.Validate(doc); err != nil {
			problems = append(problems, fmt.Sprintf("%s is no longer accepted: %s", p, firstLine(err.Error())))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return problems, nil
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	return s
}
