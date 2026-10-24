# Binary frame format v1

Every WebSocket **binary** message in protocol `glitchscout/1` is a fixed 64-byte header followed by the
payload. Binary frames carry screenshots (`CAPTURE` replies) and live-view frames. They are
self-describing: no companion JSON message is needed. A machine-readable definition is in
[binary-frame-v1.ksy](binary-frame-v1.ksy) (Kaitai Struct).

All integers are **little-endian**.

## Header

| Offset | Size | Type | Field | Meaning |
|---|---|---|---|---|
| 0 | 4 | bytes | `magic` | ASCII `GSBF` (`0x47 0x53 0x42 0x46`) |
| 4 | 1 | u8 | `header_version` | `1` |
| 5 | 1 | u8 | `frame_type` | `1` screenshot, `2` live-view frame, `3` generic blob (reserved), `4` video chunk (reserved) |
| 6 | 1 | u8 | `encoding` | `1` PNG, `2` JPEG, `3` raw RGBA8 (debug builds only), `4` WebP (reserved) |
| 7 | 1 | u8 | `flags` | see below |
| 8 | 2 | u16 | `header_length` | `64` in v1. Readers skip `header_length − 64` extension bytes written by later versions. |
| 10 | 2 | u16 | `reserved0` | `0` |
| 12 | 4 | u32 | `payload_length` | Payload bytes in *this* message |
| 16 | 8 | u64 | `tick_frame` | Game tick at which the frame was rendered |
| 24 | 16 | bytes | `request_id` | Raw UUID (RFC 4122 byte order) of the `CAPTURE` request, or of the live-view subscription |
| 40 | 2 | u16 | `width` | Encoded image width in pixels |
| 42 | 2 | u16 | `height` | Encoded image height in pixels |
| 44 | 2 | u16 | `source_width` | Viewport width before any downscale |
| 46 | 2 | u16 | `source_height` | Viewport height before any downscale |
| 48 | 2 | u16 | `chunk_index` | 0-based chunk number (`0` if not chunked) |
| 50 | 2 | u16 | `chunk_count` | Total chunks (`1` if not chunked) |
| 52 | 4 | u32 | `payload_crc32` | CRC-32 (ISO-HDLC, as zlib `crc32`) of this message's payload |
| 56 | 8 | bytes | `reserved1` | Zero-filled; reserved for v1 extensions |
| 64 | n | bytes | `payload` | Encoded image bytes, or this chunk's slice of them |

**Flags:** bit 0 `final_chunk`, bit 1 `chunked`, bit 2 `downscaled`, bit 3 `reencoded_for_cap`,
bit 4 `lossless_baseline`, bit 5 `truncated_capture` (partial viewport), bits 6–7 reserved.

**Reserved fields** (`reserved0`, `reserved1`, flag bits 6–7) are written as zero and ignored on read,
so v1 readers accept future v1 extensions.

## Validation

Readers check, in this order, and drop the message with the first failing reason:

| Step | Check | Reason |
|---|---|---|
| 1 | Message is at least 64 bytes | `truncated` |
| 2 | `magic` is `GSBF` | `bad_magic` |
| 3 | `header_version` ≥ 1 (a higher version is parsed for the v1 fields only) | `bad_version` |
| 4 | `header_length` ≥ 64 | `bad_header_length` |
| 5 | `header_length + payload_length` equals the message length | `length_mismatch` |
| 6 | Message is at most 4 MB (4,194,304 bytes) | `too_large` |
| 7 | CRC-32 of the payload equals `payload_crc32` | `bad_crc` |
| 8 | `request_id` matches an outstanding `CAPTURE` or live-view subscription | `unknown_request` |

The orchestrator logs `frame_invalid:<reason>`, retries a screenshot `CAPTURE` once, and continues.
A missing frame never fails a test.

## Size

Header plus payload is at most 4 MB per *assembled* frame. Routine frames (cadence, failure and
live-view) target 256 KB or less: JPEG plus downscale. Visual-baseline checkpoints use lossless PNG
and are exempt from the target. Above the cap, the SDK re-encodes as JPEG at lower quality, then
downscales; if it's still too large, it sends `ERROR` with code `frame_too_large`.

## Chunking

Chunking is the fallback for transports with small send buffers.

- Each chunk is at most 1 MiB and carries a full header with the same `request_id`, `tick_frame`,
  `frame_type`, `encoding`, dimensions and `chunk_count`, and its own `payload_crc32`.
- Every chunk sets `chunked`; the chunk with `chunk_index = chunk_count − 1` also sets `final_chunk`.
- The reader reassembles by `request_id`, concatenating payloads in `chunk_index` order (chunks may
  arrive in any order). Incomplete sets are dropped after 2 seconds.

Reassembly reasons: `chunk_index_out_of_range`, `chunk_count_mismatch`, `chunk_mismatch` (header fields
differ between chunks), `duplicate_chunk`, `assembled_too_large`, `incomplete` (timed out).

## Fixtures

- `fixtures/binary/valid/*.bin`: each with a `.json` holding the decoded header and the payload's SHA-256.
- `fixtures/binary/invalid/*.bin`: each with a `.json` holding the expected reason.
- `fixtures/binary/chunked/<set>/`: chunk files plus `set.json` with the assembled payload's SHA-256.

SDKs must produce byte-identical headers for the same inputs.
