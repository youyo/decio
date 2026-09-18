# Decio

Decio turns context into a typed decision and, optionally, dispatches a
pre-declared command. It supports choice, boolean, and bounded score decisions;
stdin, command, file, and literal inputs; plain or JSON output; and the Jev
provider from TypeSafe.

The model may choose an allowed action, but it may not author the action.
Commands always come from the trusted configuration.

## Installation

Install the released binary with Homebrew:

```sh
brew install youyo/tap/decio
```

Or install it with Go:

```sh
go install github.com/youyo/decio@latest
```

Set the Jev API key before making a decision:

```sh
export TYPESAFE_API_KEY="..."
```

## Quick start

The root command accepts a decision directly, so no configuration file is
needed for simple cases. These examples read the current diff from stdin.

Choice:

```sh
git diff | decio \
  --choice none --choice normal --choice security \
  --prompt "Determine the appropriate review level."
```

Boolean:

```sh
git diff | decio \
  --boolean "Does this change require running tests?" \
  --result-exit-code
```

Score:

```sh
git diff | decio \
  --score "Rate the security risk of this change." \
  --min 0 --max 100 --json
```

The default output is a single plain value. Use `--json` when downstream code
needs metadata such as the provider, model, or confidence.

## Configuration

Use `-c`/`--config` to load a YAML configuration. If the flag is omitted, Decio
looks for `.decio.yaml` and then `.decio.yml` in the current directory.
`decio config validate -c path/to/config.yaml` validates a file without calling
the provider.

The schema is intentionally strict: unknown keys are rejected.

| Field | Type | Description |
| --- | --- | --- |
| `version` | integer | Configuration version; `1` is the current version. |
| `provider.type` | string | Provider name, currently `jev`. |
| `provider.model` | string | Provider model; Jev defaults to `jev-latest`. |
| `input.sources.<name>` | object | One named input source. Exactly one source kind is required. |
| `input.sources.<name>.stdin` | boolean | Use the original process stdin. |
| `input.sources.<name>.command` | string | Run a shell command and use stdout as the value. |
| `input.sources.<name>.file` | string | Read a file as the value. |
| `input.sources.<name>.literal` | string | Use a literal value. |
| `input.sources.<name>.timeout` | duration | Timeout for a command source, such as `5s`. |
| `decision.type` | `choice \| boolean \| score` | The typed decision to make. |
| `decision.prompt` | string | Instructions sent to the provider. |
| `decision.choices.<id>.description` | string | Description for a choice ID; choice decisions need at least two IDs. |
| `decision.range.min`, `max` | integer | Inclusive score bounds; `min` must be less than `max`. |
| `decision.levels` | list of strings | Ordered qualitative levels for a score decision. |
| `actions` | map or list | A map for choice/boolean, or a list of ranges for score. |
| `actions.<key>.command` | string | Command to execute when a map action is selected. |
| `actions[].min`, `max` | integer | Inclusive score-action bounds; ranges may not overlap. |
| `actions.*.stdin` | `original \| none` | Action stdin mode; the default is `none`. |
| `actions.*.env` | map of strings | Additional action environment values. |

For score actions, `max` may be omitted to create an open-ended upper bound.
For action stdin, `original` means the original process stdin, not the
normalized multi-source provider input.

See the complete examples in [`examples/`](examples/).

## Input sources

When no named sources are configured, piped stdin becomes the single text input.
Named sources are collected in YAML configuration order and sent to the
provider as structured state:

```yaml
input:
  sources:
    event:
      stdin: true
    diff:
      command: git diff
    policy:
      file: .decio/security-policy.md
    repository:
      literal: backend-api
```

Command sources run through `sh -c`, capture stdout, and treat a non-zero exit
or timeout as an input error. Their stderr is kept for diagnostics and is not
silently added to provider input. The original stdin is read once so it can be
used both for the decision and, when requested, by a dispatched action.

## Dispatch and action environment

Dispatch is optional. For choice and boolean decisions, the selected value is
used as an action key (`true` or `false` for boolean). For scores, the first
inclusive range containing the result is selected. A missing action means
output-only execution, not an error.

Each action receives these metadata variables:

```text
DECIO_TYPE
DECIO_VALUE
DECIO_PROVIDER
DECIO_MODEL
DECIO_CONFIDENCE
```

Custom `env` values support only these substitutions:

```text
{{ result.value }}
{{ result.type }}
{{ result.confidence }}
{{ result.provider }}
{{ result.model }}
```

Confidence is empty when the provider did not return it. Action stdout and
stderr are both written to Decio's stderr; Decio's stdout remains reserved for
the result. An action's exit code is returned directly.

## Output

Plain output contains no decoration:

- choice: the selected choice ID
- boolean: `true` or `false`
- score: the integer score

With `--json`, Decio emits the stable result shape:

```json
{
  "type": "choice",
  "value": "security",
  "confidence": 0.94,
  "provider": "jev",
  "model": "jev-latest",
  "action": {
    "executed": true,
    "exit_code": 0
  }
}
```

`confidence` and `action` are omitted when unavailable.

## Exit codes

| Code | Meaning |
| ---: | --- |
| `0` | Decio succeeded, including a `false` boolean unless `--result-exit-code` is enabled. |
| `1` | A valid `false` boolean result with `--result-exit-code`, when no action ran. |
| `2` | Usage or configuration error. |
| `3` | Input collection failure. |
| `4` | Provider failure. |
| `5` | Provider result was invalid. |
| `6` | Action dispatch preparation failure. |
| `N` | The exit code returned by a dispatched action. |

## Jev provider

Decio sends requests to TypeSafe's Jev evaluation endpoint. Configure it with
environment variables, never in a committed YAML file:

```sh
export TYPESAFE_API_KEY="..."
export TYPESAFE_BASE_URL="https://api.typesafe.ai"
```

`TYPESAFE_BASE_URL` defaults to `https://api.typesafe.ai`; the client posts to
`/v1/systemone`. The configured model defaults to `jev-latest`. HTTP 429 and
529 responses are retried with `Retry-After` when supplied, or with 0.5s, 1s,
and 2s backoff. The CLI timeout applies to the request context.

Boolean decisions use Jev's `noul` probability. A value of `0.5` or greater is
`true`; a value below `0.5` is `false`.

Score decisions use Jev's ordered score levels. If the Jev score is `s` and
there are `n` levels, Decio converts it to the configured inclusive integer
range with:

```text
round(min + (s / (n - 1)) * (max - min))
```

The default levels are the five ordered labels `very low`, `low`, `moderate`,
`high`, and `very high`. Set `decision.levels` to replace them; at least two
levels are required by the provider contract. Choice and score decisions also
preserve Jev confidence when it is returned.

## Claude Code Hooks

The [`examples/claude-code/post-tool.yaml`](examples/claude-code/post-tool.yaml)
configuration accepts a hook event on stdin, collects the current diff, and
can run a follow-up command with the original hook event on stdin.

A corresponding `settings.json` entry can invoke it as a command hook:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Bash|Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "decio -c /absolute/path/to/examples/claude-code/post-tool.yaml"
          }
        ]
      }
    ]
  }
}
```

Use an absolute configuration path, and ensure the hook process has access to
`TYPESAFE_API_KEY`. The action commands are declared by the configuration and
are never generated from provider output.

## Development

The repository pins its development tools with mise:

```sh
mise install
mise run build
mise run test
mise run lint
mise run check
```

`mise run check` formats Go files, runs the linter, and runs the test suite.
Create a release with `mise run release` after configuring the GoReleaser and
GitHub credentials described in `.goreleaser.yaml` and
`.github/workflows/release.yaml`.

## Security model

Configuration files are executable policy and must be treated like shell
scripts. Decio never executes provider-generated command text: providers select
only among commands already declared by the user. Choice values are identifiers,
not shell fragments, and arbitrary model output is not interpolated into a
command string.

Secrets stay in environment or secret stores and should not be included in
provider input unless explicitly collected. Source stderr is diagnostic only.
External commands and HTTP calls are time-bounded, and malformed or
out-of-range decisions are rejected before dispatch.

## License

MIT. See [`LICENSE`](LICENSE).
