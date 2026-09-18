# Decio with Claude Code Hooks

Claude Code invokes command hooks with a JSON event on stdin. Configure Decio
to read that event with `stdin: true`, and use an independent command source
when the decision also needs the current repository state. The action can
forward the untouched hook event with `stdin: original`.

## Hook settings

Use real Claude Code event names and an absolute path to the binary and
configuration. Add only the events that the workflow needs:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Bash|Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "/absolute/path/to/decio -c /absolute/path/to/examples/claude-code/post-tool.yaml"
          }
        ]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "/absolute/path/to/decio -c /absolute/path/to/pre-tool.yaml"
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/absolute/path/to/decio -c /absolute/path/to/stop.yaml"
          }
        ]
      }
    ]
  }
}
```

`PostToolUse`, `PreToolUse`, and `Stop` are event names. A matcher is used for
tool events; it is not needed for `Stop`.

## Stdin plus repository context

This configuration matches `examples/claude-code/post-tool.yaml`: the hook
event is one provider input and `git diff` is a second input. The choices and
actions are pre-declared, and both follow-up actions receive the original hook
JSON.

```yaml
version: 1

provider:
  type: jev
  model: jev-latest

input:
  sources:
    event:
      stdin: true
    diff:
      command: git diff
      timeout: 5s

decision:
  type: choice
  prompt: Determine whether this tool execution requires a follow-up.
  choices:
    none:
      description: No follow-up is needed.
    test:
      description: Relevant tests should run.
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

The hook event is ordinary original stdin, for example:

```text
{"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"git status"}}
```

The command source runs separately and contributes its stdout to provider
input. It does not replace or consume the hook event.

## Hook exit codes and security

The hook process observes Decio's process exit code. `0` is success, `2` is a
usage/configuration error, `3` input collection failure, `4` provider failure,
`5` invalid provider result, and `6` dispatch preparation/start failure. If a
declared action runs, its exit code is propagated directly; a non-zero action
code therefore makes the hook command fail with that code. Use
`--result-exit-code` only when a false boolean result should be a hook failure
without an action.

Claude Code gives exit codes their own meaning: `0` succeeds, `2` is a
blocking error whose stderr is fed back to Claude, and any other non-zero code
is a non-blocking error. A Decio configuration error (`2`) or an action that
exits `2` therefore blocks the tool call, while `--result-exit-code` on a false
boolean (`1`) does not. Pick action exit codes with this mapping in mind.

Use absolute paths because a hook's working directory and `PATH` may differ
from an interactive shell. Ensure the hook process can read
`TYPESAFE_API_KEY`; provide it through the environment or the secret mechanism
used by the host. Never put the key in YAML, a hook command, logs, JSON output,
or a test fixture.
