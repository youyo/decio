# Decio actions

An action is selected only after the provider result has passed typed-result
validation. The command comes from trusted YAML; Decio does not interpolate a
provider-generated shell fragment.

## Resolution

- For `choice`, the result string is used as a map key and must match a
  configured choice ID.
- For `boolean`, the result is converted to the exact string key `"true"` or
  `"false"`.
- For `score`, the integer result selects the first inclusive range containing
  it. Score ranges may not overlap; an omitted `max` is open-ended.

If no key or range matches, no action runs and the decision is still emitted.
`--no-action` disables dispatch even when a matching action exists.

## Environment and stdin

Every action receives these `DECIO_*` values:

| Variable | Value |
| --- | --- |
| `DECIO_TYPE` | `choice`, `boolean`, or `score` |
| `DECIO_VALUE` | The plain result value |
| `DECIO_PROVIDER` | Provider name |
| `DECIO_MODEL` | Provider model |
| `DECIO_CONFIDENCE` | Provider confidence, or an empty value |

The configured `env` map adds or overrides environment values. These exact
placeholders are replaced before execution: `{{ result.value }}`,
`{{ result.type }}`, `{{ result.confidence }}`, `{{ result.provider }}`, and
`{{ result.model }}`. Do not put API keys or other secrets in documentation or
configuration committed to source control.

`stdin: original` forwards the original process stdin to the action. This is
useful for a hook event. `stdin: none` (the default) gives the action an empty
stdin. The original stdin is distinct from normalized, multi-source provider
input.

## Streams and exit codes

An action is run through the standard `os/exec` command mechanism. Its stdout
and stderr are both written to Decio's stderr, so Decio stdout remains reserved
for the decision result. If the action exits non-zero, Decio has already
emitted the result and returns that action exit code directly. A failure to
prepare or start dispatch uses Decio's dispatch failure code `6`.
