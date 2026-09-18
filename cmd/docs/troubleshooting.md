# Decio troubleshooting

Use `decio config validate -c path/to/config.yaml` before invoking a provider.
It catches unknown YAML keys, invalid decision shapes, missing choices,
invalid action keys, overlapping score ranges, and other configuration errors.

## Common failures

| Symptom | Exit code | Check |
| --- | --- | --- |
| Provider key is not configured or the provider cannot authenticate | `4` | Ensure `TYPESAFE_API_KEY` is available to the process and the provider settings are correct. |
| YAML is unknown, malformed, or semantically invalid | `2` | Run `decio config validate -c path/to/config.yaml`; check source kinds and decision constraints. |
| A command or file input cannot be collected | `3` | Run the input command manually and check its path and timeout. |
| The provider returned a malformed, disallowed, or out-of-range value | `5` | Check the decision type, choices, range, and provider response contract. |
| A declared action returns non-zero | The action's code | Inspect action output on stderr; Decio propagates the action code. |

## Diagnostics

Pass `--verbose` to write configuration discovery, source sizes, provider,
decision, and action timing details to stderr. It does not change the result
format on stdout. Do not put secrets in prompts or input merely to make them
visible in diagnostics.

If automatic discovery is surprising, pass `-c`/`--config` explicitly. Decio
checks `.decio.yaml` before `.decio.yml`; an explicit path overrides both.
Use `--no-action` to inspect a decision without dispatching a configured
command. Use `--json` when a script needs provider metadata or action outcome.
