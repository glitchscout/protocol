package protocol

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"hash/crc32"
	"io/fs"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// refHeader is a reference decoding of the binary-frame-v1 header, used to check the fixtures.
type refHeader struct {
	Magic         [4]byte
	HeaderVersion uint8
	FrameType     uint8
	Encoding      uint8
	Flags         uint8
	HeaderLength  uint16
	Reserved0     uint16
	PayloadLength uint32
	TickFrame     uint64
	RequestID     [16]byte
	Width         uint16
	Height        uint16
	SourceWidth   uint16
	SourceHeight  uint16
	ChunkIndex    uint16
	ChunkCount    uint16
	PayloadCRC32  uint32
	Reserved1     [8]byte
}

const maxAssembledFrame = 4 << 20

// refParse applies the spec's validation order and returns the header, payload or the reason code.
func refParse(msg []byte) (refHeader, []byte, string) {
	var h refHeader
	if len(msg) < 64 {
		return h, nil, "truncated"
	}
	_ = binary.Read(bytes.NewReader(msg[:64]), binary.LittleEndian, &h)
	switch {
	case string(h.Magic[:]) != "GSBF":
		return h, nil, "bad_magic"
	case h.HeaderVersion == 0:
		return h, nil, "bad_version"
	case h.HeaderLength < 64:
		return h, nil, "bad_header_length"
	case int(h.HeaderLength)+int(h.PayloadLength) != len(msg):
		return h, nil, "length_mismatch"
	case len(msg) > maxAssembledFrame:
		return h, nil, "too_large"
	}
	payload := msg[h.HeaderLength:]
	if crc32.ChecksumIEEE(payload) != h.PayloadCRC32 {
		return h, nil, "bad_crc"
	}
	return h, payload, ""
}

type headerJSON struct {
	Magic         string `json:"magic"`
	HeaderVersion uint8  `json:"header_version"`
	FrameType     uint8  `json:"frame_type"`
	Encoding      uint8  `json:"encoding"`
	Flags         uint8  `json:"flags"`
	HeaderLength  uint16 `json:"header_length"`
	PayloadLength uint32 `json:"payload_length"`
	TickFrame     uint64 `json:"tick_frame"`
	RequestID     string `json:"request_id"`
	Width         uint16 `json:"width"`
	Height        uint16 `json:"height"`
	SourceWidth   uint16 `json:"source_width"`
	SourceHeight  uint16 `json:"source_height"`
	ChunkIndex    uint16 `json:"chunk_index"`
	ChunkCount    uint16 `json:"chunk_count"`
	PayloadCRC32  uint32 `json:"payload_crc32"`
}

type frameExpectation struct {
	Description   string      `json:"description"`
	Valid         bool        `json:"valid"`
	Reason        string      `json:"reason"`
	Header        *headerJSON `json:"header"`
	PayloadSHA256 string      `json:"payload_sha256"`
}

func toJSON(h refHeader) headerJSON {
	return headerJSON{
		Magic: string(h.Magic[:]), HeaderVersion: h.HeaderVersion, FrameType: h.FrameType, Encoding: h.Encoding,
		Flags: h.Flags, HeaderLength: h.HeaderLength, PayloadLength: h.PayloadLength, TickFrame: h.TickFrame,
		RequestID: uuid.UUID(h.RequestID).String(), Width: h.Width, Height: h.Height, SourceWidth: h.SourceWidth,
		SourceHeight: h.SourceHeight, ChunkIndex: h.ChunkIndex, ChunkCount: h.ChunkCount, PayloadCRC32: h.PayloadCRC32,
	}
}

func sha(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func binFixtures(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := fs.ReadDir(FS, dir)
	if err != nil {
		t.Fatalf("%s: %v", dir, err)
	}
	var out []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".bin") {
			out = append(out, path.Join(dir, e.Name()))
		}
	}
	if len(out) == 0 {
		t.Fatalf("no .bin fixtures in %s", dir)
	}
	return out
}

func readExpectation(t *testing.T, p string, v any) {
	t.Helper()
	b, err := FS.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		t.Fatalf("%s: %v", p, err)
	}
}

func TestBinaryHeaderIs64Bytes(t *testing.T) {
	if n := binary.Size(refHeader{}); n != 64 {
		t.Fatalf("binary.Size(header) = %d, want 64", n)
	}
}

func TestBinaryFixtures(t *testing.T) {
	for _, kind := range []string{"valid", "invalid"} {
		for _, bin := range binFixtures(t, "fixtures/binary/"+kind) {
			t.Run(bin, func(t *testing.T) {
				msg, err := FS.ReadFile(bin)
				if err != nil {
					t.Fatal(err)
				}
				var want frameExpectation
				readExpectation(t, strings.TrimSuffix(bin, ".bin")+".json", &want)
				if want.Valid != (kind == "valid") {
					t.Fatalf("expectation valid=%v in %s/", want.Valid, kind)
				}
				h, payload, reason := refParse(msg)
				if !want.Valid {
					if reason != want.Reason {
						t.Fatalf("reason = %q, want %q", reason, want.Reason)
					}
					return
				}
				if reason != "" {
					t.Fatalf("rejected: %s", reason)
				}
				if got := toJSON(h); want.Header == nil || got != *want.Header {
					t.Errorf("header = %+v\nwant     %+v", got, want.Header)
				}
				if got := sha(payload); got != want.PayloadSHA256 {
					t.Errorf("payload sha256 = %s, want %s", got, want.PayloadSHA256)
				}
			})
		}
	}
}

type chunkSet struct {
	Description    string   `json:"description"`
	Chunks         []string `json:"chunks"`
	AssembledSHA   string   `json:"assembled_sha256"`
	AssembledBytes int      `json:"assembled_bytes"`
}

func TestBinaryChunkSets(t *testing.T) {
	sets, err := fs.Glob(FS, "fixtures/binary/chunked/*/set.json")
	if err != nil || len(sets) == 0 {
		t.Fatalf("no chunk sets: %v", err)
	}
	for _, setPath := range sets {
		t.Run(setPath, func(t *testing.T) {
			var set chunkSet
			readExpectation(t, setPath, &set)
			var first refHeader
			parts := make([][]byte, len(set.Chunks))
			for i, name := range set.Chunks {
				msg, err := FS.ReadFile(path.Join(path.Dir(setPath), name))
				if err != nil {
					t.Fatal(err)
				}
				h, payload, reason := refParse(msg)
				if reason != "" {
					t.Fatalf("%s rejected: %s", name, reason)
				}
				if i == 0 {
					first = h
				}
				if h.RequestID != first.RequestID || h.TickFrame != first.TickFrame || h.ChunkCount != uint16(len(set.Chunks)) {
					t.Fatalf("%s: request_id, tick_frame and chunk_count must match the set", name)
				}
				if h.Flags&0b10 == 0 {
					t.Errorf("%s: chunked flag not set", name)
				}
				if final := h.Flags&0b1 != 0; final != (int(h.ChunkIndex) == len(set.Chunks)-1) {
					t.Errorf("%s: final_chunk flag must be set on the last chunk only", name)
				}
				if int(h.ChunkIndex) >= len(parts) || parts[h.ChunkIndex] != nil {
					t.Fatalf("%s: chunk_index %d out of range or duplicated", name, h.ChunkIndex)
				}
				parts[h.ChunkIndex] = payload
			}
			if slices.ContainsFunc(parts, func(b []byte) bool { return b == nil }) {
				t.Fatal("missing chunk index")
			}
			assembled := bytes.Join(parts, nil)
			if len(assembled) != set.AssembledBytes || sha(assembled) != set.AssembledSHA {
				t.Errorf("assembled %d bytes sha256 %s, want %d bytes %s", len(assembled), sha(assembled), set.AssembledBytes, set.AssembledSHA)
			}
		})
	}
}
