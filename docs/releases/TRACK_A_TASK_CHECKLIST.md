# Track A Task Checklist (Desktop Client Alpha)

This checklist turns the Track A roadmap slices into an execution plan with dependency order, estimated effort, and concrete completion checks.

How to use:
- Mark `Status` as `todo`, `in-progress`, `blocked`, or `done`.
- Keep tasks small enough to complete and validate in one pull request where practical.
- Record proof-of-completion in PR descriptions (screenshots, logs, or test output).

Estimate scale:
- `XS`: less than 0.5 day
- `S`: 0.5 to 1 day
- `M`: 1 to 3 days
- `L`: 3 to 5 days
- `XL`: more than 5 days

## Milestone Order

1. A0 Foundation
2. A1 Persistence + Identity
3. A2 Unified Layout
4. A3 Bot Registration + Orchestration
5. A4 Known Servers + Probing
6. A5 Server Context + Owner-Token Actions
7. A6 Hardening + Alpha Packaging

## Checklist

| ID | Slice | Task | Estimate | Depends On | Status | Definition of done |
| --- | --- | --- | --- | --- | --- | --- |
| A0-01 | A0 | Create Avalonia solution + project structure (`App`, `Core`, `Infrastructure`) | M | - | todo | Solution builds and launches app shell on Linux |
| A0-02 | A0 | Establish MVVM conventions and base view model infrastructure | S | A0-01 | todo | Base view model + command patterns used by at least one screen |
| A0-03 | A0 | Define client domain models (`ClientIdentity`, `BotProfile`, `KnownServer`, `ServerPluginCache`, `AgentRuntimeState`) | S | A0-01 | todo | Models compile, are serializable/mappable, and have basic validation |
| A0-04 | A0 | Add local logging/telemetry abstraction for app runtime diagnostics | S | A0-01 | todo | App writes structured local logs with levels and timestamps |
| A1-01 | A1 | Implement storage abstraction interfaces for client persistence | S | A0-03 | todo | Storage contracts support identity, bots, servers, plugin cache, runtime state |
| A1-02 | A1 | Implement SQLite-backed storage provider and startup initialization | M | A1-01 | todo | DB file is created automatically and startup does not require manual steps |
| A1-03 | A1 | Add schema version table + migration runner | M | A1-02 | todo | Migration path runs idempotently and reports current schema version |
| A1-04 | A1 | Implement first-launch client identity bootstrap + durable save | S | A1-02 | todo | First launch creates identity, restart loads same identity |
| A2-01 | A2 | Build unified main workspace view (left panel, center host, right panel) | M | A0-02, A1-02 | todo | App launches directly into unified workspace |
| A2-02 | A2 | Implement collapsible behavior for left and right panels | S | A2-01 | todo | Both panels can collapse/expand and layout remains stable |
| A2-03 | A2 | Implement context host navigation model for center activity area | M | A2-01 | todo | Selecting contexts swaps center content predictably |
| A2-04 | A2 | Create reusable card components for bot/server entries with status style hooks | S | A2-01 | todo | Card component supports title, metadata, status style, click/select |
| A3-01 | A3 | Implement bot registration/edit form in center activity area | M | A2-03, A1-02 | todo | User can add and edit bot path/args/metadata |
| A3-02 | A3 | Persist bot profiles and display them in left panel cards | S | A3-01 | todo | Bot list survives restart and renders from storage |
| A3-03 | A3 | Implement arm/disarm orchestration service for bot+agent lifecycle | L | A3-02 | todo | Arm launches processes and disarm stops them with clear result state |
| A3-04 | A3 | Wire control-channel status/lifecycle updates into bot runtime state model | M | A3-03 | todo | Runtime updates reflected in state store and visible in UI |
| A3-05 | A3 | Apply bot card glow rules (amber armed, green active session, red error) | S | A3-04, A2-04 | todo | Glows match state transitions and update in near-real time |
| A4-01 | A4 | Implement known server registration/edit flow in center activity area | M | A2-03, A1-02 | todo | User can add/edit server records with IDs and endpoints |
| A4-02 | A4 | Persist known server records + cached plugin snapshots | M | A4-01 | todo | Server and plugin cache state survives restart |
| A4-03 | A4 | Implement startup probe loop for known servers | M | A4-02 | todo | Startup probe marks servers reachable/unreachable with timeout policy |
| A4-04 | A4 | Apply server card visual states (green live, grey inactive) | S | A4-03, A2-04 | todo | Card styling matches latest probe result |
| A4-05 | A4 | Add manual refresh/reprobe action for known servers | S | A4-03 | todo | User can trigger reprobe and see updated status |
| A5-01 | A5 | Build server detail view in center activity area | M | A4-02, A2-03 | todo | Selecting a server loads details view with cached metadata |
| A5-02 | A5 | Retrieve and display agent `server_access` metadata (owner token + dashboard endpoint) | M | A3-04, A5-01 | todo | Access metadata shown for armed bot sessions and refreshable |
| A5-03 | A5 | Add owner-token-gated action stubs (create/join arena command path placeholders) | M | A5-02 | todo | Gated actions appear only when session metadata is valid |
| A5-04 | A5 | Add server plugin catalog viewer pane in center activity area | S | A5-01, A4-02 | todo | Cached plugin data is readable in UI |
| A6-01 | A6 | Add integration tests for first-launch identity + storage migration | M | A1-03, A1-04 | todo | Tests validate initialization and schema progression behavior |
| A6-02 | A6 | Add integration tests for arm/disarm/lifecycle/quit orchestration states | M | A3-04 | todo | Tests cover expected transitions and failure handling |
| A6-03 | A6 | Add resilient error handling for stale process handles and socket failures | M | A3-03 | todo | User-visible errors are clear and app recovers without restart |
| A6-04 | A6 | Document Linux build/run packaging and local development workflow | S | A2-01, A6-02 | todo | Docs allow clean setup and launch on a fresh Linux environment |
| A6-05 | A6 | Alpha readiness pass (UX sanity, data durability checks, smoke checklist) | M | A6-01, A6-02, A6-04 | todo | Internal alpha sign-off checklist completed |

## Dependency Notes

- `A3` should not begin before `A1` storage contracts stabilize, to avoid rework in runtime-state persistence.
- `A5` depends on `A3` control-channel and runtime integration for owner-token-aware behavior.
- `A6` closes gaps and should include regression checks before first public/internal alpha distribution.

## Suggested Execution Rhythm

- Sprint 1: `A0` + `A1`
- Sprint 2: `A2` + core of `A3`
- Sprint 3: finish `A3` + `A4`
- Sprint 4: `A5` + `A6` hardening
