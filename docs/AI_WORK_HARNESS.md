# AI 작업 harness

상태: active
갱신일: 2026-08-27

이 문서는 Dev Control Room을 이어서 수정하는 AI 작업의 실행 순서와
증거 기준을 고정합니다. 저장소 런타임의 기능이나 보안 경계를 바꾸지 않으며,
설치된 skill은 작업 지침일 뿐 배포 산출물의 의존성이 아닙니다.

## 목적

- 변경 유형에 맞는 지침만 읽어 컨텍스트를 절약합니다.
- 작은 coherent slice마다 테스트, 기록, 문서 상태, 커밋을 함께 끝냅니다.
- 효과 대시보드의 숫자를 실행량이나 AI 추정치와 혼동하지 않습니다.
- WSL, Windows toolchain, native Windows, 실제 Provider를 서로 대신 주장하지
  않습니다.
- 사람이 필요한 채택·승인·회사 시스템 접근은 질문과 Resume Brief로 남기고,
  독립적인 안전 작업은 계속합니다.

## Skill routing

Skill은 모두 `C:\Users\knowgyu\.codex\skills`에 설치되어 있습니다. 버전을
고정해야 하는 경우 설치 시점의 source/ref와 파일 digest를 검증 기록에 남깁니다.
현재 선택한 구성은 다음과 같습니다.

| 변경 유형 | 먼저 읽을 skill | 적용 기준 |
| --- | --- | --- |
| 화면 구조·시각 방향 | `frontend-design` | UI를 새로 만들거나 기존 화면의 정보 구조·스타일을 바꿀 때. 코드 전에 대상 사용자, 한 가지 목적, 토큰, wireframe, 기억될 한 요소를 정합니다. |
| UI 접근성·사용성 review | `web-design-guidelines` | review 전에 Vercel 원문을 다시 가져옵니다. `https://raw.githubusercontent.com/vercel-labs/web-interface-guidelines/main/command.md`의 최신 규칙을 파일·라인별로 적용합니다. |
| 실제 브라우저 여정 | `playwright` | loopback 화면의 snapshot → 상호작용 → 재-snapshot → screenshot 순서를 지킬 때. 가능한 한 `output/playwright/`에 증거를 둡니다. |
| OS 화면 캡처 | `screenshot` | 브라우저 도구로 얻을 수 없는 실제 창·바탕화면 증거가 필요할 때만 사용합니다. |
| CLI 계약·도움말 | `cli-creator` | 명령 표면, JSON envelope, doctor, auth/config, 외부 경로 smoke를 설계·검토할 때. 이 앱은 기존 Go CLI와 local-first 경계를 우선합니다. |
| 일반 Go 변경 | `golang-code-style`, `golang-error-handling`, `golang-testing` | export surface, 오류 chain, table-driven/독립 테스트를 검토할 때. |
| goroutine·lock·파일 동시성 | `golang-concurrency`, `golang-troubleshooting` | 저장소 lock, process 경합, cancellation, leak, race를 바꿀 때. |
| SQLite·migration·transaction | `golang-database`, `golang-error-handling` | context, rows close, migration checksum, transaction 경계를 바꿀 때. |
| trust boundary·파일·Provider argv | `golang-security` | path, process, secret masking, typed argv, 외부 입력을 바꿀 때. 보안 완화는 별도 승인 없이 하지 않습니다. |
| 효과·로그·측정 | `golang-observability` | 구조화된 이벤트, traceability, metric/usage/effect evidence를 바꿀 때. PII와 무제한 cardinality를 금지합니다. |

추가 skill의 cross-reference는 필요한 파일만 읽습니다. 모든 skill을 매번
동시에 적용하지 않으며, 회사 정책·`AGENTS.md`·제품 문서가 외부 skill보다
우선합니다.

## 작업 루프

### 1. 시작 전

1. `AGENTS.md`, `docs/HANDOFF.md`, 이 문서, `docs/VERIFICATION_PLAYBOOK.md`,
   관련 계획·ADR·최근 verification을 읽습니다.
2. `git status --short --branch`, `git rev-parse HEAD`, `git diff --check`를
   기록합니다. 기존 사용자 변경과 main의 미푸시 문서는 건드리지 않습니다.
3. 변경 경계를 한 문장으로 정하고, 그 경계 밖의 파일은 읽기 전용으로 둡니다.
4. 해당 skill의 `SKILL.md`와 실제로 필요한 linked reference를 먼저 읽습니다.
5. 완료 주장과 이를 증명할 focused test, 전체 gate, 사용자 여정을 표로 만듭니다.

### 2. 구현 중

- 먼저 실패를 재현하는 테스트나 fixture를 추가합니다.
- Go 오류는 반환 또는 한 곳에서만 구조화해 기록하고, `%w`와 `errors.Is/As`로
  경계를 보존합니다. 사용자에게는 기술 세부 대신 원인과 다음 조치를 보여줍니다.
- goroutine에는 명확한 종료·취소·대기 소유자가 있어야 합니다. 파일 lock과
  SQLite busy 재시도는 제한 시간과 typed error를 갖고, 무한 재시도하지 않습니다.
- DB 작업은 context를 전달하고 rows/transaction을 닫으며, migration은
  checksum·history·future version을 fail closed로 검증합니다.
- Provider나 process 실행은 `node.exe`/검증된 script/typed argv처럼 각각
  검토 가능한 값으로 유지합니다. `cmd.exe`, 임의 `.cmd`/`.bat`, shell 문자열
  재조합, raw transcript·secret 저장을 추가하지 않습니다.
- UI는 상태·결과·다음 action을 먼저 보이고, 상세 evidence는 점진적으로 엽니다.
  빈 상태·loading·partial failure·blocked·success를 모두 설계합니다. 버튼·
  상태·라벨은 짧은 명사형, 설명 문장은 필요한 만큼만 `합니다`체로 씁니다.

### 3. 변경 직후 검증

가능한 가장 작은 순서로 실패 원인을 찾고, 한 번에 한 가설만 바꿉니다.

```text
gofmt -l <changed-go-files>
go test -count=1 <focused-packages> -run '<focused-pattern>'
go test -count=1 ./...
CGO_ENABLED=1 go test -count=1 -race ./...
go vet ./...
go mod verify
go build ./...
node --check internal/app/ui/app.js
git diff --check
```

Windows가 필요한 변경은 native NTFS checkout에서 다음도 실행합니다.

```powershell
pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full
```

UI 변경은 별도로 다음을 기록합니다.

1. Vercel 원문 guideline 재조회와 파일·라인별 review
2. Playwright 또는 in-app Browser로 fresh snapshot 기반 desktop/narrow 여정
3. skip link, Tab/Shift+Tab, Enter/Space, Esc, focus-visible, live region,
   오류 복구, 빈 상태 확인
4. 안정적인 screenshot과 console 오류 결과

WSL test, cross-build, fake Provider, local fixture는 native Windows 실행이나
실제 회사 Provider 접근을 증명하지 않습니다. 실행하지 못한 항목은 PASS로
적지 않고 `not run`과 이유를 남깁니다.

## 증거와 효과 추적

각 coherent slice의 verification 기록에는 최소한 다음을 남깁니다.

| 항목 | 기록 |
| --- | --- |
| Source | 정확한 commit SHA, branch, 변경 범위 |
| Environment | OS, PowerShell, Go, Node, gcc와 native/WSL 구분 |
| Claim | 무엇을 증명하는지 한 문장 |
| Commands | focused, full, race, vet, build, UI/journey 명령과 결과 |
| Artifacts | report/screenshot/manifest의 저장 위치와 SHA-256. secret·로컬 절대경로는 제외 |
| Gaps | 실행하지 못했거나 아직 사람에게 남은 acceptance |
| Next action | 남은 문제, issue, 필요한 질문 또는 다음 milestone |

효과 대시보드의 measured 효과는 다음 연결이 모두 있을 때만 집계합니다.

```text
Finding / Quality Run / invocation
        ↓ stable identity + exact HEAD
masked report / artifact manifest + SHA-256
        ↓ human adoption or verified protection
adopted commit + fresh Worktree observation
        ↓ successful re-verification
Effect classification + trace nodes + period/provider/model scope
```

`measured`, `prevented regression`, `user-estimated`, `AI inference`,
`unavailable`을 분리합니다. 실행 횟수, token 수, 가격 equivalent, 추정 시간은
측정 효과의 대체값이 아닙니다. 원인·채택·재검증이 없으면 대시보드에 효과를
만들지 않고 evidence quality와 unavailable 상태로 보입니다.

## 사람에게 넘길 때

사람의 결정이 필요한 경우 아래 형식으로 저장합니다.

```text
Question: 한 가지 선택만 묻는 짧은 질문
Why blocked: 현재 scope/authority/evidence가 부족한 이유
Safe work completed: 독립적으로 끝낸 작업
Resume Brief: source SHA, selected scope, completed/pending runs, failed evidence,
  changed files, next safe command
```

사람에게 답을 기다리는 동안 read-only 분석, fixture 보강, 문서·검증 기록
정리는 계속합니다. 새 보안 경계 완화, 실제 회사 시스템 변경, push/release
권한 확대가 필요한 경우에만 멈춥니다.

## Milestone·commit·release 규칙

- 한 milestone 또는 하나의 coherent vertical slice만 staging합니다.
- focused test와 해당 evidence 문서를 먼저 갱신하고, `ROADMAP.md`,
  `HANDOFF.md`, 활성 계획의 상태를 같은 slice에서 갱신합니다.
- 상태는 `implemented`/`verified`/`partial`/`not run`을 구분합니다. 테스트가
  통과했다는 이유만으로 native, Provider authority, human adoption을 완료로
  표시하지 않습니다.
- 검토 후 하나의 설명 가능한 커밋을 만들고, 커밋 전 `git diff --cached`와
  `git status`를 다시 확인합니다.
- 사용자가 허용한 patch/minor release는 검증된 slice별로 만들 수 있지만,
  tag/release에는 정확한 source SHA, Windows 산출물, SHA-256, release note,
  PASS/not-run/gaps를 함께 넣습니다. push는 사용자가 승인한 release 단계에서만
  합니다.

## 설치된 출처

2026-08-27 현재 다음 source에서 user-scoped Codex skill을 설치했습니다.

- `anthropics/skills`: `frontend-design`
- `vercel-labs/agent-skills`: `web-design-guidelines`
- `samber/cc-skills-golang`: `golang-code-style`, `golang-error-handling`,
  `golang-testing`, `golang-concurrency`, `golang-database`,
  `golang-troubleshooting`, `golang-security`, `golang-observability`
- `openai/skills` curated: `playwright`, `screenshot`, `cli-creator`

다음 설치·업데이트 시에는 `npx skills find <query>`로 후보를 확인하고,
공개 source의 `SKILL.md`를 검토한 뒤 설치합니다. 이 프로젝트의 current
Codex desktop harness에서는 설치 helper가 `C:\Users\knowgyu\.codex\skills`에
직접 배치한 결과를 기준으로 합니다.
