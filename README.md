# agentpack

**A control plane for AI coding agent configuration.**

Organizations using AI coding agents (Claude Code, Cursor, Copilot, …) face the
same problem: rules, skills, and agent definitions live per-repository, teams
own ten or more repos each, and org-wide conventions drift the moment they are
copy-pasted. agentpack distributes that configuration by **identity** instead
of by repository.

- An organization keeps versioned **packs** (rules, skills, agents, memories)
  in one git repository, mapped to **groups**: org-wide, per team, per role.
- Developers run `agentpack login` once. Every `agentpack sync` resolves which
  packs apply to *them* and materializes the content into each AI tool's
  native surfaces — Claude Code's user-level skills, agents, and `CLAUDE.md`
  today, plus a merged `AGENTS.md` for tools that read the cross-vendor
  standard.
- When someone updates a team pack, the whole team gets it on their next sync.
  When someone changes teams, packs that no longer apply are **pruned**.

Per-repository configuration (a repo's own `CLAUDE.md`/`AGENTS.md`) stays
where it is; agentpack covers everything *above* the repository level.

## Install

Prebuilt binaries for Windows, macOS, and Linux (amd64/arm64) are attached to
each [GitHub Release](https://github.com/doriansobacki/agentpack/releases).
With a Go toolchain, `go install github.com/doriansobacki/agentpack@latest`
also works.

Scoop, Homebrew tap, and winget manifests are wired into the release
pipeline and will be activated when the repository goes public (a private
repo's release assets are not downloadable by package managers).

## Quickstart

```sh
# 1. An admin scaffolds and publishes the org config repo
agentpack init my-org-config
cd my-org-config && git init && git add -A && git commit -m "org config" && git push ...

# 2. Every developer, once
agentpack login dev@example.com --source https://github.com/my-org/my-org-config

# 3. Whenever (or on a schedule / login script)
agentpack sync
```

`agentpack sync --dry-run` shows what would change; `agentpack status` shows
the current profile and what the last sync produced.

## Automatic sync

Nobody should have to remember to run sync. Register agentpack with the
operating system's own per-user scheduler — Windows Task Scheduler, macOS
launchd, or Linux systemd user timers; no daemon, no admin rights:

```sh
agentpack service install --interval 5m
agentpack service status      # the job as the OS scheduler sees it
agentpack service uninstall
```

Scheduled runs are headless and append one summary line per sync to
`<agentpack home>/logs/sync.log`. A lock file serializes concurrent syncs, so
a manual `agentpack sync` and a scheduled one never interleave.

For a foreground alternative (a terminal you keep open, a container), use
`agentpack watch --interval 5m`, which backs off on repeated failures.

## The org config repository

```
agentpack.yaml            # the manifest: identity, targets, groups, users
packages/
  org-baseline/           # one directory per pack
    package.yaml          # name + description (optional)
    rules/*.md            # instructions -> CLAUDE.md managed block, AGENTS.md
    memories/*.md         # org/team vocabulary and facts -> same surfaces
    skills/<name>/        # Claude Code skills (SKILL.md + support files)
    agents/*.md           # Claude Code subagent definitions
```

```yaml
# agentpack.yaml
identity:
  provider: static        # group membership from the users: map below

targets: [claude, agentsmd]

groups:
  "*": [org-baseline]     # everyone
  team-a: [team-a-core]
  team-a/backend: [dotnet]
  team-a/frontend: [react]

users:
  alice@example.com: [team-a, team-a/backend]
  bob@example.com: [team-a, team-a/frontend]
```

Hierarchy is expressed by giving a user several groups; packs resolve in
order (org-wide first, then team, then role) and duplicates collapse.

## What a sync does

1. Fetches the org config (local path, or git clone/pull into a cache).
2. Resolves your identity and groups via the configured **identity provider**.
3. Maps groups to an ordered pack list and loads the pack contents.
4. Runs every configured **target**, each writing its tool's surfaces.
5. Prunes files written by a previous sync that no longer apply.
6. Records state, so the next sync knows what agentpack owns.

Two safety properties hold throughout: agentpack never overwrites a file it
did not create (it warns and skips), and in `CLAUDE.md` it only ever touches
the content between its `<!-- agentpack:begin/end -->` markers.

## Extending agentpack

agentpack is deliberately open at the two seams that vary between
organizations:

**Identity providers** (`pkg/identity`) answer "who is this user, and which
groups do they belong to?" Built-ins: `static` (reads the manifest's `users:`
map) and `entra` (Microsoft Entra ID: device-code sign-in, groups from token
claims with a Microsoft Graph fallback — see the
[admin guide](docs/entra.md)). Any directory service can be added by
implementing a two-method interface and registering it.

**Targets** (`pkg/target`) answer "how does this tool consume configuration?"
Built-ins: `claude` (full fidelity: skills, agents, managed CLAUDE.md block),
`agentsmd` (merged AGENTS.md), `cursor` (experimental: paste-ready User Rules
file, until Cursor exposes a writable user-level surface). Targets receive
packs as plain data, decoupled from the repo layout.

Both extension points are registries: a custom build registers additional
implementations in its `main` and everything else stays intact. An exec-based
protocol for out-of-tree extensions (separate `agentpack-provider-*` /
`agentpack-target-*` binaries) is on the roadmap for when the interfaces
stabilize.

## Roadmap

- **Copilot target**: org custom instructions via the GitHub API.
- **Cursor**: push Team Rules through the dashboard API.
- **Watch mode / background daemon**: near-real-time propagation.
- **Claude Code plugin generation**: emit a plugin marketplace from packs.
- **VS Code extension**: a thin UX shell over this CLI (sign-in prompt,
  status bar, update toasts).
- **Out-of-tree extensions**: exec-based provider/target protocol.

## Development

```sh
go build ./...
go test ./...
```

The `examples/org-config` directory is a complete sample org config; try it
without touching your real setup:

```sh
AGENTPACK_HOME=/tmp/aphome AGENTPACK_CLAUDE_DIR=/tmp/apclaude \
  go run . login you@example.com --source ./examples/org-config
AGENTPACK_HOME=/tmp/aphome AGENTPACK_CLAUDE_DIR=/tmp/apclaude \
  go run . sync
```

## License

MIT — see [LICENSE](LICENSE).
