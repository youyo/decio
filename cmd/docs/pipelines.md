# Decio pipeline recipes

Each recipe has a purpose, a configuration or command, and an expected
behavior. Keep action commands in reviewed configuration when dispatch is
needed; use direct CLI flags when a pipeline only needs a typed result.

## Git pre-commit

Purpose: gate a commit when the staged diff requires tests.

Configuration or command:

```sh
#!/bin/sh
git diff --cached | decio \
  --boolean "Does this staged change require running tests?" \
  --result-exit-code
```

Expected behavior: a valid `true` result exits `0`; a valid `false` result
exits `1`; input, provider, or configuration failures use their documented
codes and stop the commit.

## Git pre-push

Purpose: classify the outgoing diff and require a security review for a
security choice.

Configuration or command:

```sh
#!/bin/sh
choice=$(git diff origin/main...HEAD | decio \
  --choice none --choice normal --choice security \
  --prompt "Determine the appropriate review level.") || exit $?
case "$choice" in
  security) ./scripts/security-review.sh || exit $? ;;
  none|normal) exit 0 ;;
  *) exit 5 ;;
esac
```

Expected behavior: the bounded choice selects one case; no provider text is
executed as shell syntax.

## GitHub Actions

Purpose: install the released binary, score a change, and use JSON metadata in
a CI gate.

Configuration or command:

```yaml
steps:
  - uses: youyo/decio@v0
  - name: Check risk score
    shell: bash
    run: |
      git diff HEAD~1 | decio --score "Rate the security risk of this change." --min 0 --max 100 --json | jq -e '.value < 60'
  - name: Require a positive boolean gate
    run: git diff HEAD~1 | decio --boolean "Should tests run?" --result-exit-code
```

Expected behavior: the action installs a released `decio` binary, `jq` reads
the JSON result from stdout, and `--result-exit-code` makes a valid false gate
fail while preserving Decio's other failure codes.

## Shell variables and cases

Purpose: use a plain typed result in a shell script.

Configuration or command:

```sh
value=$(git diff | decio --boolean "Should tests run?") || exit $?
case "$value" in
  true) mise run test ;;
  false) echo "tests not required" ;;
  *) echo "unexpected decision: $value" >&2; exit 5 ;;
esac
```

Expected behavior: `value` contains only `true` or `false` plus the command
substitution's removed newline; action and diagnostic output remain on stderr.

## Chained decisions

Purpose: narrow a workflow with a boolean gate before asking for a more
specific choice.

Configuration or command:

```sh
if git diff | decio --boolean "Does this change need review?" --result-exit-code; then
  review=$(git diff | decio \
    --choice none --choice normal --choice security \
    --prompt "Which review level is needed?") || exit $?
  case "$review" in
    security) ./scripts/security-review.sh ;;
    normal) ./scripts/normal-review.sh ;;
    none) exit 0 ;;
    *) exit 5 ;;
  esac
else
  code=$?
  [ "$code" -eq 1 ] && exit 0
  exit "$code"
fi
```

Expected behavior: only a valid `true` boolean continues to the choice; a
valid `false` result is handled as “no review”, while operational failures are
propagated.
