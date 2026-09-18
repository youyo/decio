# Decio output and exit codes

## Plain output

Without `--json`, stdout contains only the typed result value followed by a
newline:

```text
normal
true
89
```

Action output and diagnostic details do not mix with this stdout stream; both
go to stderr.

## JSON output

With `--json`, stdout contains one object with this stable shape:

```json
{
  "type": "choice",
  "value": "security",
  "confidence": 0.92,
  "provider": "jev",
  "model": "jev-latest",
  "action": {
    "executed": true,
    "exit_code": 0
  }
}
```

`type`, `value`, `provider`, and `model` are always present. `confidence` is
omitted when the provider does not return it. `action` is omitted when no
matching action ran, including when dispatch was disabled with `--no-action`.
When an action ran, `action.executed` is `true` and `action.exit_code` is its
exit code.

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Decio succeeded, including a `false` boolean unless `--result-exit-code` is enabled. |
| `1` | A valid `false` boolean result with `--result-exit-code`, when no action ran. |
| `2` | Usage or configuration error. |
| `3` | Input collection failure. |
| `4` | Provider failure. |
| `5` | Invalid or out-of-range provider result. |
| `6` | Action dispatch preparation failure. |
| `N` | The exit code returned by a dispatched action. |

`--result-exit-code` changes only the no-action, valid-false boolean case. A
valid `true` result remains `0`; provider, input, configuration, result, and
action failures keep their own codes. An action's non-zero code is propagated
directly even if it numerically overlaps a Decio error code.
