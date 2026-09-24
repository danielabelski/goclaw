# AnySearch Skill — Maintenance Guide

Development and maintenance notes for the bundled GoClaw skill at `skills/anysearch/`.

## Provenance

| Field | Value |
|-------|-------|
| Upstream | [anysearch-ai/anysearch-skill](https://github.com/anysearch-ai/anysearch-skill) |
| Vendored tag | `v3.1.1` |
| License | Apache-2.0 (see `LICENSE`, `NOTICE`) |
| Vendored marker | `UPSTREAM.txt` |
| Skill version | 3.1.1 (frontmatter `version`) |
| Bounty claim | AnySearch Open Source Bounty — **goclaw#001** (see Disclosure) |

This skill vendors the upstream cross-platform CLIs and shared spec so GoClaw agents can search without installing an MCP server. Keep `LICENSE` and `NOTICE` whenever upstream files are redistributed.

## Disclosure

This integration is submitted under the **AnySearch Open Source Bounty** program.

| Field | Value |
|-------|-------|
| Claim ID | **goclaw#001** |
| Relationship | Bounty participant integrating AnySearch into GoClaw |
| Paid relationship with GoClaw maintainers | None |
| Prior AnySearch merge from this contributor | None |

Keep this claim disclosure in any linked PR/issue body as well (program requirement).

## Directory layout

```text
skills/anysearch/
├── SKILL.md                 # Agent instructions + frontmatter (GoClaw seeder)
├── MAINTENANCE.md           # This document
├── LICENSE                  # Apache-2.0 (upstream)
├── NOTICE                   # Copyright notice (upstream)
├── SECURITY.md              # Vulnerability reporting (upstream)
├── UPSTREAM.txt             # Pin: repo + tag + vendored date
├── .env.example             # ANYSEARCH_API_KEY template (never commit .env)
├── requirements.txt         # Python CLI: requests>=2.20
└── scripts/
    ├── anysearch_cli.py     # Primary CLI (Python + requests)
    ├── anysearch_cli.js     # Node.js CLI (no third-party deps)
    ├── generate.py          # Upstream regenerator (also writes sh/ps1; we only ship py/js)
    ├── test_cli.py          # Offline fixture tests
    └── shared/
        ├── constants.json   # API endpoint + vertical domain list
        └── doc_spec.md      # Interface spec rendered by `doc`
```

Do **not** commit `.env` or `runtime.conf`.

## Capability map (bounty / product)

| Capability | CLI | HTTP |
|------------|-----|------|
| General search (incl. anonymous) | `search` | `POST /v1/search` |
| Parallel batch search | `batch_search` (1–5) | N × `POST /v1/search` (bounded in-flight) |
| Vertical domain search | `get_sub_domains` then `search --domain/--sub_domain/--params` | `GET /v1/sub-domains` + `POST /v1/search` |
| Page extract | `extract` | `POST /v1/extract` |

Auth header: `Authorization: Bearer <ANYSEARCH_API_KEY>` (optional). Anonymous works with lower rate limits.

## Runtime / dependency matrix (GoClaw images)

| Image variant | Python | Node | Recommended CLI |
|---------------|--------|------|-----------------|
| `latest` | yes | no | `anysearch_cli.py` + `pip3 install requests` |
| `full` | yes | yes | Python or Node |
| `base` | no | no | Install Python + `requests`, or run on a host with Node.js |
| Desktop / bare binary | host-dependent | host-dependent | Detect via `runtime.conf` |

Python dependency is declared in `requirements.txt` (`requests>=2.20`). GoClaw's `dep_scanner` will surface `pip:requests` if missing; install with `pip3 install -r requirements.txt` (or `pip3 install requests`).

## Syncing from upstream

1. Pick a released tag from https://github.com/anysearch-ai/anysearch-skill/releases
2. Replace `scripts/**`, `LICENSE`, `NOTICE`, `SECURITY.md`, `.env.example`, `requirements.txt`
3. Re-apply GoClaw-specific `SKILL.md` header/notes (keep decision flow and safety text)
4. Update `UPSTREAM.txt` and frontmatter `version`
5. Run offline tests and a live smoke (see below)
6. Open a PR to GoClaw `dev`

GoClaw intentionally vendors **only** the Python and Node ports (review feedback on PR #1578: four parallel CLIs enlarge the maintenance/security surface). `scripts/generate.py` (upstream) can regenerate all four ports — run it only when changing `scripts/shared/`, then keep `anysearch_cli.py` / `anysearch_cli.js` and drop the Sh/PS outputs again.

## Testing

### Offline (no API key, no network fixtures)

```bash
python3 scripts/test_cli.py
# or limit runtimes:
python3 scripts/test_cli.py --runtime Python,Node
```

### Live smoke

```bash
export ANYSEARCH_API_KEY=...   # optional
python3 scripts/anysearch_cli.py search "hello world" --max_results 1
python3 scripts/anysearch_cli.py batch_search --query "hello" --query "world" --max_results 1
python3 scripts/anysearch_cli.py get_sub_domains --domain finance
python3 scripts/anysearch_cli.py extract "https://example.com"
```

### GoClaw regression (when this skill is the only change)

```bash
go build ./...
go build -tags sqliteonly ./...
go vet ./...
go test -race ./...
```

No schema/migration/UI changes are required for this skill.

## Security checklist

- [ ] No committed API keys or tokens
- [ ] `.env` gitignored; only `.env.example` in tree
- [ ] No `curl | sh` or remote-code install paths
- [ ] Extract/search results documented as untrusted data
- [ ] Outbound API limited to `https://api.anysearch.com` (see `scripts/shared/constants.json`)
- [ ] `LICENSE` + `NOTICE` retained for Apache-2.0 compliance

## Licensing notes for GoClaw

GoClaw's repository license is CC BY-NC 4.0 (plus a commercial license for production use). This skill directory contains Apache-2.0 upstream code. When redistributing:

1. Keep `LICENSE` and `NOTICE` intact.
2. Do not relicense upstream files as GoClaw proprietary code.
3. Record the upstream pin in `UPSTREAM.txt`.
4. If maintainers require zero Apache-2.0 files in-tree, switch to a thin wrapper that downloads a pinned upstream release at install time (strategy L2).

**L1 open question (maintainer decision):** accept Apache-2.0 files under `skills/anysearch/` inside the CC BY-NC repository when items 1–3 are followed? Default shipping strategy is **L1** (vendored sources + retained license files). Fallback is **L2** if L1 is rejected. Ask this in the proposal issue; do not treat silence as approval.

## Release checklist for a PR touching this skill

- [ ] `SKILL.md` frontmatter parses; `name: anysearch`
- [ ] Four CLIs present and `doc` works on the primary runtime
- [ ] `scripts/test_cli.py` passes offline
- [ ] Live smoke sample pasted in the PR (redact keys)
- [ ] `docs/anysearch-skill.md` updated if commands/behavior changed
- [ ] `UPSTREAM.txt` matches vendored contents
- [ ] GoClaw gates green (build / sqliteonly / vet / test)
