# eyeful

> "Given enough eyeballs, all bugs are shallow." — Linus's Law

eyeful reviews code changes with a group of agents. A planner picks the experts a change needs, the
experts review in parallel, and a finding counts as verified only when eyeful has run its
reproduction and seen it fail. Findings come back as review comments; eyeful never approves.

It runs on your machine with the coding agent you already use (Claude Code, Codex or pi), from the
command line or a desktop app. Cloud reviews of pull requests are planned.

```sh
mise install && mise run setup
mise run install          # puts eyeful on your PATH
eyeful connect claude     # or codex, pi
eyeful review             # review your uncommitted changes
```

Documentation: <https://pleasurecruise.github.io/eyeful/>, starting with
[Running locally](docs/LOCAL.md). eyeful is a final-year project; the [roadmap](docs/ROADMAP.md)
says what each version delivers.

## License

[AGPL-3.0-only](LICENSE).
