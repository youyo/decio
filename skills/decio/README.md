# Decio skill

This skill teaches an agent how to build pipelines, Git hooks, CI gates, and
Claude Code Hooks with Decio. It relies on `decio docs` for reference details.

Install it with a skill installer:

```sh
gh skill install youyo/decio --scope user --agent claude-code   # GitHub CLI 2.90+
npx skills add youyo/decio -g -a claude-code -a codex           # Skills CLI
```

Or copy this directory into the skills folder your agent reads, for example
`~/.claude/skills/decio` (Claude Code), `~/.codex/skills/decio` (Codex CLI),
or the shared `~/.agents/skills/decio`. The README of the main repository lists
the directory for each agent. Other agents can read `SKILL.md` directly.
