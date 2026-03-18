# oc CLI — OpenCode Plugin Selector

## TL;DR

> **Quick Summary**: Go CLI 앱 "oc"를 구축한다. opencode.json의 `"plugin"` 배열에서 플러그인 목록을 읽어 bubbletea TUI로 선택/해제하고, 선택하지 않은 플러그인은 JSONC 주석(`//`)으로 비활성화한 뒤 opencode를 실행하는 래퍼 도구.
> 
> **Deliverables**:
> - `oc` 바이너리 (Mac arm64/amd64, Windows amd64)
> - bubbletea 기반 multi-select TUI
> - JSONC 주석 토글 엔진 (라인 기반)
> - `~/.oc` TOML 설정 파일 지원 (plugin whitelist)
> - Makefile (cross-compile + 릴리즈)
> - TDD 기반 테스트 스위트
> 
> **Estimated Effort**: Medium
> **Parallel Execution**: YES — 3 waves
> **Critical Path**: Task 1 → Task 2 → Task 5 → Task 7 → Task 8 → Task 9

---

## Context

### Original Request
Go 언어로 작성한 CLI 앱. opencode의 플러그인을 배타적으로 선택해서 활성화(주석 해제)하고 opencode를 실행하는 것이 주목적. bubbletea TUI로 복수 선택, 화살표 키 이동. ~/.oc에서 plugin whitelist 지정 가능.

### Interview Summary
**Key Discussions**:
- 토글 방식: `enabled` 속성이 아닌 **JSONC 주석 처리** 방식 선택
- TUI 트리거: 플래그 없이 기본 동작 (모든 인자는 opencode에 전달)
- opencode.json 위치: `~/.config/opencode/opencode.json` (없으면 에러+종료)
- 설정 파일: `~/.oc` TOML 형식
- 릴리즈: 수동 (gh CLI)
- 테스트: TDD

**Research Findings**:
- opencode는 JSONC 포맷 사용 (주석, trailing comma 지원)
- `"plugin"` 키는 단순 문자열 배열 — 블록 파싱 불필요
- Go에 JSONC 주석 조작 라이브러리 없음 → 라인 기반 직접 구현
- bubbletea v2: `charm.land/bubbletea/v2` 경로, `tea.KeyPressMsg`, `tea.NewView()` API
- BurntSushi/toml v1.6.0 — 표준 Go TOML 라이브러리

### Metis Review
**Identified Gaps** (addressed):
- Ctrl+C 취소 시 동작 → abort (파일 미수정, opencode 미실행)
- `~/.oc` TOML 구조 → `plugins = ["a", "b"]` 배열 형태
- 빈 plugin 배열 → TUI 스킵, opencode 바로 실행
- Windows 줄바꿈 → `\r\n` 감지 및 보존
- Trailing comma 처리 → JSONC이므로 항상 허용
- opencode PATH 확인 → `exec.LookPath` 사전 검증

---

## Work Objectives

### Core Objective
opencode.json의 plugin 배열 항목을 JSONC 주석으로 토글하는 bubbletea TUI CLI 래퍼를 Go로 구축한다.

### Concrete Deliverables
- `cmd/oc/main.go` — 진입점
- `internal/config/` — opencode.json 파서 + TOML 설정 리더
- `internal/plugin/` — 플러그인 모델 + 화이트리스트 필터링
- `internal/tui/` — bubbletea multi-select 모델
- `internal/runner/` — opencode 서브프로세스 실행
- `Makefile` — 빌드/릴리즈 타겟
- `.gitignore` — Go 프로젝트용

### Definition of Done
- [ ] `oc` 실행 시 TUI에서 plugin 목록이 표시됨
- [ ] 선택/해제 후 opencode.json이 올바르게 수정됨 (주석 토글)
- [ ] opencode가 모든 인자와 함께 실행됨
- [ ] `~/.oc` whitelist 지정 시 해당 플러그인만 TUI에 표시
- [ ] Mac/Windows 바이너리가 정상 빌드됨
- [ ] 모든 테스트 통과

### Must Have
- JSONC 주석 기반 플러그인 토글 (enabled 속성 아님)
- bubbletea v2 multi-select TUI
- 모든 CLI 인자 opencode에 그대로 전달
- `~/.oc` TOML whitelist 지원
- Ctrl+C 시 파일 미수정 + opencode 미실행
- 수정 후 JSONC 유효성 검증

### Must NOT Have (Guardrails)
- `enabled: true/false` 속성 방식 사용 금지
- 플러그인 버전 해석 (`@latest` 등) 금지 — 문자열 통째로 취급
- opencode.json 또는 ~/.oc 자동 생성 금지
- 플러그인 검색/다운로드/의존성 해석 금지
- Alt-screen TUI 금지 — 인라인 렌더링만
- bubbles v2 list 컴포넌트 사용 금지 — 직접 구현
- `syscall.Exec` 사용 금지 — `os/exec` `cmd.Run()` 사용
- JSON 스키마 검증 금지
- 로깅 프레임워크 금지 — stderr 단순 출력
- 자동 업데이트 기능 금지

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: NO (greenfield — 설정 필요)
- **Automated tests**: TDD (RED → GREEN → REFACTOR)
- **Framework**: Go 내장 `testing` 패키지
- **If TDD**: 각 태스크는 테스트 먼저 작성 → 구현 → 리팩터링

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **CLI**: Use Bash — Run command, validate output, check exit code
- **TUI**: Use interactive_bash (tmux) — Run oc, send keystrokes, validate output
- **Library/Module**: Use Bash (`go test`) — Run tests, compare output

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — foundation):
├── Task 1: Project scaffolding (go mod, dirs, .gitignore, Makefile) [quick]
├── Task 2: JSONC plugin parser + tests [deep]
├── Task 3: TOML config reader + tests [quick]

Wave 2 (After Wave 1 — core modules):
├── Task 4: Plugin model + whitelist filtering + tests (depends: 2, 3) [quick]
├── Task 5: JSONC comment toggler + file writer + tests (depends: 2) [deep]
├── Task 6: Bubbletea TUI model + tests (depends: none, but 4 provides types) [unspecified-high]

Wave 3 (After Wave 2 — integration):
├── Task 7: OpenCode subprocess runner + tests (depends: none for code, but 6 for flow) [quick]
├── Task 8: Main.go wiring + integration tests (depends: 4, 5, 6, 7) [deep]
├── Task 9: Cross-platform build + README (depends: 8) [quick]

Wave FINAL (After ALL tasks):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Real manual QA (unspecified-high)
└── Task F4: Scope fidelity check (deep)
-> Present results -> Get explicit user okay

Critical Path: Task 1 → Task 2 → Task 5 → Task 8 → Task 9 → F1-F4 → user okay
Parallel Speedup: ~50% faster than sequential
Max Concurrent: 3 (Wave 1 & 2)
```

### Dependency Matrix

| Task | Depends On | Blocks |
|------|-----------|--------|
| 1 | — | 2, 3, 6, 7 |
| 2 | 1 | 4, 5 |
| 3 | 1 | 4 |
| 4 | 2, 3 | 8 |
| 5 | 2 | 8 |
| 6 | 1 (4 for types) | 8 |
| 7 | 1 | 8 |
| 8 | 4, 5, 6, 7 | 9 |
| 9 | 8 | F1-F4 |

### Agent Dispatch Summary

- **Wave 1**: **3** — T1 → `quick`, T2 → `deep`, T3 → `quick`
- **Wave 2**: **3** — T4 → `quick`, T5 → `deep`, T6 → `unspecified-high`
- **Wave 3**: **3** — T7 → `quick`, T8 → `deep`, T9 → `quick`
- **FINAL**: **4** — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [ ] 1. Project Scaffolding — Go Module, Directories, Makefile, .gitignore

  **What to do**:
  - `go mod init github.com/kayden-kim/oc`
  - 디렉토리 생성: `cmd/oc/`, `internal/config/`, `internal/plugin/`, `internal/tui/`, `internal/runner/`
  - `cmd/oc/main.go` — 최소한의 진입점 (빈 main 함수, 빌드 확인용)
  - `.gitignore` — Go 프로젝트용 (dist/, *.exe, vendor/ 등)
  - `Makefile` — 기본 타겟: `build`, `test`, `build-all` (cross-compile), `clean`
    - `build-all`: `CGO_ENABLED=0 GOOS=darwin GOARCH=arm64`, `GOOS=darwin GOARCH=amd64`, `GOOS=windows GOARCH=amd64`
    - 출력: `dist/oc-darwin-arm64`, `dist/oc-darwin-amd64`, `dist/oc-windows-amd64.exe`
  - bubbletea v2 등 핵심 의존성 추가: `go get charm.land/bubbletea/v2`, `go get charm.land/lipgloss/v2`, `go get github.com/tidwall/jsonc`, `go get github.com/BurntSushi/toml`
  - `go build ./cmd/oc/` 로 빌드 성공 확인

  **Must NOT do**:
  - 비즈니스 로직 구현 금지 (구조만 잡기)
  - README 작성 금지 (Task 9에서)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 파일 생성과 명령어 실행만 필요한 단순 스캐폴딩
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `git-master`: 커밋은 태스크 완료 시 자동 처리

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3)
  - **Blocks**: Tasks 2, 3, 6, 7
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - 없음 (greenfield)

  **External References**:
  - Bubbletea v2 import path: `charm.land/bubbletea/v2` (NOT `github.com/charmbracelet/bubbletea/v2`)
  - Lipgloss v2: `charm.land/lipgloss/v2`
  - tidwall/jsonc: `github.com/tidwall/jsonc`
  - BurntSushi/toml: `github.com/BurntSushi/toml`

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Go module builds successfully
    Tool: Bash
    Preconditions: Task complete
    Steps:
      1. Run `go build ./cmd/oc/`
      2. Check exit code is 0
      3. Verify binary exists
    Expected Result: Build succeeds with exit code 0
    Failure Indicators: Compilation errors, missing imports
    Evidence: .sisyphus/evidence/task-1-go-build.txt

  Scenario: Directory structure is correct
    Tool: Bash
    Preconditions: Task complete
    Steps:
      1. Run `ls -la cmd/oc/ internal/config/ internal/plugin/ internal/tui/ internal/runner/`
      2. Verify each directory contains at least a placeholder or main.go
    Expected Result: All directories exist
    Failure Indicators: Directory not found errors
    Evidence: .sisyphus/evidence/task-1-dirs.txt

  Scenario: Makefile cross-compile works
    Tool: Bash
    Preconditions: Task complete
    Steps:
      1. Run `make build-all`
      2. Check `dist/oc-darwin-arm64`, `dist/oc-darwin-amd64`, `dist/oc-windows-amd64.exe` exist
    Expected Result: 3 binaries in dist/
    Failure Indicators: Missing binaries, build errors
    Evidence: .sisyphus/evidence/task-1-cross-compile.txt
  ```

  **Commit**: YES
  - Message: `init: scaffold Go module and project structure`
  - Files: `go.mod, go.sum, cmd/oc/main.go, .gitignore, Makefile, internal/*/`
  - Pre-commit: `go build ./cmd/oc/`

- [ ] 2. JSONC Plugin Parser — 라인 기반 파싱 엔진 (TDD)

  **What to do**:
  - **테스트 먼저**: `internal/config/jsonc_parser_test.go`
    - 정상 케이스: 활성 플러그인 + 주석 플러그인 혼합 배열 파싱
    - 빈 배열 `"plugin": []`
    - 전부 활성, 전부 주석
    - 특수문자 포함 플러그인명 (`opencode-antigravity-auth@latest`, `@scope/plugin`)
    - 주석 형태 변형 (`//"plugin"`, `// "plugin"`, `//  "plugin"`)
    - trailing comma 유무
    - Windows 줄바꿈 (`\r\n`)
    - plugin 키가 없는 JSON → 에러 반환
    - plugin 키가 배열이 아닌 경우 → 에러 반환
  - **구현**: `internal/config/jsonc_parser.go`
    - `ParsePlugins(content []byte) ([]Plugin, error)` 함수
    - `Plugin` 구조체: `Name string`, `Enabled bool`, `LineIndex int`, `OriginalLine string`
    - 라인별로 읽으며 `"plugin"` 배열 영역 감지 (`[` ~ `]`)
    - 각 라인에서 주석 여부 판별 + 플러그인명 추출
    - 줄바꿈 문자 감지 및 보존 (`\r\n` vs `\n`)
  - `Plugin` 구조체는 이 파일이나 별도 `internal/config/types.go`에 정의

  **Must NOT do**:
  - 파일 I/O 직접 수행 금지 ([]byte 입력만 받음)
  - 주석 토글 로직 금지 (Task 5에서)
  - JSON 스키마 검증 금지

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: JSONC 라인 기반 파서는 엣지 케이스가 많은 핵심 로직. 정교한 TDD 필요
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3) — 단, Task 1 완료 후 시작
  - **Blocks**: Tasks 4, 5
  - **Blocked By**: Task 1

  **References**:

  **Pattern References**:
  - 없음 (greenfield)

  **API/Type References**:
  - opencode.json 실제 구조 (사용자 제공):
    ```jsonc
    {
      "$schema": "https://opencode.ai/config.json",
      "plugin": [
        "oh-my-opencode",
        // "opencode-antigravity-auth@latest"
      ]
    }
    ```

  **External References**:
  - `bufio.Scanner` — 라인별 읽기
  - `strings.TrimSpace`, `strings.TrimPrefix` — 주석/공백 처리
  - `regexp` — 플러그인명 추출 패턴: `^\s*"([^"]+)"` (활성), `^\s*//\s*"([^"]+)"` (주석)

  **Acceptance Criteria**:

  - [ ] `go test ./internal/config/ -run TestParsePlugins` → PASS (8+ test cases)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Parse mixed active and commented plugins
    Tool: Bash (go test)
    Preconditions: Test file with 2 active + 1 commented plugin
    Steps:
      1. Run `go test ./internal/config/ -run TestParsePlugins -v`
      2. Verify 3 plugins parsed: 2 enabled, 1 disabled
    Expected Result: All tests pass, correct plugin names and states
    Failure Indicators: Wrong count, wrong enabled state, name mismatch
    Evidence: .sisyphus/evidence/task-2-parse-plugins.txt

  Scenario: Handle edge cases (empty, special chars, whitespace variants)
    Tool: Bash (go test)
    Preconditions: Test cases for empty array, @latest suffix, various comment spacing
    Steps:
      1. Run `go test ./internal/config/ -run TestParsePlugins -v`
      2. Check edge case subtests pass
    Expected Result: All edge case tests pass
    Failure Indicators: Panic on empty array, wrong name for @latest plugins
    Evidence: .sisyphus/evidence/task-2-edge-cases.txt
  ```

  **Commit**: YES
  - Message: `feat(config): add JSONC plugin parser with TDD`
  - Files: `internal/config/jsonc_parser.go, internal/config/jsonc_parser_test.go, internal/config/types.go`
  - Pre-commit: `go test ./internal/config/...`

- [ ] 3. TOML Config Reader — ~/.oc 화이트리스트 (TDD)

  **What to do**:
  - **테스트 먼저**: `internal/config/toml_config_test.go`
    - 정상 케이스: `plugins = ["a", "b"]` 파싱
    - 파일 없음 → nil whitelist (에러 아님, whitelist 미적용)
    - 빈 plugins 배열 → 빈 슬라이스
    - 잘못된 TOML 형식 → 에러 반환
  - **구현**: `internal/config/toml_config.go`
    - `type OcConfig struct { Plugins []string \`toml:"plugins"\` }`
    - `LoadOcConfig(path string) (*OcConfig, error)` — 파일 없으면 nil, nil 반환
    - `os.UserHomeDir()` + `filepath.Join()` 으로 경로 구성
  - `~/.oc` 예시:
    ```toml
    plugins = ["superpowers", "oh-my-opencode"]
    ```

  **Must NOT do**:
  - ~/.oc 파일 자동 생성 금지
  - 파일 없을 때 에러 반환 금지 (nil 반환)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: BurntSushi/toml 사용한 단순 파일 읽기
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2) — 단, Task 1 완료 후 시작
  - **Blocks**: Task 4
  - **Blocked By**: Task 1

  **References**:

  **External References**:
  - BurntSushi/toml: `toml.DecodeFile(path, &config)` — https://github.com/BurntSushi/toml
  - `os.UserHomeDir()` — 크로스 플랫폼 홈 디렉토리
  - `os.IsNotExist(err)` — 파일 부재 감지

  **Acceptance Criteria**:

  - [ ] `go test ./internal/config/ -run TestLoadOcConfig` → PASS (4+ test cases)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Parse valid TOML whitelist
    Tool: Bash (go test)
    Preconditions: Temp file with plugins = ["a", "b"]
    Steps:
      1. Run `go test ./internal/config/ -run TestLoadOcConfig -v`
      2. Verify plugins slice contains ["a", "b"]
    Expected Result: Correct plugins parsed
    Failure Indicators: Empty slice, wrong values
    Evidence: .sisyphus/evidence/task-3-toml-config.txt

  Scenario: Missing config file returns nil (not error)
    Tool: Bash (go test)
    Preconditions: Non-existent path
    Steps:
      1. Run `go test ./internal/config/ -run TestLoadOcConfigMissing -v`
      2. Verify nil config, nil error
    Expected Result: No error, nil config
    Failure Indicators: Error returned for missing file
    Evidence: .sisyphus/evidence/task-3-missing-config.txt
  ```

  **Commit**: YES
  - Message: `feat(config): add TOML whitelist config reader`
  - Files: `internal/config/toml_config.go, internal/config/toml_config_test.go`
  - Pre-commit: `go test ./internal/config/...`

- [ ] 4. Plugin Model — 화이트리스트 필터링 (TDD)

  **What to do**:
  - **테스트 먼저**: `internal/plugin/plugin_test.go`
    - whitelist 있을 때: whitelist에 있는 플러그인만 필터링
    - whitelist 없을 때 (nil): 모든 플러그인 반환
    - whitelist에 없는 플러그인의 상태는 보존 (변경하지 않음)
    - 빈 whitelist → 빈 결과
    - 대소문자 구분 테스트
  - **구현**: `internal/plugin/plugin.go`
    - `FilterByWhitelist(plugins []config.Plugin, whitelist []string) (visible []config.Plugin, hidden []config.Plugin)`
    - `visible`: TUI에 표시할 플러그인 (whitelist에 포함된 것)
    - `hidden`: TUI에 표시하지 않을 플러그인 (whitelist에 없는 것 — 파일 수정 시 원래 상태 유지)
    - whitelist가 nil이면 모든 플러그인이 visible

  **Must NOT do**:
  - 파일 I/O 금지
  - TUI 관련 코드 금지

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 단순 슬라이스 필터링 로직
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 5, 6)
  - **Blocks**: Task 8
  - **Blocked By**: Tasks 2, 3

  **References**:

  **API/Type References**:
  - `internal/config/types.go:Plugin` — Plugin 구조체 (Name, Enabled, LineIndex, OriginalLine)
  - `internal/config/toml_config.go:OcConfig` — Plugins []string

  **Acceptance Criteria**:

  - [ ] `go test ./internal/plugin/ -run TestFilter` → PASS (5+ test cases)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Whitelist filters correctly
    Tool: Bash (go test)
    Preconditions: 4 plugins, whitelist with 2 names
    Steps:
      1. Run `go test ./internal/plugin/ -v`
      2. Verify visible contains only 2 whitelisted plugins
      3. Verify hidden contains the other 2
    Expected Result: Correct split between visible and hidden
    Failure Indicators: Wrong count, plugins in wrong group
    Evidence: .sisyphus/evidence/task-4-whitelist-filter.txt

  Scenario: No whitelist returns all plugins as visible
    Tool: Bash (go test)
    Preconditions: 4 plugins, nil whitelist
    Steps:
      1. Run `go test ./internal/plugin/ -run TestFilterNoWhitelist -v`
      2. Verify all 4 plugins in visible, 0 in hidden
    Expected Result: All plugins visible
    Failure Indicators: Plugins incorrectly hidden
    Evidence: .sisyphus/evidence/task-4-no-whitelist.txt
  ```

  **Commit**: YES
  - Message: `feat(plugin): add plugin model and whitelist filter`
  - Files: `internal/plugin/plugin.go, internal/plugin/plugin_test.go`
  - Pre-commit: `go test ./internal/plugin/...`

- [ ] 5. JSONC Comment Toggler — 주석 추가/제거 + 파일 쓰기 (TDD)

  **What to do**:
  - **테스트 먼저**: `internal/config/jsonc_writer_test.go`
    - 활성 플러그인 → 주석 처리: `    "name",` → `    // "name",`
    - 주석 플러그인 → 활성화: `    // "name",` → `    "name",`
    - 혼합 토글 (일부 주석 추가, 일부 해제)
    - 파일 다른 부분 (schema, mcp 등) 보존 확인
    - 들여쓰기 보존
    - 줄바꿈 문자 보존 (`\r\n` → `\r\n` 유지)
    - trailing comma 유지
    - 수정 후 `tidwall/jsonc` + `json.Unmarshal` 로 유효성 검증
    - hidden 플러그인 (whitelist 외)은 원래 상태 그대로 유지
  - **구현**: `internal/config/jsonc_writer.go`
    - `ApplySelections(content []byte, selections map[string]bool) ([]byte, error)`
      - `selections`: 플러그인명 → true(활성화)/false(주석처리)
      - 원본 content의 각 라인을 순회하며 plugin 배열 내 해당 항목만 수정
      - selections에 없는 플러그인은 원래 상태 유지 (hidden 처리)
    - 수정 후 `jsonc.ToJSON()` + `json.Unmarshal()` 로 유효성 검증
    - `WriteConfigFile(path string, content []byte) error` — 파일 쓰기 (atomic write 권장: temp → rename)

  **Must NOT do**:
  - `enabled` 속성 추가/수정 금지
  - JSON 재생성 금지 (원본 라인 기반 수정만)
  - plugin 배열 외부 내용 수정 금지

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: 핵심 비즈니스 로직. 라인 기반 텍스트 조작의 엣지 케이스가 많음
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 4, 6)
  - **Blocks**: Task 8
  - **Blocked By**: Task 2

  **References**:

  **API/Type References**:
  - `internal/config/jsonc_parser.go:ParsePlugins` — 파싱 결과를 기반으로 라인 수정
  - `internal/config/types.go:Plugin` — LineIndex로 수정 대상 라인 식별

  **External References**:
  - `github.com/tidwall/jsonc` — `jsonc.ToJSON(content)` 로 유효성 검증
  - `encoding/json` — `json.Unmarshal` 로 파싱 가능 여부 확인
  - `os.CreateTemp` + `os.Rename` — atomic write 패턴

  **Acceptance Criteria**:

  - [ ] `go test ./internal/config/ -run TestApplySelections` → PASS (8+ test cases)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Toggle comments correctly
    Tool: Bash (go test)
    Preconditions: JSONC with 3 plugins (2 active, 1 commented)
    Steps:
      1. Run `go test ./internal/config/ -run TestApplySelections -v`
      2. Verify: selecting only 1st plugin → 2nd commented, 3rd stays commented
      3. Verify output is valid JSONC
    Expected Result: Correct lines toggled, file valid
    Failure Indicators: Wrong lines modified, invalid JSONC, lost formatting
    Evidence: .sisyphus/evidence/task-5-toggle.txt

  Scenario: Preserve non-plugin content
    Tool: Bash (go test)
    Preconditions: JSONC with schema, mcp, and plugin sections
    Steps:
      1. Run `go test ./internal/config/ -run TestApplySelectionsPreserve -v`
      2. Verify schema and mcp sections unchanged
    Expected Result: Only plugin array lines modified
    Failure Indicators: Changes to non-plugin lines
    Evidence: .sisyphus/evidence/task-5-preserve.txt

  Scenario: Windows line endings preserved
    Tool: Bash (go test)
    Preconditions: JSONC with \r\n line endings
    Steps:
      1. Run `go test ./internal/config/ -run TestApplySelectionsLineEndings -v`
      2. Verify output still uses \r\n
    Expected Result: Line endings preserved
    Failure Indicators: \r\n converted to \n or vice versa
    Evidence: .sisyphus/evidence/task-5-line-endings.txt
  ```

  **Commit**: YES
  - Message: `feat(config): add comment toggler and file writer`
  - Files: `internal/config/jsonc_writer.go, internal/config/jsonc_writer_test.go`
  - Pre-commit: `go test ./internal/config/...`

- [ ] 6. Bubbletea TUI — Multi-Select Model (TDD)

  **What to do**:
  - **테스트 먼저**: `internal/tui/model_test.go`
    - 초기 상태: cursor=0, 활성 플러그인은 미리 선택됨
    - 화살표 키 위/아래 이동 → cursor 변경
    - Space 키 → 선택 토글
    - Enter 키 → 확인 (quit + 선택 결과 반환)
    - Ctrl+C / q → 취소 (quit + cancelled=true)
    - j/k vim 바인딩
    - 경계값: cursor가 0일 때 위, 마지막일 때 아래
    - 빈 목록 → 즉시 종료 (선택 화면 안 보임)
  - **구현**: `internal/tui/model.go`
    - `type Model struct`:
      - `plugins []PluginItem` — Name, InitiallyEnabled
      - `cursor int`
      - `selected map[int]struct{}` — 선택된 인덱스
      - `cancelled bool` — Ctrl+C 여부
      - `confirmed bool` — Enter 여부
    - `Init() tea.Cmd` → nil
    - `Update(msg tea.Msg) (tea.Model, tea.Cmd)`:
      - `tea.KeyPressMsg` 처리: up/k, down/j, space, enter, ctrl+c/q/esc
    - `View() tea.View`:
      ```
      Select plugins (Space: toggle, Enter: confirm, q: quit):

      > [*] oh-my-opencode
        [ ] opencode-antigravity-auth@latest

      ↑/↓: navigate • space: toggle • enter: confirm • q: quit
      ```
    - `type PluginItem struct { Name string; InitiallyEnabled bool }`
    - `func NewModel(items []PluginItem) Model` — 초기화 (활성 플러그인 미리 선택)
    - `func (m Model) Selections() map[string]bool` — 결과 반환 (name → selected)
    - `func (m Model) Cancelled() bool`
  - lipgloss v2로 선택된 항목, 커서 스타일링

  **Must NOT do**:
  - Alt-screen 모드 사용 금지 (인라인만)
  - bubbles v2 list 컴포넌트 사용 금지
  - 파일 I/O 금지
  - opencode 실행 금지

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: bubbletea v2 API를 정확히 사용해야 하며 (KeyPressMsg, NewView), TUI 모델 상태 머신 설계 필요
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 4, 5)
  - **Blocks**: Task 8
  - **Blocked By**: Task 1 (go.mod 필요), Task 4 (PluginItem 타입 참조 가능하나 자체 정의도 가능)

  **References**:

  **External References**:
  - Bubbletea v2 tutorial: https://github.com/charmbracelet/bubbletea/tree/main/tutorials/basics
  - Import: `charm.land/bubbletea/v2`
  - **v2 API 변경사항**:
    - `View()` returns `tea.View` — use `tea.NewView(s)`
    - `tea.KeyMsg` → `tea.KeyPressMsg`
    - Space key: `"space"` (not `" "`)
    - `msg.Type` → `msg.Code`
  - Lipgloss v2: `charm.land/lipgloss/v2`

  **WHY Each Reference Matters**:
  - bubbletea v2는 v1과 API가 상당히 다름. 반드시 v2 API 확인 필요

  **Acceptance Criteria**:

  - [ ] `go test ./internal/tui/ -run TestModel` → PASS (8+ test cases)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: TUI model state transitions
    Tool: Bash (go test)
    Preconditions: Model with 3 plugins (2 enabled, 1 disabled)
    Steps:
      1. Run `go test ./internal/tui/ -v`
      2. Verify: initial state has cursor=0, 2 pre-selected
      3. Verify: down key moves cursor to 1
      4. Verify: space toggles selection at cursor
      5. Verify: enter sets confirmed=true and returns tea.Quit
      6. Verify: ctrl+c sets cancelled=true
    Expected Result: All state transitions correct
    Failure Indicators: Wrong cursor position, wrong selection state
    Evidence: .sisyphus/evidence/task-6-tui-model.txt

  Scenario: Empty plugin list skips TUI
    Tool: Bash (go test)
    Preconditions: Model with 0 plugins
    Steps:
      1. Run `go test ./internal/tui/ -run TestModelEmpty -v`
      2. Verify: confirmed=true immediately, no interaction needed
    Expected Result: Empty list handled gracefully
    Failure Indicators: Panic, infinite loop
    Evidence: .sisyphus/evidence/task-6-empty-list.txt
  ```

  **Commit**: YES
  - Message: `feat(tui): add bubbletea multi-select model`
  - Files: `internal/tui/model.go, internal/tui/model_test.go`
  - Pre-commit: `go test ./internal/tui/...`

- [ ] 7. OpenCode Subprocess Runner (TDD)

  **What to do**:
  - **테스트 먼저**: `internal/runner/runner_test.go`
    - opencode가 PATH에 있을 때 → 실행 성공 (re-exec 패턴으로 테스트)
    - opencode가 PATH에 없을 때 → 에러 반환
    - 인자 전달 확인: `["--model", "gpt-4"]` → opencode에 그대로 전달
    - stdin/stdout/stderr 연결 확인
    - 종료 코드 전달 확인
    - **테스트 방법**: `GO_TEST_PROCESS=1` 환경변수 re-exec 패턴
      - 테스트가 자기 자신을 서브프로세스로 재실행하여 mock opencode 역할
  - **구현**: `internal/runner/runner.go`
    - `type Runner struct { Command string }` — 실행할 명령어 (기본: "opencode")
    - `func NewRunner() *Runner`
    - `func (r *Runner) CheckAvailable() error` — `exec.LookPath(r.Command)` 로 확인
    - `func (r *Runner) Run(args []string) error`:
      - `exec.Command(r.Command, args...)`
      - `cmd.Stdin = os.Stdin`, `cmd.Stdout = os.Stdout`, `cmd.Stderr = os.Stderr`
      - `cmd.Run()` 실행
      - 종료 코드가 0이 아니면 `exec.ExitError`에서 코드 추출하여 `os.Exit(code)` 호출

  **Must NOT do**:
  - `syscall.Exec` 사용 금지
  - 인자 파싱/수정 금지 (그대로 전달)
  - opencode 출력 캡처 금지 (직접 터미널에 연결)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: os/exec 래퍼는 비교적 단순
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 8, 9)
  - **Blocks**: Task 8
  - **Blocked By**: Task 1

  **References**:

  **External References**:
  - `os/exec` — Go 표준 라이브러리 subprocess 실행
  - `exec.LookPath` — PATH에서 바이너리 검색
  - Re-exec 테스트 패턴: `TestHelperProcess` + `GO_TEST_PROCESS` 환경변수

  **Acceptance Criteria**:

  - [ ] `go test ./internal/runner/ -run TestRunner` → PASS (4+ test cases)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Subprocess receives correct arguments
    Tool: Bash (go test)
    Preconditions: Re-exec helper that echoes args
    Steps:
      1. Run `go test ./internal/runner/ -v`
      2. Verify args passed correctly to subprocess
    Expected Result: All args forwarded as-is
    Failure Indicators: Missing args, reordered args
    Evidence: .sisyphus/evidence/task-7-runner-args.txt

  Scenario: Missing command returns error
    Tool: Bash (go test)
    Preconditions: Command set to non-existent binary
    Steps:
      1. Run `go test ./internal/runner/ -run TestRunnerNotFound -v`
      2. Verify error returned from CheckAvailable()
    Expected Result: Clear error about missing command
    Failure Indicators: Panic, no error
    Evidence: .sisyphus/evidence/task-7-runner-missing.txt
  ```

  **Commit**: YES
  - Message: `feat(runner): add opencode subprocess launcher`
  - Files: `internal/runner/runner.go, internal/runner/runner_test.go`
  - Pre-commit: `go test ./internal/runner/...`

- [ ] 8. Main.go Wiring + Integration Tests (TDD)

  **What to do**:
  - **구현**: `cmd/oc/main.go` — 전체 플로우 연결 (<50줄 목표)
    1. `os.Args[1:]` 로 opencode에 전달할 인자 수집
    2. `runner.CheckAvailable()` — opencode 존재 확인
    3. `config.LoadOcConfig(homeDir + "/.oc")` — whitelist 로드 (없으면 nil)
    4. `config.ReadFile(homeDir + "/.config/opencode/opencode.json")` — 파일 읽기
       - 없으면 에러 메시지 출력 + `os.Exit(1)`
    5. `config.ParsePlugins(content)` — 플러그인 목록 파싱
    6. `plugin.FilterByWhitelist(plugins, whitelist)` — visible/hidden 분리
    7. visible이 비어있으면 TUI 스킵, 바로 opencode 실행
    8. `tui.NewModel(visibleItems)` → `tea.NewProgram(model).Run()`
    9. cancelled이면 `os.Exit(0)` (파일 미수정)
    10. `config.ApplySelections(content, selections)` — 주석 토글
        - hidden 플러그인은 selections에 포함하지 않으므로 원래 상태 유지
    11. `config.WriteConfigFile(path, modified)` — 파일 저장
    12. `runner.Run(args)` — opencode 실행
  - **앱 로직을 함수로 분리**: `func run() error` — 테스트 가능하게
  - **통합 테스트**: `cmd/oc/main_test.go` 또는 `internal/app/app_test.go`
    - temp 디렉토리에 opencode.json 생성
    - 전체 플로우 실행 (TUI 제외 — 프로그래매틱 선택)
    - 파일 수정 결과 검증
    - 에러 케이스 (파일 없음, 빈 배열)

  **Must NOT do**:
  - 글로벌 상태 사용 금지
  - 하드코딩 경로 금지 (모두 os.UserHomeDir + filepath.Join)
  - 새 기능 추가 금지 (기존 패키지 연결만)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: 모든 패키지를 올바르게 연결하고 통합 테스트 작성이 필요
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Sequential (Wave 3에서 Task 7과 순서 주의)
  - **Blocks**: Task 9
  - **Blocked By**: Tasks 4, 5, 6, 7

  **References**:

  **API/Type References**:
  - `internal/config/jsonc_parser.go:ParsePlugins` — 플러그인 파싱
  - `internal/config/jsonc_writer.go:ApplySelections` — 주석 토글
  - `internal/config/jsonc_writer.go:WriteConfigFile` — 파일 저장
  - `internal/config/toml_config.go:LoadOcConfig` — 설정 로드
  - `internal/plugin/plugin.go:FilterByWhitelist` — 화이트리스트 필터
  - `internal/tui/model.go:NewModel, Selections, Cancelled` — TUI 모델
  - `internal/runner/runner.go:NewRunner, CheckAvailable, Run` — 실행

  **Acceptance Criteria**:

  - [ ] `go test ./... ` → ALL PASS
  - [ ] `go build ./cmd/oc/` → 성공

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Full integration flow (programmatic)
    Tool: Bash (go test)
    Preconditions: Temp dir with opencode.json (3 plugins: 2 active, 1 commented)
    Steps:
      1. Run `go test ./cmd/oc/ -run TestIntegration -v` (or internal/app)
      2. Verify: programmatic selection of 1 plugin → file correctly modified
      3. Verify: commented plugins have // prefix
      4. Verify: active plugins have no // prefix
      5. Verify: non-plugin content preserved
    Expected Result: File correctly modified with all content preserved
    Failure Indicators: Wrong toggle, content loss, invalid JSONC
    Evidence: .sisyphus/evidence/task-8-integration.txt

  Scenario: Missing opencode.json exits with error
    Tool: Bash
    Preconditions: No opencode.json at expected path
    Steps:
      1. Run compiled oc binary with HOME set to temp dir
      2. Check stderr contains error message
      3. Check exit code is 1
    Expected Result: Error message on stderr, exit 1
    Failure Indicators: Panic, exit 0, no error message
    Evidence: .sisyphus/evidence/task-8-missing-config.txt

  Scenario: TUI renders and responds to input
    Tool: interactive_bash (tmux)
    Preconditions: opencode.json with 2 plugins at expected path
    Steps:
      1. Create tmux session: `new-session -d -s oc-test`
      2. Run oc binary: `send-keys -t oc-test "./oc" Enter`
      3. Wait 2s for TUI to render
      4. Capture pane: `capture-pane -t oc-test -p`
      5. Verify output contains "[*]" and plugin names
      6. Send 'q' to quit: `send-keys -t oc-test "q"`
    Expected Result: TUI shows plugin list with checkboxes
    Failure Indicators: No TUI output, crash, garbled text
    Evidence: .sisyphus/evidence/task-8-tui-render.png
  ```

  **Commit**: YES
  - Message: `feat: wire up main.go and add integration tests`
  - Files: `cmd/oc/main.go, cmd/oc/main_test.go`
  - Pre-commit: `go test ./...`

- [ ] 9. Cross-Platform Build + README

  **What to do**:
  - Makefile 최종 확인 및 보강:
    - `make build` — 현재 플랫폼 빌드
    - `make build-all` — darwin/arm64, darwin/amd64, windows/amd64
    - `make test` — `go test ./...`
    - `make clean` — dist/ 삭제
    - `make release VERSION=v1.0.0` — `gh release create` 래퍼
      - `gh release create $(VERSION) dist/* --title "$(VERSION)" --generate-notes`
    - ldflags로 버전 정보 주입: `-X main.version=$(VERSION)`
  - `cmd/oc/main.go`에 version 변수 + `--version` 플래그 추가
  - `README.md` 작성:
    - 프로젝트 설명 (영어 — GitHub 표준)
    - 설치 방법 (GitHub releases에서 다운로드)
    - 사용법: `oc [opencode args...]`
    - `~/.oc` 설정 예시
    - opencode.json 플러그인 형식 설명
    - 빌드 방법

  **Must NOT do**:
  - CI/CD 워크플로우 파일 생성 금지
  - GoReleaser 설정 금지
  - Homebrew formula 금지

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Makefile 수정과 README 작성
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Sequential (Task 8 완료 후)
  - **Blocks**: F1-F4
  - **Blocked By**: Task 8

  **References**:

  **External References**:
  - `gh release create` — https://cli.github.com/manual/gh_release_create
  - Go cross-compilation: `CGO_ENABLED=0 GOOS=X GOARCH=Y go build`

  **Acceptance Criteria**:

  - [ ] `make build-all` → 3 binaries in dist/
  - [ ] `./dist/oc-*-amd64 --version` → 버전 출력

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Cross-platform binaries build successfully
    Tool: Bash
    Preconditions: All code committed
    Steps:
      1. Run `make clean && make build-all`
      2. List `ls -la dist/`
      3. Verify 3 files: oc-darwin-arm64, oc-darwin-amd64, oc-windows-amd64.exe
      4. Check file sizes are reasonable (>1MB each)
    Expected Result: 3 binaries built, non-zero sizes
    Failure Indicators: Missing binaries, 0-byte files, build errors
    Evidence: .sisyphus/evidence/task-9-build-all.txt

  Scenario: Version flag works
    Tool: Bash
    Preconditions: Binary built with VERSION ldflags
    Steps:
      1. Run `make build VERSION=v0.1.0`
      2. Run `./oc --version`
      3. Verify output contains "v0.1.0"
    Expected Result: Version string printed
    Failure Indicators: No output, wrong version, crash
    Evidence: .sisyphus/evidence/task-9-version.txt

  Scenario: README exists and is accurate
    Tool: Bash
    Preconditions: README.md written
    Steps:
      1. Check README.md exists
      2. Verify it contains: installation, usage, config example sections
    Expected Result: README with key sections
    Failure Indicators: Missing README, missing sections
    Evidence: .sisyphus/evidence/task-9-readme.txt
  ```

  **Commit**: YES
  - Message: `build: add cross-platform Makefile and README`
  - Files: `Makefile, README.md, cmd/oc/main.go`
  - Pre-commit: `make build-all && go test ./...`

---

## Final Verification Wave

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run `go vet ./...` + `go test ./...`. Review all Go files for: type assertion without ok check, empty error handling, `fmt.Println` in prod code (should be stderr), unused imports, exported symbols without doc comments. Check AI slop: excessive comments, over-abstraction, generic variable names.
  Output: `Build [PASS/FAIL] | Vet [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [ ] F3. **Real Manual QA** — `unspecified-high`
  Start from clean state. Create test opencode.json with 3 plugins (2 active, 1 commented). Run oc via tmux. Navigate TUI, toggle selections, confirm. Verify file changes. Test Ctrl+C cancel. Test missing file error. Test whitelist filtering. Save evidence to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [ ] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built, nothing beyond spec. Check "Must NOT do" compliance. Detect cross-task contamination. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

| Order | Message | Files | Pre-commit |
|-------|---------|-------|------------|
| 1 | `init: scaffold Go module and project structure` | go.mod, .gitignore, Makefile, dirs | `go build ./...` |
| 2 | `feat(config): add JSONC plugin parser with TDD` | internal/config/jsonc*.go, *_test.go | `go test ./internal/config/...` |
| 3 | `feat(config): add TOML whitelist config reader` | internal/config/toml*.go, *_test.go | `go test ./internal/config/...` |
| 4 | `feat(plugin): add plugin model and whitelist filter` | internal/plugin/*.go, *_test.go | `go test ./internal/plugin/...` |
| 5 | `feat(config): add comment toggler and file writer` | internal/config/writer*.go, *_test.go | `go test ./internal/config/...` |
| 6 | `feat(tui): add bubbletea multi-select model` | internal/tui/*.go, *_test.go | `go test ./internal/tui/...` |
| 7 | `feat(runner): add opencode subprocess launcher` | internal/runner/*.go, *_test.go | `go test ./internal/runner/...` |
| 8 | `feat: wire up main.go and add integration tests` | cmd/oc/main.go, *_test.go | `go test ./...` |
| 9 | `build: add cross-platform Makefile and README` | Makefile, README.md | `make build-all` |

---

## Success Criteria

### Verification Commands
```bash
go test ./...           # Expected: all tests PASS
go vet ./...            # Expected: no issues
go build ./cmd/oc/      # Expected: binary built successfully
make build-all          # Expected: 3 binaries in dist/
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All tests pass (`go test ./...`)
- [ ] Cross-platform binaries built
- [ ] TUI renders correctly (verified via tmux)
- [ ] JSONC comment toggling preserves file formatting
