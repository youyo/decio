# Decio

## What is Decio?

Decio is a general-purpose decision CLI for turning context into a typed
decision and, optionally, dispatching a predefined action. It is designed for
Unix workflows, hooks, CI/CD, agents, and automation. Claude Code Hooks are
one representative use case, not a requirement: Decio itself has no
Claude-specific assumptions.

Decio does not generate or execute arbitrary commands. Every command that can
run as an action must be declared in trusted configuration. Neither a
decision result nor free-form model output is treated as a shell fragment.
The boundary remains:

```text
context → decision → predefined action
```

## Core concepts

Decio's core model is:

```text
Input → Decision → Action
```

- **Input** obtains the context that provides the material for a decision.
- **Decision** returns a typed result: `choice`, `boolean`, or `score`.
- **Action** runs a predefined operation associated with that decision, when
  one is configured.

Decio can complete an invocation after producing the decision; dispatch is
optional. This makes `context → typed decision` useful on its own for shell
conditions, CI/CD gates, and downstream automation, without turning Decio
into a free-form text generation CLI.

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

## Quick Start

Decio supports two levels of use. For simple cases, no configuration file is
needed: pass a decision type and prompt as CLI flags, and pipe context through
stdin. When the workflow needs complex inputs or actions, use declarative
YAML configuration.

### Simple: CLI flags and stdin

This boolean decision can be used directly in a Unix pipeline:

```sh
git diff | decio --boolean "Does this change require running tests?" --result-exit-code
```

Choice:

```sh
git diff | decio \
  --choice none --choice normal --choice security \
  --prompt "Determine the appropriate review level."
```

Score:

```sh
git diff | decio \
  --score "Rate the security risk of this change." \
  --min 0 --max 100 --json
```

The default output is a single plain value. Use `--json` when downstream code
needs metadata such as the provider, model, or confidence. With
`--result-exit-code`, a valid `false` boolean result returns exit code `1`
when no action ran, which makes a typed decision usable as a shell condition;
provider and other Decio failures still use their documented error codes.

### Declarative: configuration

Start with a commented configuration template, edit it, then validate it:

```sh
decio init --type boolean -o .decio.yaml
decio config validate -c .decio.yaml
```

Run the configured workflow with:

```sh
decio -c .decio.yaml
```

## Configuration

Use `-c`/`--config` to load a YAML configuration. If the flag is omitted,
Decio looks for `.decio.yaml` and then `.decio.yml` in the current directory.
`decio config validate -c path/to/config.yaml` validates a file without
calling the provider.

`decio init` writes a commented configuration template to stdout or to the
path given by `-o`/`--output`; choose `choice`, `boolean`, or `score` with
`--type`, and use `--force` to overwrite an existing file.

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

## Use Cases

### Claude Code Hooks

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

### Git Hooks

A Git hook can make a typed decision from the staged diff. With
`--result-exit-code`, `0` means a valid `true` result and `1` means a valid
`false` result when no action ran; codes `2` and above indicate a Decio or
provider failure.

```sh
git diff --cached | decio \
  --boolean "Does this staged change require running tests?" \
  --result-exit-code
```

### CI/CD

CI can consume JSON metadata and make a follow-up gate with standard shell
tools:

```sh
git diff HEAD~1 | decio \
  --score "Rate the security risk of this change." \
  --min 0 --max 100 --json | jq -e '.value < 60'
```

For reusable CI policy, put the input and action policy in a configuration
file and run `decio -c .decio.yaml --json`.

### Shell automation

The plain result is convenient for shell variables, while `--json` exposes
stable metadata for tools such as `jq`:

```sh
value=$(git diff | decio --boolean "Should tests run?")
printf 'decision=%s\n' "$value"
```

Actions remain optional and predefined, so a pipeline can choose whether to
handle the typed result itself or dispatch a configured action.

## Input sources

Input can come from outside Decio through stdin, or Decio can collect it from
configured sources. This keeps both ordinary Unix pipelines and reusable
declarative workflows straightforward.

External context can be piped directly into a flag-based invocation:

```sh
git diff | decio --boolean "Does this change require running tests?"
```

Decio can also obtain context itself from a configured command:

```yaml
input:
  sources:
    diff:
      command: git diff HEAD~1
```

Each source kind has a distinct role:

- `stdin`: context handed in by the caller, such as a hook event or a piped diff.
- `command`: context Decio collects itself by running a predefined command.
- `file`: a policy or reference document read from disk.
- `literal`: a fixed value such as a repository or environment name.

If no named sources are configured, piped stdin becomes the single text input.
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

Command sources run through `sh -c`, capture stdout, and treat a non-zero
exit or timeout as an input error. Their stderr is kept for diagnostics and is
not silently added to provider input. The original stdin is read once so it
can be used both for the decision and, when requested, by a dispatched action.

## Dispatch and action environment

Dispatch is optional. For choice and boolean decisions, the selected value is
used as an action key (`true` or `false` for boolean). For scores, the first
inclusive range containing the result is selected. A missing action means
output-only execution, not an error.

Actions are selected from commands already declared in configuration. Decio
does not interpolate a model response into a command or execute a command
written by the provider.

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
scripts. Decio never executes provider-generated command text: providers
select only among commands already declared by the user. Choice values are
identifiers, not shell fragments, and arbitrary model output is not
interpolated into a command string.

Secrets stay in environment or secret stores and should not be included in
provider input unless explicitly collected. Source stderr is diagnostic only.
External commands and HTTP calls are time-bounded, and malformed or
out-of-range decisions are rejected before dispatch.

## License

MIT. See [`LICENSE`](LICENSE).
