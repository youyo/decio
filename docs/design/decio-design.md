# Decio Design Specification

**Status:** Initial design / v1 baseline\
**Implementation:** Go + Cobra\
**Core concept:** Decision I/O --- turn unstructured context into a
typed semantic decision, optionally dispatching follow-up commands.

## 1. Overview

Decio is a small, provider-agnostic CLI for making semantic decisions
inside shell workflows, hooks, CI/CD, agents, and automation.

It collects input from stdin and/or configured commands/files/literals,
asks a decision provider to return a constrained result, writes that
result in a predictable format, and can optionally execute a follow-up
command.

Jev is the first provider, not the product abstraction. Providers must
be replaceable without changing Decio configurations at the
decision/action layer.

``` text
Sources -> Collect -> Decide -> Output -> Dispatch (optional)
                         |
                         +-> Jev / future providers
```

Primary initial integration: Claude Code Hooks. Decio itself must
contain no Claude-specific assumptions.

## 2. Goals

-   Be a general Unix primitive for semantic `if`, `case`, scoring, and
    routing.
-   Support direct CLI usage without a config file.
-   Support declarative YAML configuration for reusable workflows.
-   Collect context using stdin as well as arbitrary commands.
-   Support typed decisions rather than free-form generation.
-   Execute deterministic commands after a decision when configured.
-   Keep model/provider concerns behind a stable interface.
-   Work naturally in Claude Code Hooks without becoming a
    Claude-specific tool.
-   Ship as a single Go binary with shell completion and Homebrew
    distribution.

## 3. Non-goals for v1

-   General-purpose agent framework.
-   Arbitrary LLM text generation.
-   Workflow DAG/orchestration engine.
-   Embedded expression language.
-   Claude Code plugin framework.
-   Long-running daemon/server.
-   MCP server.

These can be reconsidered later without expanding the v1 core.

## 4. Core execution model

``` text
             +----------------+
stdin ------>|                |
command ---->|    Collect     |
file ------->|                |
literal ---->|                |
             +-------+--------+
                     |
                     v
             +----------------+
             |     Decide     |
             | choice         |
             | boolean        |
             | score          |
             +-------+--------+
                     |
                     v
             +----------------+
             |     Output     |
             | text / JSON    |
             +-------+--------+
                     |
                 optional
                     |
                     v
             +----------------+
             |    Dispatch    |
             | command        |
             +----------------+
```

Execution phases are explicit:

1.  **Collect** --- resolve configured input sources.
2.  **Decide** --- send a typed request to the selected provider.
3.  **Output** --- emit the normalized result.
4.  **Dispatch** --- optionally execute action(s) associated with the
    result.

A decision without dispatch is a complete and first-class Decio
invocation.

## 5. Decision types

### 5.1 Choice

Select exactly one value from a predefined set.

``` yaml
decision:
  type: choice
  prompt: Determine the appropriate review level.
  choices:
    none:
      description: No additional review is required.
    normal:
      description: Normal review is required.
    security:
      description: Security review is required.
```

Default output:

``` text
security
```

Direct usage should also be possible:

``` bash
git diff | decio --choice none --choice normal --choice security \
  --prompt "Determine the appropriate review level."
```

### 5.2 Boolean

Return `true` or `false`.

``` yaml
decision:
  type: boolean
  prompt: Does this change require running tests?
```

``` bash
git diff | decio --boolean "Does this change require running tests?"
```

Default stdout:

``` text
true
```

Boolean false is a valid result, not an execution error. Therefore v1
should not map false to a non-zero exit code by default.

An explicit `--result-exit-code` option may map:

-   true -\> 0
-   false -\> 1
-   Decio/provider/configuration failure -\> 2+

This keeps normal machine-readable invocation safe while allowing
shell-condition usage:

``` bash
git diff | decio --boolean "Should tests run?" --result-exit-code \
  && mise run test
```

### 5.3 Score

Return a bounded numeric score.

``` yaml
decision:
  type: score
  prompt: Rate the security risk of this change.
  range:
    min: 0
    max: 100
```

``` bash
git diff | decio --score "Rate the security risk." --min 0 --max 100
```

Default output:

``` text
87
```

v1 should avoid introducing a general expression evaluator. Score-based
dispatch should use explicit ranges/thresholds.

## 6. Input model

### 6.1 Direct stdin

If no `input` section is configured and stdin is piped, Decio treats
stdin as the default input.

``` bash
git diff | decio -c review.yaml
```

### 6.2 Named sources

Reusable configurations can collect multiple sources:

``` yaml
input:
  sources:
    event:
      stdin: true

    diff:
      command: git diff

    changed_files:
      command: git diff --name-only

    policy:
      file: .decio/security-policy.md

    repository:
      literal: backend-api
```

The normalized provider input is conceptually:

``` json
{
  "event": {},
  "diff": "...",
  "changed_files": "...",
  "policy": "...",
  "repository": "backend-api"
}
```

Supported v1 source types:

-   `stdin`
-   `command`
-   `file`
-   `literal`

Exactly one source type must be specified per named source.

### 6.3 Command sources

Command sources execute before the decision.

``` yaml
input:
  sources:
    diff:
      command: git diff HEAD
      timeout: 5s
```

Requirements:

-   Capture stdout.
-   Treat non-zero exit status as input collection failure by default.
-   Capture stderr for diagnostics, but do not silently send it to the
    model.
-   Support configurable timeout.
-   Execute relative to Decio's working directory unless a
    source-specific working directory is later introduced.
-   Preserve source ordering from configuration for diagnostics, while
    providers receive named structured values.

### 6.4 stdin reuse

stdin must be read once and retained in memory so that it can both:

-   participate in the semantic decision; and
-   be forwarded to a dispatched command when requested.

This matters for hook events.

## 7. Configuration

Default filename convention:

``` text
.decio.yaml
.decio.yml
.decio/<name>.yaml
```

Explicit config:

``` bash
decio -c .decio/review.yaml
decio --config .claude/hooks/post-tool.yaml
```

Suggested v1 schema:

``` yaml
version: 1

provider:
  type: jev
  model: default

input:
  sources:
    event:
      stdin: true
    diff:
      command: git diff
      timeout: 5s

decision:
  type: choice
  prompt: Determine what should happen after this tool execution.
  choices:
    none:
      description: No follow-up is required.
    test:
      description: Relevant tests should run.
    security:
      description: Security review is required.

actions:
  test:
    command: mise run test

  security:
    command: ./scripts/security-review.sh
    stdin: original
    env:
      DECIO_DECISION: "{{ result.value }}"
```

Configuration validation happens before provider invocation.

Unknown keys should be rejected by default to catch configuration
mistakes early.

## 8. Dispatch

Dispatch is optional.

### 8.1 Choice actions

``` yaml
actions:
  test:
    command: mise run test
  security:
    command: ./security-review.sh
```

The selected choice maps directly to an action with the same key.

No matching action means "output only", not failure.

### 8.2 Boolean actions

``` yaml
actions:
  "true":
    command: mise run test
```

Both true and false may have actions, but neither is required.

### 8.3 Score actions

Avoid an expression language in v1:

``` yaml
actions:
  - min: 80
    command: ./security-review.sh
  - min: 50
    max: 79
    command: ./normal-review.sh
```

Ranges must not overlap. Configuration validation should reject
ambiguity.

### 8.4 Action environment

Decio exposes normalized metadata to commands.

Baseline environment:

``` text
DECIO_TYPE
DECIO_VALUE
DECIO_PROVIDER
DECIO_MODEL
DECIO_CONFIDENCE
```

Values not returned by a provider, such as confidence, are empty rather
than fabricated.

Template-based custom env values may be supported with a deliberately
tiny set of variables. Do not introduce a general templating language
unnecessarily.

### 8.5 Action stdin

Supported modes:

``` yaml
stdin: original
```

and:

``` yaml
stdin: none
```

`original` means the original process stdin, not the normalized
multi-source provider payload.

A future `input`/`result` mode may forward normalized collected input or
the result JSON, but v1 can start with `original|none`.

### 8.6 Action exit behavior

By default, if an action is executed, Decio returns the action's exit
code.

Provider/configuration/input failures use Decio-reserved non-zero codes.

This makes Decio transparent inside hooks and CI.

## 9. Output

### 9.1 Plain output

Plain output is optimized for shell composition:

-   choice -\> selected ID
-   boolean -\> `true` / `false`
-   score -\> number

No decoration is written to stdout.

Diagnostics go to stderr.

### 9.2 JSON output

``` bash
decio -c review.yaml --json
```

Example:

``` json
{
  "type": "choice",
  "value": "security",
  "confidence": 0.94,
  "provider": "jev",
  "model": "default"
}
```

Fields that are unavailable should be omitted or `null` according to one
documented convention. Prefer omission for optional provider-specific
metadata.

Action metadata may be added:

``` json
{
  "type": "choice",
  "value": "security",
  "provider": "jev",
  "action": {
    "executed": true,
    "exit_code": 0
  }
}
```

Provider-native response details should not leak into the stable default
schema.

## 10. Provider architecture

Core interface:

``` go
type Provider interface {
    Decide(ctx context.Context, req DecisionRequest) (DecisionResult, error)
}
```

Conceptual request:

``` go
type DecisionRequest struct {
    Type    DecisionType
    Prompt  string
    Input   map[string]InputValue
    Choices []Choice
    Range   *ScoreRange
}
```

Conceptual result:

``` go
type DecisionResult struct {
    Type       DecisionType
    Value      any
    Confidence *float64
    Provider   string
    Model      string
}
```

Provider responsibilities:

-   Translate the normalized request to provider API semantics.
-   Validate/normalize provider responses.
-   Never return a value outside the requested choice/range/type.
-   Surface provider errors without silently falling back unless
    fallback is explicitly configured.

### 10.1 Jev provider

Jev is the first implementation.

Credentials should come from environment variables and never from
committed config.

Example:

``` text
JEV_API_KEY
```

The exact provider-specific variable names should follow the upstream
API when implementation begins.

### 10.2 Future providers

Potential providers include:

-   OpenAI-compatible APIs
-   Bedrock
-   Ollama/OpenAI-compatible local endpoints
-   other System 1 / classification models

Provider selection must not change `decision`, `input`, or `actions`
semantics.

## 11. Claude Code Hooks

Claude Code Hooks are a primary integration, but implemented entirely
through ordinary stdin/config/command semantics.

Concept:

``` text
Claude Code
    |
    | Hook event JSON on stdin
    v
Decio
    |
    +-> stdin source: hook event
    +-> command source: git diff
    |
    v
typed decision
    |
    +-> output only
    `-> optional follow-up command
```

Example configuration:

``` yaml
version: 1

provider:
  type: jev

input:
  sources:
    event:
      stdin: true
    diff:
      command: git diff

decision:
  type: choice
  prompt: Determine whether this tool execution requires a follow-up.
  choices:
    none:
      description: No follow-up is needed.
    test:
      description: Tests should run.
    security:
      description: A security review should run.

actions:
  test:
    command: mise run test
    stdin: original
  security:
    command: ./scripts/security-review.sh
    stdin: original
```

Claude-specific helper commands/templates can be added later, but the
execution engine must not depend on them.

## 12. CLI design

Root:

``` text
decio
```

Primary invocation intentionally requires no subcommand:

``` bash
decio -c config.yaml
cat input | decio -c config.yaml
```

Important flags:

``` text
-c, --config <path>
--prompt <text>
--choice <value>          repeatable
--boolean <prompt>
--score <prompt>
--min <number>
--max <number>
--provider <name>
--model <name>
--json
--no-action
--result-exit-code
--timeout <duration>
```

Configuration supplies defaults; explicit CLI flags override compatible
configuration fields.

Useful supporting commands:

``` text
decio version
decio completion zsh
decio config validate -c <path>
```

Potential later commands:

``` text
decio providers
decio init
```

Avoid subcommand proliferation in v1.

## 13. Zsh completion

Cobra's completion generation should be exposed:

``` bash
decio completion zsh
```

Homebrew installation should install the generated zsh completion
automatically where practical.

Also support bash/fish completion if Cobra provides it essentially for
free, while zsh is the explicitly supported/tested shell for v1.

## 14. Repository layout

``` text
decio/
├── cmd/
│   ├── root.go
│   ├── completion.go
│   ├── config.go
│   └── version.go
├── internal/
│   ├── action/
│   ├── config/
│   ├── decision/
│   ├── input/
│   ├── output/
│   └── provider/
│       └── jev/
├── examples/
│   ├── basic-choice.yaml
│   ├── boolean.yaml
│   ├── score.yaml
│   └── claude-code/
├── .github/
│   └── workflows/
├── .mise.toml
├── .goreleaser.yaml
├── go.mod
├── go.sum
├── LICENSE
└── README.md
```

Keep provider implementations below `internal/provider` until there is a
demonstrated need for a public Go SDK.

## 15. Go implementation

-   Language: Go
-   CLI framework: Cobra
-   YAML parsing: a maintained YAML v3 implementation
-   Process execution: `os/exec` with `context.Context`
-   HTTP: standard `net/http` unless an upstream SDK materially reduces
    complexity
-   Configuration: typed structs + strict decoding/validation
-   Logging: minimal structured diagnostics to stderr; no heavy logging
    dependency initially

Prefer standard library primitives where practical.

## 16. mise

mise manages both development runtime/tool versions and repository
tasks.

Example `.mise.toml`:

``` toml
[tools]
go = "1.26"
golangci-lint = "latest"
goreleaser = "latest"

[tasks.build]
run = "go build -o dist/decio ."

[tasks.test]
run = "go test ./..."

[tasks.lint]
run = "golangci-lint run"

[tasks.fmt]
run = "gofmt -w ."

[tasks.check]
depends = ["fmt", "lint", "test"]

[tasks.release]
run = "goreleaser release --clean"
```

Pin concrete tool versions in the actual repository rather than leaving
`latest`; the above is schematic.

Expected workflow:

``` bash
mise install
mise run test
mise run lint
mise run build
mise run check
```

## 17. GoReleaser

GoReleaser handles:

-   cross-platform binaries
-   checksums
-   GitHub Releases
-   archives
-   Homebrew formula/cask publishing as appropriate
-   generated changelog/release metadata

Initial targets should include at least:

``` text
darwin/arm64
darwin/amd64
linux/arm64
linux/amd64
```

Windows can be included if it does not complicate command-dispatch
semantics; Unix-like environments remain the primary v1 target.

Release flow:

``` text
tag push
   |
   v
GitHub Actions
   |
   +-> GitHub App authentication
   |
   +-> installation token
   |
   v
GoReleaser
   |
   +-> GitHub Release
   +-> binaries/checksums
   `-> Homebrew repository update
```

## 18. GitHub App authentication for releases

Use a dedicated GitHub App rather than a long-lived PAT for automated
cross-repository Homebrew updates.

Repository/organization secrets:

``` text
APP_ID
APP_PRIVATE_KEY
```

The workflow exchanges these for a short-lived GitHub App installation
token.

That token is provided to the release/Homebrew publishing step.

Conceptual workflow:

``` yaml
- name: Generate GitHub App token
  id: app-token
  uses: actions/create-github-app-token@v2
  with:
    app-id: ${{ secrets.APP_ID }}
    private-key: ${{ secrets.APP_PRIVATE_KEY }}
    owner: <owner>

- name: Release
  env:
    GITHUB_TOKEN: ${{ steps.app-token.outputs.token }}
  run: mise run release
```

The GitHub App should receive only the minimum repository permissions
required for:

-   release repository contents/releases;
-   pushing/updating the Homebrew tap repository.

Prefer installation scoping to only the Decio and tap repositories.

## 19. Homebrew

Recommended distribution:

``` bash
brew install <owner>/tap/decio
```

with a dedicated/shared tap repository such as:

``` text
<owner>/homebrew-tap
```

GoReleaser updates the Decio formula using the short-lived GitHub App
token.

If/when Decio qualifies for Homebrew core, core submission can be
considered separately. The project must not depend on core acceptance.

## 20. Security model

Decio deliberately executes commands, so command execution is an
explicit trust boundary.

Principles:

-   Config files are executable policy and must be treated like shell
    scripts.
-   Never execute provider-generated command text.
-   Providers select only among commands already declared by the
    user/config.
-   Choice values are identifiers, not shell fragments.
-   Do not interpolate arbitrary model output into a command string.
-   Secrets remain in environment/secret stores and should not be
    included in provider input unless explicitly collected.
-   stderr from source commands is diagnostic and not automatically sent
    to providers.
-   Time out external commands and HTTP calls.
-   Never silently execute an action after malformed or out-of-range
    provider output.
-   Validate all typed decisions before dispatch.

This is a core invariant:

> **The model may choose an allowed action; it may not author the
> action.**

## 21. Failure semantics

Suggested exit-code classes:

``` text
0   Decio succeeded (including boolean false unless --result-exit-code)
1   valid negative boolean result when --result-exit-code is enabled
2   usage/configuration error
3   input collection failure
4   provider failure
5   invalid provider result
6   dispatch preparation failure
N   dispatched command's exit code where safely representable
```

Exact action-code propagation needs one documented rule to avoid
collision with Decio's reserved codes. A simple v1 approach is to
propagate the action code directly and reserve only Decio errors when no
action ran; JSON/stderr identifies the origin.

## 22. Observability

Default behavior must remain quiet and pipeline-safe.

``` text
stdout -> result
stderr -> diagnostics
```

Optional:

``` text
--verbose
```

may show:

-   config used
-   collected source names and sizes
-   provider/model
-   request duration
-   selected decision
-   action executed and duration

Do not print source contents by default because they may contain
sensitive code or hook context.

## 23. Testing strategy

Unit tests:

-   strict configuration parsing
-   choice validation
-   boolean normalization
-   score range validation
-   action mapping
-   overlapping score range rejection
-   source command failures/timeouts
-   stdout/stderr separation
-   provider response validation

Provider tests:

-   mock HTTP server
-   malformed responses
-   timeouts
-   auth errors
-   out-of-contract result

Integration tests:

``` text
stdin -> mock provider -> choice -> stdout
command source -> mock provider -> boolean
multi-source -> choice -> action
Claude-like JSON stdin -> action stdin forwarding
```

Release tests:

-   `decio version`
-   zsh completion generation
-   Linux/macOS binary smoke tests
-   GoReleaser snapshot build

## 24. v1 scope

The first useful release should contain:

1.  Go/Cobra CLI.
2.  YAML config with strict validation.
3.  stdin, command, file, literal sources.
4.  choice, boolean, score decisions.
5.  Jev provider.
6.  plain and JSON output.
7.  optional command dispatch.
8.  action stdin forwarding.
9.  action environment metadata.
10. Claude Code Hook example.
11. zsh completion.
12. mise development/runtime/task management.
13. GoReleaser releases.
14. GitHub App token flow for Homebrew tap updates.
15. macOS/Linux binaries.

## 25. Post-v1 candidates

Only add these based on concrete usage:

-   provider fallback chains
-   OpenAI-compatible provider
-   Bedrock provider
-   local/Ollama provider
-   richer score routing
-   normalized-input forwarding to actions
-   result JSON forwarding to actions
-   source transforms
-   parallel source collection
-   per-source size limits
-   redaction/filtering before provider submission
-   `decio init`
-   Claude Code setup helper
-   GitHub Actions examples
-   MCP integration
-   dry-run/explain mode
-   provider capability discovery

## 26. Product definition

**Decio = Decision I/O.**

A concise description:

> **Decio turns unstructured context into typed decisions and optional
> actions.**

A developer-oriented description:

> **A provider-agnostic semantic decision primitive for shell workflows,
> hooks, CI, and agents.**

The key abstraction is not Jev, Claude, or hooks. It is:

``` text
unstructured context
        |
        v
typed semantic decision
        |
        +-> stdout
        `-> deterministic user-defined action
```

That abstraction should remain stable as models and integrations change.
