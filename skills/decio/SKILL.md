---
name: decio
description: Build pipelines, Git hooks, CI gates, and Claude Code Hooks with decio, a CLI that turns context into typed decisions (choice/boolean/score) and dispatches pre-declared actions. Use when asked to gate, classify, or score changes with decio, or to wire decio into hooks or CI.
---

# Decio

Use Decio when a workflow needs a typed decision from trusted context and may
optionally dispatch a reviewed action. Keep detailed syntax and recipes in the
embedded documentation; use this skill for the workflow and safety checks.

## Before you start

1. Confirm the CLI is installed without exposing credentials:

   ```sh
   decio version
   ```

   If it is unavailable, install it with `brew install youyo/tap/decio` or
   `go install github.com/youyo/decio@latest`.
2. Check that `TYPESAFE_API_KEY` is set without printing its value:

   ```sh
   test -n "${TYPESAFE_API_KEY:-}" && echo "TYPESAFE_API_KEY is set" || echo "TYPESAFE_API_KEY is not set" >&2
   ```

3. Discover the embedded references, then read the topics relevant to the task.
   Always read `overview` and `config`; for a hook also read `claude-code`, and
   for CI or a Git hook also read `pipelines`:

   ```sh
   decio docs --list
   decio docs overview
   decio docs config
   decio docs claude-code
   decio docs pipelines
   ```

## Workflow

1. Choose the decision type: use `boolean` for yes/no, `choice` for one value
   from known alternatives, and `score` when a threshold determines the branch.
2. For a one-off decision with no actions or multiple sources, use CLI flags.
   When actions or multiple sources are needed, start from a template with
   `decio init --type <t> -o <path>`.
3. Write a focused prompt and explicit choices or score range. Actions must
   point only to existing, trusted commands declared in reviewed configuration.
4. Validate configuration before calling the provider:
   `decio config validate -c <path>`.
5. Run representative inputs with `--no-action` and inspect the typed result.
   Add `--json --verbose` when structured output or diagnostics are needed.
6. Wire the validated command into the hook, CI job, Git hook, or script. Use
   absolute paths where the runner may have a different working directory, and
   pass required environment variables through the runner.
7. Execute the real integration path and verify both the result and its exit
   code at the boundary that consumes them.

## Guardrails

- Never put model output or a decision result into an interpolated shell
  command. Actions are pre-declared; the model may select an allowed action but
  may not author one.
- Never place `TYPESAFE_API_KEY` in YAML, logs, commits, JSON output, or test
  fixtures. Check only whether it is set, never print its value.
- In a Claude Code Hook, treat exit code 2 as blocking and confirm that the
  hook's consumer handles it as intended.
- stdout is reserved for the typed result. Do not expect action output on
  stdout; use stderr for diagnostics.
- `--result-exit-code` changes only a boolean `false` result without an action
  to exit code 1. Configuration, input, provider, and action failures have
  separate failure codes.
- Unknown configuration keys are errors. Fix the configuration instead of
  silently ignoring them.

## Quick reference

Topics exposed by `decio docs --list`:

| Topic | Description |
| --- | --- |
| `overview` | Decio's model, boundaries, and when to use it |
| `cli` | Root flags, subcommands, and configuration discovery |
| `config` | The YAML schema, constraints, and complete examples |
| `actions` | Action resolution, environment, stdin, and exit codes |
| `output` | Plain and JSON output shapes and exit-code semantics |
| `claude-code` | Claude Code Hook integration recipes |
| `pipelines` | Git, CI/CD, shell, and chained-decision recipes |
| `troubleshooting` | Common failures, diagnostics, and validation |

Common commands:

```sh
decio version
decio docs --list
decio docs overview
decio init --type choice -o .decio.yaml
decio config validate -c .decio.yaml
decio --config .decio.yaml --no-action --json --verbose
```
