# Architecture Refactoring Roadmap Design

## Goal

Create a low-risk refactoring roadmap for the `oc` Go CLI/TUI codebase that improves maintainability and extension speed without changing launcher behavior, user-visible controls, or current package-level architecture.

The roadmap should identify the highest-value structural targets first, explain why they are risky today, and define a sequence that balances safety, clearer ownership, and future extensibility.

## Current Context

Recent commits already moved the codebase toward clearer boundaries:

- `internal/app` is split across orchestration, dependency wiring, iteration-state loading, and launch helpers.
- `internal/tui` already split some stats rendering, navigation, and loading helpers into sibling files.
- `internal/stats` already split overview math, report types, and window reports into separate files.

That means the next wave should continue the existing refactoring style instead of introducing a new architectural direction.

The main remaining hotspots today are:

- `internal/stats/stats.go` at about 672 lines
- `internal/stats/window_reports.go` at about 612 lines
- `internal/tui/model.go` at about 464 lines
- `internal/tui/model_mode_actions.go` and `internal/tui/model_modes.go` as medium-sized control-flow files that are clearer than before, but still central enough to deserve explicit ownership rules

This suggests the repository is not blocked by missing packages or missing abstractions. It is blocked by a few concentrated files that still combine multiple reasons to change.

## Problem Statement

The problem is not line count alone. The problem is responsibility concentration inside files that combine domain concepts, orchestration, infrastructure access, and presentation concerns.

The highest-risk examples are:

- `internal/stats/stats.go` still mixes package entrypoints, shared query helpers, row scanning, event merging, and report assembly concerns.
- `internal/stats/window_reports.go` still combines window query orchestration, SQL-facing projections, and report shaping.
- `internal/tui/model.go` still concentrates shared state, styles, formatting helpers, and view-adjacent behavior in one place.

When these files keep growing, future feature work becomes slower because contributors need to understand too many unrelated details before making a safe change.

## Success Criteria

The user selected a balanced outcome:

- preserve behavior and test safety
- materially improve ownership boundaries
- make future feature work easier without forcing a large rewrite

This roadmap succeeds if:

- the refactoring order is clear and low risk
- `internal/stats` becomes easier to extend without mixing domain calculations with DB-facing concerns
- `internal/tui` makes mode and rendering responsibilities easier to locate
- package boundaries and exported APIs stay stable during the first wave
- future contributors can identify where new report logic, TUI state logic, and helper code should live without regrowing current hotspots

## Design Principles

- Preserve existing package boundaries. This roadmap stays within current packages and prefers focused sibling files over new packages.
- Prefer extraction of cohesive helper groups over renaming concepts or redesigning APIs.
- Keep stable entrypoints in place: `RunWithDeps` for app orchestration, `Model` for the main TUI model, and current `internal/stats` exported loaders.
- Refactor by responsibility, not by arbitrary file-size targets.
- Use existing tests as the primary behavior guard.
- Introduce only a thin domain-model separation: separate core concepts and pure calculations from DB, query, JSON, and TUI infrastructure concerns, but do not build a heavyweight domain layer.

## Non-Goals

- No user-visible UX change
- No TUI key binding change
- No launcher flow change
- No new package tree in this roadmap
- No config format or persistence semantic change
- No cost/pricing behavior rewrite as part of the structural pass
- No broad clean-architecture layer rewrite across the whole repository

## Recommended Approach

Three approaches were considered:

1. Incremental file decomposition inside existing packages
2. Stronger package or layer redesign
3. Test-first restructuring before any production moves

The recommended approach is option 1.

Why:

- it matches the repository's recent commit history
- it gives the best balance of safety and visible structure improvement
- it avoids over-engineering a codebase whose main complexity is orchestration, TUI state, config editing, and stats aggregation rather than deep enterprise domain rules

The roadmap therefore keeps package boundaries intact and improves responsibility boundaries inside those packages.

## Thin Domain Model Separation

This roadmap adds a narrow domain-model rule, especially in `internal/stats`.

The goal is not to create a new `domain` package or formal application layer. The goal is to keep domain concepts and pure calculations from being tangled with infrastructure-heavy logic.

### Where It Applies Strongly

`internal/stats` is the main target.

Domain-side concepts already exist there:

- `Report`
- `WindowReport`
- `YearMonthlyReport`
- `MonthDailyReport`
- `Day`
- summary, streak, ranking, and session-gap calculations

Those concepts should stay easy to understand without needing to read SQLite queries or JSON extraction logic.

### What Moves Away From Domain-Centric Code

The following concerns should live in query or merge helpers rather than next to domain calculations:

- SQL text assembly
- row scanning
- JSON field extraction
- message/part event decoding
- scoped-directory SQL conditions
- DB-specific merge plumbing

### Where It Applies Lightly

`internal/tui` should not gain a formal domain layer. The TUI is primarily a presentation and interaction package.

The useful separation there is lighter:

- keep mode routing and state transitions distinct from rendering helpers
- keep style and formatting helpers distinct from action handlers
- keep async message handling distinct from direct state mutation helpers

### Optional Follow-Up Areas

`internal/plugin` and `internal/port` already behave like small domain-focused packages. They do not need a roadmap-driven rewrite, but their pure rules should stay isolated if touched later.

## Priority Decisions

### Priority 1: `internal/stats`

This is the first target because it is both structurally large and the best place to apply thin domain-model separation.

`internal/stats` owns multiple concerns at once:

- report types and summary structures
- aggregation entrypoints
- shared metric math and post-processing
- SQLite-facing merge/query helpers
- date/window-specific report assembly
- pricing-backed cost estimation coordination

Desired outcome:

- keep the `internal/stats` package intact
- keep exported entrypoints stable
- reduce `stats.go` to package entrypoints and only the most central cross-report helpers
- move message/part merge logic and shared DB-facing helpers into focused sibling files
- keep pure summary and report-shaping calculations distinct from SQL- and row-scan-heavy helpers

Candidate file direction:

- `stats.go`: package front door and top-level loaders
- `stats_merge_messages.go`: message-based aggregation helpers
- `stats_merge_parts.go`: part-based aggregation helpers
- `stats_query_shared.go`: scoped directory clauses, time helpers, and query fragments shared across report builders
- existing `overview_reports.go`, `summary_math.go`, `report_types.go`, `litellm_pricing.go`: retain their current role with sharper boundaries

This is intentionally conservative. It improves ownership without forcing new packages or public API churn.

### Priority 2: `internal/tui`

This is the second target because the TUI already has partial decomposition and now mainly needs clearer file ownership around state, helper, and rendering responsibilities.

Desired outcome:

- keep one `Model`
- keep one top-level `Update`
- keep one top-level `View`
- continue avoiding direct IO in the TUI layer
- isolate style, formatting, and stats-state helper logic so `model.go` stays the shared model home rather than a catch-all

Candidate file direction:

- `model.go`: shared model/state definitions and the most central view glue
- `model_modes.go`: input routing by mode
- `model_mode_actions.go`: mode-specific action helpers
- `model_stats_state.go`: stats offset synchronization and related state helpers
- `model_styles.go`: shared styles and badge/header helpers
- `model_helpers.go`: general formatting and string helpers such as session summary formatting

The key is not to introduce new abstractions for their own sake. The key is to make future changes land in obvious places.

### Priority 3: `internal/stats/window_reports.go`

This is part of the stats roadmap, but it should follow the first `stats.go` cleanup rather than lead it.

Reason:

- `stats.go` is the better first target for clarifying package-level ownership
- after that, `window_reports.go` can be split with the same rules and naming style

Desired outcome:

- keep `LoadWindowReport` and related exported API stable
- separate window query orchestration from reusable projection or shaping helpers
- keep window-specific calculations distinct from SQL-facing logic where practical

### Priority 4: Large test helper cleanup

Test cleanup supports the production refactor rather than defining it.

Desired outcome:

- keep scenario-oriented tests readable
- move bulky repeated setup and fake helpers into helper files where repetition is materially high
- avoid extracting helpers so aggressively that tests become opaque

This should happen after the production boundaries settle.

## Recommended Sequence

### Phase 1: `internal/stats/stats.go`

Rules:

- no behavior change
- keep exported entrypoints stable
- extract DB-facing merge/query helpers before redesigning calculations
- preserve current error wrapping and pricing fallback behavior

Expected result:

- `stats.go` becomes the package front door instead of the package catch-all
- message and part merge logic move behind clearer file boundaries
- domain-facing report and summary calculations become easier to reason about independently

### Phase 2: `internal/tui/model.go`

Rules:

- preserve key bindings and visual behavior
- keep `Model` as the shared state holder
- keep async work flowing through Bubble Tea messages
- move helpers intact before rewriting internals

Expected result:

- new mode-specific behavior has an obvious file target
- styles, summaries, and stats-state sync do not regrow inside `model.go`

### Phase 3: `internal/stats/window_reports.go`

Rules:

- preserve window report behavior and exported entrypoints
- use the same thin domain/infrastructure split established in phase 1
- avoid creating files that are smaller but still mixed in responsibility

Expected result:

- window report work lands in predictable query, projection, and summary locations
- future additions to monthly or daily windows avoid regrowing a single large file

### Phase 4: test helper cleanup

Rules:

- do not convert readable scenario tests into opaque helper stacks
- only extract helpers where duplication or setup noise is materially high

Expected result:

- production refactors require less test-file surgery
- tests remain readable as behavioral documentation

## Data And Dependency Boundaries

### Stats boundary

`internal/stats` should continue owning:

- SQLite-backed usage aggregation
- overview and windowed report loading
- cost estimation integration
- report structs returned to `app` and `tui`

It should not:

- leak UI-specific decisions
- duplicate shared DB/path/time helpers that already belong in `internal/opencodedb`
- mix domain-facing summary calculations directly with SQL scanning code unless the logic is trivial

### TUI boundary

`internal/tui` should continue owning:

- input handling
- cursor and mode state
- rendering
- async message reception

It should not absorb more config, persistence, or direct runtime IO responsibilities.

### App boundary

`internal/app` should remain the orchestration spine.

It is not the highest refactoring priority right now because recent work already moved it closer to its intended shape. App follow-up refactors are valid later, but not the first implementation wave.

## Error Handling

- Preserve current error ownership at package boundaries.
- `internal/stats` may refactor internal helpers, but exported loaders must continue returning errors with equivalent meaning and caller-facing context.
- `internal/app` remains responsible for interpreting runtime failures from config, editor, launcher, and runner integration.
- `internal/tui` remains message-driven and must not absorb direct IO or persistence behavior during refactoring.
- Do not hide pricing resolution failures or silently change degraded behavior during refactoring.

## Testing Strategy

Verification should be phase-based.

For `internal/stats` work:

- run `go test ./internal/stats/... -count=1`
- add focused tests only when extracted pure helpers have meaningful standalone behavior

For `internal/tui` work:

- run `go test ./internal/tui/... -count=1`
- preserve key navigation, mode switching, and stats loading behavior

For cross-package safety after each phase:

- run `go test ./... -count=1`

The plan should favor many small structural moves over one large rewrite.

## Risks And Mitigations

### Risk: line-moving refactors accidentally change behavior

Mitigation:

- move cohesive helper groups intact first
- verify package tests after each extraction
- defer semantic cleanup until after boundaries are stable

### Risk: over-fragmenting files without improving ownership

Mitigation:

- split only where the extracted code has one clear reason to change
- keep stable shared types in predictable locations
- avoid creating new packages just to reduce file length

### Risk: over-applying domain modeling to a mostly orchestration-heavy codebase

Mitigation:

- keep domain separation thin and local
- focus domain-style extraction on report concepts and pure calculations in `internal/stats`
- avoid introducing heavyweight interfaces or cross-package layers unless later work proves they are necessary

### Risk: optimizing the wrong hotspot first

Mitigation:

- prioritize by responsibility density, not only line count
- start with `internal/stats`, where complexity and growth pressure are both high
- treat `internal/app` as a follow-up area, not the immediate target

## Phase Completion Criteria

- Stats phase is complete when `go test ./internal/stats/... -count=1` passes, `stats.go` is at least about 100 lines smaller than its current baseline, and at least one DB-facing helper group currently mixed into `stats.go` has moved into a sibling file without changing exported package entrypoints.
- TUI phase is complete when `go test ./internal/tui/... -count=1` passes, `model.go` remains the package entrypoint, and at least one of these helper groups has been moved out cleanly: stats-state sync, shared styles, or formatting helpers.
- Window-report phase is complete when `go test ./internal/stats/... -count=1` passes, `window_reports.go` is at least about 100 lines smaller than its current baseline, and at least one cohesive query/projection/helper group has moved into a sibling file without changing exported loaders.
- Test-cleanup phase is complete when `go test ./internal/app/... ./internal/stats/... ./internal/tui/... -count=1` passes and at least one repeated fake/setup helper group has been extracted without collapsing readable behavior scenarios into opaque helper-only tests.
