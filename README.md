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
| `CHOOSE` | O→E | Perform one of the observation's `legal_actions` by id (with a `target` when the entry lists targets) |
| `STEP` | O→E | Advance N fixed ticks in step mode |
| `COMMAND` | O→E | Setup and debug commands, standard or game-registered |
| `STATE_CTRL` | O→E | Reset tiers: `RELOAD_SCENE`, `SAVE` / `LOAD` a slot |
| `QUERY` / `QUERY_REPLY` | O→E / E→O | Navigation paths and samples, raycasts, line of sight, entity search, on-screen checks |
| `CAPTURE` | O→E | Request a screenshot or start a live-view stream, answered with binary frames |
| `ACK` | both | Success reply for requests without a dedicated reply type |
| `HEARTBEAT`, `ERROR`, `GOODBYE` | both | Liveness, structured errors, clean shutdown |

Each message type has a schema in [schemas/messages/](schemas/messages/), and
[schemas/message.schema.json](schemas/message.schema.json) validates any message. Payloads are
strict: unknown fields are rejected.

**Legal actions.** Games with discrete decisions (card games, visual novels, turn-based games) can list the
moves they would accept right now in an observation's `legal_actions` section, and set
`manifest.supports.legal_actions: true`. Each entry has a stable `id`, a `kind` (`card`, `choice`, `move`,
`buy`, `end_turn`, `ui` or `custom`) and a `label`, and optionally `targets` (entity ids), `cost`
(resource → amount), `group` (e.g. `hand`, `shop`) and `source` (`provider`, the default, or `ui` when the
list was built from visible UI controls). The orchestrator performs one with
`CHOOSE {action_id, target?, args?}`, answered by `ACK`, or by `ERROR` with code `illegal_action` and
`details.reason`: `unknown_action` (the id isn't in the current list, e.g. a card already played),
`bad_target` (missing, or not one of the entry's `targets`) or `refused` (listed, but the game rejects it now). A `STATE_UPDATE` delta never diffs
the list: it carries the whole list whenever it changed (`[]` when no move is legal) and leaves it out when it didn't.

**Size limits.** JSON text frames are capped at 256 KB by default (`manifest.limits.max_text_frame_bytes`).
`OBSERVE` takes `radius`, `entity_types`, `max_n` (nearest first; for `legal_actions`, the first n in the
game's order) and `sections` filters. An observation that would still exceed the cap is truncated (entities
by distance, `legal_actions` from the end) and flagged `truncated: true`. Binary frames
are capped at 4 MB (`max_binary_frame_bytes`).

## UI elements

An observation's `ui_elements` describe the game's UI the same way in every engine, so a test that says
`type=button text='Play'` finds the same control in a Godot, Unity or Unreal build. An SDK follows these rules:

- **Types.** `type` is one of the standard types (`protocol.UITypes` in Go), never the engine's class name:

  | Type | What it is | `value` | `set_value` takes |
  |---|---|---|---|
  | `button` | pressed to do something: a button, a menu entry, a link | none | nothing |
  | `checkbox` | toggled on and off | boolean | a boolean, or `"true"` / `"false"` |
  | `option` | a dropdown | the selected option's text | the option's text or its 0-based index |
  | `slider` | a number in a range, spin boxes included | number | a number (not `"0.5"`) |
  | `text_input` | a single- or multi-line text field | its text | a string |
  | `label` | text a player reads but can't operate | none | nothing |
  | `control` | any other control the game marks, such as a tab | none | nothing |

  A `set_value` the control can't take is answered with `ERROR` code `invalid_args`.
- **Text.** A `button` or `checkbox` reports its caption as its own `text`, also when the engine keeps it in a
  child text widget; that child isn't reported as a separate element. An `option`'s `text` is its selected
  option, not its caption.
- **Labels.** A slider, text field or dropdown has no stable text of its own: its `text` is what's typed or
  selected, and a slider has none. Its optional `label` is the name a player reads for it, usually a caption
  beside it ("Volume", "Player name", "Quality"), and it doesn't change as the control is used. A `button` or
  `checkbox` usually has no `label`; its `text` names it. Where an SDK takes a label from is up to the engine:
  an explicit name the game sets, the engine's accessibility name, a text field's placeholder, or the text just
  before the control in its layout.
- **Dropdown lists.** A dropdown's open list of options isn't reported as elements; a test picks an option with
  `set_value`.
- **Hidden controls.** A hidden or inactive control may be reported with `visible: false` or left out; the two mean
  the same.
- **Rectangles.** `rect` is `[x, y, width, height]` in pixels from the top-left of the screen, y downward. An engine
  whose UI is laid out from the bottom-left (Unity uGUI) converts.

[fixtures/valid/OBSERVATION/ui-types.json](fixtures/valid/OBSERVATION/ui-types.json) has one element of each type.
[fixtures/valid/OBSERVATION/ui-labels.json](fixtures/valid/OBSERVATION/ui-labels.json) has a labelled slider,
text field and dropdown.

## Binary frames

Screenshots and live-view frames are WebSocket binary messages: a 64-byte little-endian header plus
the payload. See [binary-frame-v1.md](binary-frame-v1.md) and the Kaitai Struct definition
[binary-frame-v1.ksy](binary-frame-v1.ksy). Golden frames are in `fixtures/binary/`.

## Evidence formats

What a run leaves behind is versioned with the protocol, in `schemas/evidence/`, so reporters, dashboards and
other tools can read it without the orchestrator:

| Schema | File | What |
|---|---|---|
| `run` | `runs/<run_id>/run.json` | versions, build and the tests selected |
| `result` | `results/<test_id>/attempt-<n>/result.json` | status, each Gherkin step's status, the deciding oracle, findings, timings, cost |
| `trace_line` | `trace.jsonl` (one object per line) | per step: tick, observation digest, decision, skill, actions, oracle values, events |
| `evidence_index` | `evidence_index.json` | every file of the attempt with its type, SHA-256, size and tick range |
| `shrink_result` | `repro/shrink.json` | what came of reducing a failing attempt: decisions and ticks before and after, replays used, how often it reproduced |

Files are validated against these schemas when they are written. Fixtures are in `fixtures/evidence/`.

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

`protocol.UITypes` lists the standard UI element types, with a constant for each (`protocol.UIButton`, …)
and `protocol.UIInteractiveTypes` for the ones a player operates.

`protocol.Compiler(protocol.FS)` returns a `santhosh-tekuri/jsonschema` compiler with every schema registered.

## Compatibility

Within a protocol version, a schema change must keep accepting every golden fixture on the base
branch, and no message type may be removed. CI checks every pull request with:

```bash
go run ./cmd/compatcheck -base <checkout of main> -head .
```

A breaking change needs a new protocol version (`glitchscout/2`).

An optional field added within a version (such as `ui_element.label`) changes nothing for an SDK that doesn't
send it. Most objects reject properties they don't list, though, so a receiver built before the field rejects a
message that carries it: release the receiver's support first, then the sender's.

## Checks

```bash
go test ./...
```
