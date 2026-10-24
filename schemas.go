// Package protocol publishes the Glitchscout protocol v1 (glitchscout/1) JSON Schemas and golden
// fixtures, so Go consumers can read them from a pinned module version.
package protocol

import "embed"

// Version is the protocol identifier carried in every envelope.
const Version = "glitchscout/1"

// SchemaBaseURL is the $id prefix of every schema. Schemas are resolved from FS, never fetched.
const SchemaBaseURL = "https://glitchscout.com/schemas/v1/"

// FS holds schemas/**/*.schema.json and fixtures/{valid,invalid}/<MESSAGE_TYPE>/*.json.
//
//go:embed schemas fixtures
var FS embed.FS
