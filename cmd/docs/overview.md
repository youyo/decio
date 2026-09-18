# Decio overview

Decio turns trusted context into a typed decision and optionally dispatches a
pre-declared action. Its execution boundary is:

```text
Input → Decision → Action
```

Input collects context from stdin, a command, a file, or a literal. Decision
asks the configured provider for exactly one typed value. Action selects a
command that was already declared in trusted configuration.

## Typed decisions

Decio supports three decision types:

- `choice` selects one identifier from at least two configured choices.
- `boolean` returns `true` or `false`.
- `score` returns an integer inside an inclusive `min`/`max` range and may use
  ordered qualitative `levels`.

The result is a value, not a shell fragment. A provider result can select only
among configured choices or action ranges; Decio never treats free-form model
output as a command.

## Action boundary and streams

Actions are pre-declared in YAML. Decio runs the configured command only after
the typed result has been validated and matched. It does not generate a shell
command from the result or execute arbitrary provider output.

The decision result is written to stdout. Plain output is one value followed by
a newline; `--json` writes one JSON object. Action stdout and stderr are both
routed to Decio's stderr, leaving stdout safe for pipelines and machine
readers. Diagnostics from `--verbose` also go to stderr.

## When to use Decio

Use Decio when a workflow needs a bounded decision in a Unix pipeline, Git
hook, CI/CD gate, agent loop, or automation script. It is useful when the
allowed outcomes and follow-up commands can be declared ahead of time.

Do not use Decio as a general text-generation tool, as an evaluator of
untrusted command strings, or when a provider must invent and execute arbitrary
operations. Put executable policy in reviewed configuration and keep secrets
in the environment or a secret store.
