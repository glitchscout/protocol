# protocol

Engine-agnostic protocol between Glitchscout and game-engine SDKs: JSON Schemas, binary frame spec,
golden fixtures and the SDK conformance suite. Licensed under Apache-2.0.

## Protocol v1 (`glitchscout/1`)

The orchestrator is a WebSocket server on `127.0.0.1`; each game instance connects as a client,
using the endpoint and token it was launched with. Every JSON text frame is an envelope
([schemas/envelope.schema.json](schemas/envelope.schema.json)):

```json
{ "protocol": "glitchscout/1", "message_type": "STEP", "id": "<uuid>", "reply_to": null,
  "session": "s-1", "tick_frame": 100, "ts": 1761264000.25, "payload": { "ticks": 30 } }
```

| Message | Direction | Purpose |
|---|---|---|
| `HELLO` / `HELLO_ACK` | E→O / O→E | Token auth, version negotiation, session resume, capability manifest |
| `OBSERVE` / `OBSERVATION` | O→E / E→O | Pull a state snapshot, with interest-management filters |
| `STATE_UPDATE` | E→O | Optional stream of full or delta observations |
| `EVENT` | E→O | Game events, log lines, script errors |
| `ACTION_INJECT` | O→E | Semantic input by input-action name: buttons and analog axes |
| `UI_ACTION` | O→E | Focus, click, set a value on, or submit a UI element |
| `STEP` | O→E | Advance N fixed ticks in step mode |
| `COMMAND` | O→E | Setup and debug commands, standard or game-registered |
| `STATE_CTRL` | O→E | Reset tiers: `RELOAD_SCENE`, `SAVE` / `LOAD` a slot |
| `QUERY` / `QUERY_REPLY` | O→E / E→O | Navigation paths and samples, raycasts, line of sight, entity search, on-screen checks |
| `CAPTURE` | O→E | Request a screenshot, answered with binary frames |
| `ACK` | both | Success reply for requests without a dedicated reply type |
| `HEARTBEAT`, `ERROR`, `GOODBYE` | both | Liveness, structured errors, clean shutdown |

Each message type has a schema in [schemas/messages/](schemas/messages/), and
[schemas/message.schema.json](schemas/message.schema.json) validates any message. Payloads are
strict: unknown fields are rejected.

**Size limits.** JSON text frames are capped at 256 KB by default (`manifest.limits.max_text_frame_bytes`).
`OBSERVE` takes `radius`, `entity_types`, `max_n` (nearest first) and `sections` filters. An observation
that would still exceed the cap is truncated by distance and flagged `truncated: true`. Binary frames
are capped at 4 MB (`max_binary_frame_bytes`).

## Fixtures

`fixtures/valid/<MESSAGE_TYPE>/*.json` are golden messages every implementation must accept;
`fixtures/invalid/<MESSAGE_TYPE>/*.json` (and `fixtures/invalid/envelope/`) must be rejected.

## Using it from Go

The module embeds the schemas and fixtures:

```go
import "github.com/glitchscout/protocol"

b, err := protocol.FS.ReadFile("schemas/messages/step.schema.json")
```

Schema `$id`s live under `https://glitchscout.com/schemas/v1/`; resolve them from `protocol.FS`
rather than over the network.

`protocol.Compiler(protocol.FS)` returns a `santhosh-tekuri/jsonschema` compiler with every schema registered.

## Compatibility

Within a protocol version, a schema change must keep accepting every golden fixture on the base
branch, and no message type may be removed. CI checks every pull request with:

```bash
go run ./cmd/compatcheck -base <checkout of main> -head .
```

A breaking change needs a new protocol version (`glitchscout/2`).

## Checks

```bash
go test ./...
```
