# Decio configuration

Configuration is YAML with strict field checking. Unknown keys are rejected.
The current version is `1`. A provider has a `type` (currently `jev`) and an
optional `model`. `input.sources` is a map of named sources; each source must
use exactly one of `stdin`, `command`, `file`, or `literal`.

## Schema and constraints

| Key | Type | Required | Constraint |
| --- | --- | --- | --- |
| `version` | integer | no | Omitted or `1`. |
| `provider.type` | string | no | Provider name; defaults to `jev`. |
| `provider.model` | string | no | Provider model; Jev defaults to `jev-latest`. |
| `input.sources.<name>` | object | no | Exactly one source kind is required. |
| `input.sources.<name>.stdin` | boolean | one source kind | Uses the original process stdin. |
| `input.sources.<name>.command` | string | one source kind | Runs a command and uses stdout as input. |
| `input.sources.<name>.file` | string | one source kind | Reads the file as input. |
| `input.sources.<name>.literal` | string | one source kind | Uses the literal as input. |
| `input.sources.<name>.timeout` | duration | no | Positive duration for a command source, such as `5s`. |
| `decision.type` | `choice`, `boolean`, or `score` | yes | Selects the typed decision. |
| `decision.prompt` | string | no | Prompt sent to the provider. |
| `decision.choices` | map | choice | At least two choice IDs are required. |
| `decision.choices.<id>.description` | string | no | Human-readable choice meaning. |
| `decision.range.min`, `max` | integer | score | Inclusive bounds; `min < max`. |
| `decision.levels` | list of strings | no | If set for score, at least two ordered levels. |
| `actions` | map or list | no | Map for choice/boolean; list of ranges for score. |
| `actions.<key>.command` | string | action | Non-empty trusted command. |
| `actions[].min`, `max` | integer | score action | Inclusive ranges; ranges must not overlap. `max` may be omitted. |
| `actions.*.stdin` | `original` or `none` | no | Defaults to `none`; `original` forwards original stdin. |
| `actions.*.env` | map of strings | no | Environment values with result substitutions. |

Boolean action keys are exactly `"true"` and `"false"`. Choice action keys
must match a configured choice ID. A score action is selected by its inclusive
range. A source timeout is a duration and should be positive when configured.

## Choice configuration

```yaml
version: 1

provider:
  type: jev
  model: jev-latest

input:
  sources:
    context:
      stdin: true

decision:
  type: choice
  prompt: Determine the appropriate follow-up for this change.
  choices:
    none:
      description: No additional review is required.
    normal:
      description: A normal review is required.
    security:
      description: A security review is required.

actions:
  security:
    command: ./scripts/security-review.sh
    stdin: original
```

## Boolean configuration

```yaml
version: 1

provider:
  type: jev
  model: jev-latest

input:
  sources:
    diff:
      command: git diff
      timeout: 5s

decision:
  type: boolean
  prompt: Does this change require running the test suite?

actions:
  "true":
    command: mise run test
```

## Score configuration

```yaml
version: 1

provider:
  type: jev
  model: jev-latest

input:
  sources:
    diff:
      command: git diff
      timeout: 5s

decision:
  type: score
  prompt: Rate the security risk of this change.
  range:
    min: 0
    max: 100
  levels:
    - very low
    - low
    - moderate
    - high
    - very high

actions:
  - min: 0
    max: 49
    command: ./scripts/normal-review.sh
  - min: 50
    max: 100
    command: ./scripts/security-review.sh
```

Generate a starting point with `decio init --type choice`,
`decio init --type boolean`, or `decio init --type score`, then validate it
with `decio config validate -c path/to/config.yaml`.
