# Decio skill

This skill teaches an agent how to build pipelines, Git hooks, CI gates, and
Claude Code Hooks with Decio. It relies on `decio docs` for reference details.

```sh
# Claude Code, for every project of the current user
cp -r skills/decio ~/.claude/skills/decio

# Claude Code, for one project only
cp -r skills/decio .claude/skills/decio

# Codex CLI
cp -r skills/decio ~/.codex/skills/decio

# Any agent that follows the Agent Skills layout, via the skills CLI
npx skills add youyo/decio
```

Other agents can read `SKILL.md` directly.
