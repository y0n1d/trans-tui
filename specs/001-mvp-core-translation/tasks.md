# Tasks: MVP Core Translation

**Input**: Design documents from `/specs/001-mvp-core-translation/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Focused automated tests added for core translation service, IPC framing, OpenAI provider (mocked), and cancellation/error behavior.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different modules, contracts stable, no blocking dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [ ] T001 Create project directory structure per plan.md (cmd/my-trans/, internal/, ui/tui/, configs/)
- [ ] T002 Initialize Go module with `go mod init` and add dependencies (bubbletea, lipgloss, bubbles)
- [ ] T003 [P] Create example config file in configs/example.toml

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T004 [P] Implement TranslationRecord and AppState types in internal/core/state.go per contracts/state.go (fields: ID, Source, Translation, SourceLang, TargetLang, Provider, Model, CreatedAt, Error; validation rules from data-model.md)
- [ ] T005 [P] Implement TranslationRequest and TranslationResult types in internal/translator/provider.go per contracts/translator.go
- [ ] T006 [P] Define Translator interface in internal/translator/provider.go (single Translate method accepting context.Context and TranslationRequest, returning TranslationResult and error)
- [ ] T007 [P] Implement IPC message types (Request, Response) in internal/ipc/schema.go per contracts/ipc.go (versioned JSON, fields match schema)
- [ ] T008 [P] Implement config loader in internal/config/config.go (TOML parsing, api_key_env field, source_lang/target_lang defaults)
- [ ] T009 Implement IPC server (Unix socket listener) in internal/ipc/server.go (length-prefixed JSON framing, accepts connections, reads request, writes response)
- [ ] T010 Implement IPC client in internal/ipc/client.go (connects to socket, sends request, reads response, closes connection)
- [ ] T011 Implement translation service in internal/core/service.go (orchestrates: receive request → call Translator → return result; handles context cancellation)
- [ ] T012 Implement event types in internal/core/events.go (TranslationStarted, TranslationCompleted, TranslationFailed for UI communication)

**Checkpoint**: Foundation ready — contracts stable, modules can be developed in parallel

---

## Phase 2B: Foundation Tests

**Purpose**: Validate foundational contracts and services before user story work

- [ ] T013 [P] Write unit tests for IPC message framing (length-prefix encode/decode round-trip) in internal/ipc/schema_test.go
- [ ] T014 [P] Write unit tests for IPC client/server (connect, send request, receive response) in internal/ipc/server_test.go using a temp socket
- [ ] T015 [P] Write unit tests for translation service in internal/core/service_test.go (mock Translator, verify orchestration, verify context cancellation cancels translation)
- [ ] T016 [P] Write unit tests for config loader in internal/config/config_test.go (valid config, missing api_key_env, default languages)

---

## Phase 3: User Story 1 - Direct Text Translation (Priority: P1) 🎯 MVP

**Goal**: User runs `my-trans "Hello"`, TUI opens in foot, translation is displayed

**Independent Test**: Run `my-trans "Hello"` and verify foot opens with translated result

**Prerequisites**: Phase 2 and 2B complete. Contracts are stable.

### Implementation for User Story 1

- [ ] T017 [P] [US1] Implement OpenAI-compatible provider in internal/translator/openai_compatible.go (HTTP client, reads api_key_env via config, respects context cancellation, returns TranslationResult with provider and model fields)
- [ ] T018 [US1] Implement TUI model in ui/tui/model.go (wraps AppState, implements bubbletea.Model with Init/Update/View)
- [ ] T019 [US1] Implement TUI view rendering in ui/tui/view.go (displays translation records: original text + translation, status bar with record count)
- [ ] T020 [P] [US1] Implement Lip Gloss styles in ui/tui/styles.go (record layout, status bar, header styling)
- [ ] T021 [US1] Implement TUI update handlers in ui/tui/update.go (handle translation result messages, set Loading state, append record to history)
- [ ] T022 [US1] Implement CLI entry point in cmd/my-trans/main.go (parse text argument, call lifecycle to start or send request)
- [ ] T023 [US1] Implement lifecycle management in internal/runtime/lifecycle.go (detect socket, start server goroutine, launch foot with --app-id=my-trans, send initial request via IPC client)

**Checkpoint**: At this point, User Story 1 should be fully functional — `my-trans "Hello"` opens TUI with translation

### Tests for User Story 1

- [ ] T024 [P] [US1] Write mocked OpenAI provider test in internal/translator/openai_compatible_test.go (mock HTTP server returns known translation, verify TranslationResult fields, verify context cancellation aborts request, verify missing API key returns error)
- [ ] T025 [US1] Write integration test for end-to-end translation flow in internal/core/service_integration_test.go (mock Translator, simulate IPC round-trip, verify record appended to AppState)

---

## Phase 4: User Story 2 - Single Instance Reuse (Priority: P1)

**Goal**: Second `my-trans "World"` reuses existing TUI, appends to history

**Independent Test**: Run `my-trans "Hello"` then `my-trans "World"` — one window, two records

**Prerequisites**: US1 functional (server + client + TUI all working)

### Implementation for User Story 2

- [ ] T026 [US2] Implement socket detection in internal/runtime/lifecycle.go (try connect to existing socket; if fails, remove stale file and start new server; if succeeds, send request via client)
- [ ] T027 [US2] Implement stale socket cleanup in internal/runtime/lifecycle.go (detect stale socket via failed connection, remove file, proceed with fresh server)
- [ ] T028 [US2] Implement server-side request handling in internal/ipc/server.go (receive translate request → call service → append record → send response)
- [ ] T029 [US2] Verify single instance behavior: second CLI invocation sends to existing server, no second foot window created

**Checkpoint**: At this point, User Stories 1 AND 2 should both work — single instance with append-only history

### Tests for User Story 2

- [ ] T030 [P] [US2] Write unit tests for socket detection and stale cleanup in internal/runtime/lifecycle_test.go (test: no socket → start fresh; live socket → send to it; stale socket → remove and start fresh)

---

## Phase 5: User Story 3 - History Browsing (Priority: P2)

**Goal**: User can scroll through translation history with keyboard and mouse

**Independent Test**: Submit 5+ translations, scroll up/down, verify all entries accessible

**Prerequisites**: US1 functional (TUI + records exist)

### Implementation for User Story 3

- [ ] T031 [US3] Implement key bindings in ui/tui/keys.go (up, down, page up, page down, home, end, quit)
- [ ] T032 [US3] Integrate Bubbles viewport component in ui/tui/model.go (replace manual scroll tracking with viewport for smooth scrolling)
- [ ] T033 [US3] Implement mouse wheel handling in ui/tui/update.go (handle bubbletea.MouseMsg for wheel up/down events)
- [ ] T034 [US3] Implement auto-scroll behavior in ui/tui/update.go (on new translation: if user was at bottom, scroll to bottom; otherwise stay at current position)
- [ ] T035 [US3] Update view rendering in ui/tui/view.go (use viewport for scrollable content area, render records within viewport bounds)

**Checkpoint**: At this point, User Stories 1, 2, AND 3 should all work — scrolling through history

---

## Phase 6: User Story 4 - Error Handling and Retry (Priority: P2)

**Goal**: Errors display clearly, user can retry with `r` key

**Independent Test**: Simulate provider failure, verify error message, retry succeeds

**Prerequisites**: US1 functional (translation flow exists to test errors against)

### Implementation for User Story 4

- [ ] T036 [US4] Implement error display in ui/tui/view.go (render error block with styled message when AppState.Error is non-empty)
- [ ] T037 [US4] Implement retry key binding in ui/tui/keys.go (map `r` key to retry action)
- [ ] T038 [US4] Implement retry logic in ui/tui/update.go (on retry: clear Error, set Loading=true, re-submit last failed request via service)
- [ ] T039 [US4] Implement request cancellation in internal/core/service.go (cancel previous context when new request arrives; FR-014)
- [ ] T040 [US4] Implement loading state display in ui/tui/view.go (show "Translating..." or spinner while Loading=true)
- [ ] T041 [US4] Implement user-friendly error messages for: missing API key, network failure, provider error, empty result (internal/runtime/lifecycle.go and ui/tui/update.go)

**Checkpoint**: All user stories should now be independently functional

### Tests for User Story 4

- [ ] T042 [P] [US4] Write unit tests for cancellation behavior in internal/core/service_test.go (submit request A, submit request B before A completes, verify A's context was cancelled)
- [ ] T043 [P] [US4] Write unit tests for error propagation in internal/core/service_test.go (mock Translator returning error, verify AppState.Error set and LastFailed points to the record)

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T044 [P] Implement graceful shutdown in internal/runtime/lifecycle.go (detect foot close via signal/EOF, cancel pending requests, remove socket file, exit cleanly)
- [ ] T045 [P] Run `go vet ./...` and fix any issues
- [ ] T046 [P] Run quickstart.md validation scenarios from specs/001-mvp-core-translation/quickstart.md
- [ ] T047 Verify Constitution compliance: Core has no TUI imports, no daemon created, API keys only from env vars, socket cleaned up on exit

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS all user stories
- **Foundation Tests (Phase 2B)**: Depends on Phase 2 — validates contracts before story work
- **User Story 1 (Phase 3)**: Depends on Phase 2B — establishes the core loop
- **User Story 2 (Phase 4)**: Depends on US1 (needs running server + client to test socket detection)
- **User Story 3 (Phase 5)**: Depends on US1 TUI base (T018-T021) — TUI Agent continues directly from US1
- **User Story 4 (Phase 6)**: Depends on US1 TUI base (T018-T021) for UI tasks; T039 cancellation depends on Core service (T011)
- **Polish (Phase 7)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Phase 2B — No dependencies on other stories
- **User Story 2 (P1)**: Can start after US1 is functional (needs running server to test socket detection)
- **User Story 3 (P2)**: Can start after US1 TUI base is done (T018-T021) — does NOT depend on US2
- **User Story 4 (P2)**: Can start after US1 TUI base is done (T018-T021) — does NOT depend on US2 or US3; T039 cancellation depends on Core service (T011)

### Within Each User Story

- Models/types before services
- Services before TUI integration
- Core implementation before lifecycle wiring
- Story complete before moving to next priority

### Dependency Graph

```
Phase 2B (contracts validated)
    │
    ├──→ US1 TUI base (T018-T021)
    │         │
    │         ├──────────────┐
    │         ↓              ↓
    │    US3 (T031-T035)  US4 UI (T036-T038, T040-T041)
    │    TUI Agent         TUI Agent
    │         │              │
    │         └──────┬───────┘
    │                ↓
    │         Integration (T022-T023)
    │         主 Agent (single)
    │                ↓
    │         US2 (T026-T029)
    │         主 Agent
    │                ↓
    │         US4 Core (T039, T042-T043)
    │         Core Agent
    │
    └──→ OpenAI Provider (T017) + Tests (T024)
          Provider Agent
```

---

## Module-Oriented Parallel Strategy

Contracts defined in Phase 2 are stable shared boundaries. After Phase 2B validates the contracts, modules can be developed in parallel by different agents.

### Layer 1: Foundation (Sequential — must complete first)

```
Phase 1: Setup
Phase 2: Foundational types, interfaces, IPC schema
Phase 2B: Foundation tests (validate contracts)
```

### Layer 2: Modules (Parallel — contracts stable)

Once Phase 2B passes, these modules can be developed simultaneously:

```
Provider Agent:
  T017 OpenAI-compatible provider
  T024 Mocked provider tests

TUI Agent:
  T018-T021 TUI model, view, update, styles (US1 base)
  T031-T035 Key bindings, viewport, scrolling (US3 — continues from US1 base)
  T036-T038, T040-T041 Error display, retry key, loading display (US4 UI — continues from US1 base)

Core Agent:
  T011 Translation service
  T012 Event types
  T039 Request cancellation (US4 Core)
  T042-T043 Cancellation and error tests (US4)
```

### Layer 3: Integration (Sequential — single 主 Agent)

**⚠️ CRITICAL**: Lifecycle and CLI integration MUST be done by a single 主 Agent.
Multiple agents touching lifecycle.go causes merge conflicts and architecture violations.

```
主 Agent (single, exclusive):
  T022 Server-side request handling (IPC server wiring)
  T023 CLI entry point + lifecycle management (socket detection, foot launch, shutdown)
  T026-T027 Socket detection + stale cleanup (US2)
  T029 Verify single instance behavior (US2)
  T030 Socket detection tests (US2)
  T044 Graceful shutdown (Polish)
```

### Layer 4: Validation (after all layers complete)

```
T045 Run go vet
T046 Run quickstart.md validation
T047 Constitution compliance check
```

### Rules

- **TUI Agent owns ui/tui/ exclusively.** After US1 base (T018-T021), the TUI Agent continues to US3 (T031-T035) and US4 UI (T036-T038, T040-T041) without waiting for US2.
- **Provider Agent owns internal/translator/ exclusively.** T017 + T024 are both Provider Agent tasks.
- **Core Agent owns internal/core/ and US4 cancellation.** T011, T012, T039, T042-T043.
- **主 Agent owns lifecycle.go, server wiring, and CLI entry point.** No other agent modifies lifecycle.go. The 主 Agent integrates all modules after they are ready.
- **US2 is implemented by the 主 Agent** because it modifies lifecycle.go (socket detection, stale cleanup).
- **US3 and US4 TUI tasks do NOT depend on US2.** The TUI Agent can proceed immediately after US1 base is functional.
- **US4 has two owners**: TUI Agent handles UI tasks (T036-T038, T040-T041); Core Agent handles cancellation (T039) and tests (T042-T043).
- **Merge conflicts are minimized** because each agent works in a distinct package: `internal/translator/`, `internal/ipc/`, `ui/tui/`, `internal/core/`, or `internal/runtime/` (主 Agent only).

### Parallel Example: After Phase 2B

```bash
# Launch simultaneously (no dependencies between these agents):
Provider Agent: T017 → T024
TUI Agent: T018 → T019 → T020 → T021 → T031 → T032 → T033 → T034 → T035 → T036 → T037 → T038 → T040 → T041
Core Agent: T011 → T012 → T039 → T042 → T043

# After modules are ready:
主 Agent: T022 → T023 → T026 → T027 → T028 → T029 → T030 → T044

# Final validation:
T045 → T046 → T047
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Complete Phase 2B: Foundation tests (validate contracts)
4. Complete Phase 3: User Story 1
5. **STOP and VALIDATE**: Run `my-trans "Hello"` — foot opens with translation
6. Commit and verify

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. Foundation tests pass → Contracts validated
3. Add US1 → `my-trans "Hello"` works → **MVP!**
4. Add US2 (主 Agent) + US3 (TUI Agent) + US4 UI (TUI Agent) in parallel
5. Add US4 Core (Core Agent) — cancellation + tests
6. Polish → Constitution compliance verified

---

## Notes

- [P] tasks = different modules, contracts stable, no blocking dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable after US1 is functional
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Test tasks validate contracts and behavior, not UI rendering
