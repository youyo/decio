# Decio CLI

The root command accepts a decision directly, or a configuration file can
provide the same values. The current root help is:

```text
make a typed decision from context

Usage:
  decio [flags]
  decio [command]

Available Commands:
  completion  generate shell completion
  config      configuration commands
  docs        read embedded agent documentation
  help        Help about any command
  init        write a configuration template
  version     print version

Flags:
      --boolean string       boolean decision prompt
      --choice stringArray   allowed choice (repeatable)
  -c, --config string        configuration file
  -h, --help                 help for decio
      --json                 write JSON output
      --level stringArray    score level (repeatable)
      --max int              maximum score
      --min int              minimum score
      --model string         provider model
      --no-action            skip configured action
      --prompt string        decision prompt
      --provider string      provider name (default "jev")
      --result-exit-code     return 1 for a false boolean result without an action
      --score string         score decision prompt
      --timeout duration     decision timeout (default 30s)
      --verbose              write diagnostic details to stderr

Use "decio [command] --help" for more information about a command.
```

The root flags are:

- `--choice` may be repeated and defines a `choice` decision.
- `--boolean` defines a boolean decision prompt.
- `--score` defines a score decision prompt.
- `--min` and `--max` define the inclusive score range.
- `--level` may be repeated to define ordered score levels.
- `--prompt` sets the prompt for the decision.
- `-c`/`--config` selects a YAML configuration file.
- `--json` emits JSON instead of the plain result value.
- `--no-action` skips action dispatch.
- `--result-exit-code` returns `1` for a valid false boolean result when no
  action ran.
- `--timeout` sets the decision context timeout; the default is `30s`.
- `--provider` selects the provider; the default is `jev`.
- `--model` selects the provider model.
- `--verbose` writes diagnostic details to stderr.
- `-h`/`--help` prints command help.

When `-c`/`--config` is omitted, Decio looks in the current directory for
`.decio.yaml`, then `.decio.yml`. An explicitly supplied config path wins over
automatic discovery. After the file is loaded, explicitly supplied compatible
CLI flags override its decision, provider, model, and score-level fields.
The mutually exclusive type flags are `--choice`, `--boolean`, and `--score`.

Supporting commands are:

- `decio init` writes a commented `choice`, `boolean`, or `score` template.
- `decio config validate -c path.yaml` validates YAML without calling the
  provider.
- `decio completion <shell>` generates shell completion for zsh, bash, fish,
  or PowerShell.
- `decio version` prints the version.
- `decio docs [topic]` prints this embedded reference; use `--list` to list
  topics.
