meta:
  id: glitchscout_binary_frame_v1
  title: Glitchscout binary frame v1
  license: Apache-2.0
  endian: le
doc: |
  One WebSocket binary message in protocol glitchscout/1: a 64-byte header (longer in later
  versions, see header_length) followed by the payload. See binary-frame-v1.md.
seq:
  - id: magic
    contents: GSBF
  - id: header_version
    type: u1
    valid:
      min: 1
  - id: frame_type
    type: u1
    enum: frame_type
  - id: encoding
    type: u1
    enum: encoding
  - id: flags
    type: flags
  - id: header_length
    type: u2
    valid:
      min: 64
  - id: reserved0
    type: u2
  - id: payload_length
    type: u4
  - id: tick_frame
    type: u8
  - id: request_id
    size: 16
    doc: Raw UUID, RFC 4122 byte order.
  - id: width
    type: u2
  - id: height
    type: u2
  - id: source_width
    type: u2
  - id: source_height
    type: u2
  - id: chunk_index
    type: u2
  - id: chunk_count
    type: u2
  - id: payload_crc32
    type: u4
    doc: CRC-32 (ISO-HDLC) of payload.
  - id: reserved1
    size: 8
  - id: extension
    size: header_length - 64
    doc: Bytes added by later header versions; v1 readers skip them.
  - id: payload
    size: payload_length
types:
  flags:
    seq:
      - id: reserved
        type: b2
      - id: truncated_capture
        type: b1
      - id: lossless_baseline
        type: b1
      - id: reencoded_for_cap
        type: b1
      - id: downscaled
        type: b1
      - id: chunked
        type: b1
      - id: final_chunk
        type: b1
enums:
  frame_type:
    1: screenshot
    2: live_view
    3: generic_blob
    4: video_chunk
  encoding:
    1: png
    2: jpeg
    3: raw_rgba8
    4: webp
