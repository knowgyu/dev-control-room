# Phase 2 사용자 여정 검증

이 문서는 첫 사용, 복귀, Provider 복구의 확인 순서와 CLI 대응 명령을 고정합니다. 각 여정은 로컬 임시 데이터 디렉터리에서 실행합니다.

## 검증 분류

| 분류 | 의미 |
| --- | --- |
| 자동 검증 | CLI와 loopback HTTP로 반복 실행하며 결과를 assertion으로 확인합니다. |
| 로컬 관찰 | 이 Windows 작업 환경의 in-app Browser에서 화면과 단일 동작을 확인했습니다. |
| 미실행 | 실제 Provider/회사 시스템, 전체 키보드 순회, 별도 Windows 장치처럼 아직 실행하지 않은 항목입니다. |

반복 가능한 CLI/loopback gate는 다음 명령으로 실행합니다. 스크립트는 임시 폴더에만 실행 파일·app home·Git fixture를 만들고, 실제 Provider나 회사 endpoint를 호출하지 않습니다. 성공 후에는 자신이 만든 임시 root만 정리하며, 실패 원인 확인이 필요하면 `-KeepTemp`를 사용합니다.

```powershell
pwsh -NoProfile -File .\scripts\verify-phase2-journeys.ps1
```

검증 대상은 첫 사용 전 빈 상태, fixture 등록 후 복귀/설정 상태, 선택적 Provider 상태 그룹, Assurance sessions/campaigns/runs/invocations/artifacts/effects/pricing read path와 빈 dashboard입니다. UI 시각 검증을 대신하지 않습니다.

## 첫 사용

1. `dev-control-room --help`로 시작 경로를 확인합니다.
2. `dev-control-room serve --home <temp> --listen 127.0.0.1:38471`로 로컬 화면을 엽니다.
3. 홈에서 `프로젝트 등록`을 선택합니다.
4. 폴더를 선택하고 저장소 후보를 읽기 전용으로 확인합니다.
5. `지금 점검`으로 첫 상태를 생성합니다.

CLI 대응:

```powershell
dev-control-room project add --name sample --path C:\work\sample --home $temp
dev-control-room project list --json --home $temp
dev-control-room env doctor --json --home $temp
```

## 복귀

1. 홈에서 프로젝트 수, 저장소 수, 열린 확인 항목, 마지막 점검을 확인합니다.
2. `지금 확인할 항목`에서 가장 높은 상태를 먼저 엽니다.
3. Provider 상태는 `사용 가능`, `미설정`, `확인 필요`로 구분합니다.
4. 이전 세션과 품질 실행은 기록 화면에서 다시 엽니다.

CLI 대응:

```powershell
dev-control-room project list --json --home $temp
dev-control-room assurance provider --json --home $temp
dev-control-room assurance dashboard --json --home $temp
```

## Provider 복구

1. 진단의 `Provider 상태`에서 실패 원인과 확인된 실행 경로를 확인합니다.
2. Codex npm launcher는 `node.exe`와 검증된 `@openai/codex`의 `bin/codex.js`만 사용합니다.
3. `cmd.exe`, 임의 `.cmd`, `.bat` 파일은 실행하지 않습니다.
4. 설치·경로를 수정한 뒤 `Provider 다시 점검`을 실행합니다.
5. 상태가 `사용 가능`이 되기 전에는 Agent 실행을 시작하지 않습니다.

## 키보드·스크린샷 확인

- `Tab` 순서가 사이드 탐색 → 상단 기본 동작 → 본문 동작 순서인지 확인합니다.
- `Enter`로 링크·버튼을 실행하고 `Space`로 버튼을 실행합니다.
- `Skip to content`가 키보드 첫 포커스에서 보이는지 확인합니다.
- 다이얼로그가 열리면 제목과 첫 입력에 포커스가 있고 `Esc`로 닫히는지 확인합니다.
- 홈, 진단, Provider 상태의 스크린샷을 저장합니다.
- 색상을 끄거나 흑백으로 보아도 상태명과 다음 행동을 읽을 수 있어야 합니다.

## 2026-08-26 검증 기록

- 자동 검증: `pwsh -NoProfile -File .\scripts\verify-phase2-journeys.ps1`가 239 assertions를 통과했습니다. fresh temporary app home과 두 local Git fixture에서 first-use/return, local npm Codex recovery, fake-provider Assurance, registered static Quality Run, blocked mutation/action, duplicate-warning grouping, secret canary non-persistence, restart를 확인했고 만든 임시 root만 정리했습니다.
- 실제 Provider 경계: `pwsh -NoProfile -File .\scripts\verify-real-codex.ps1`가 disposable local Git fixture에서 실제 authenticated Codex invocation을 실행해 46 assertions를 통과했습니다. `node.exe`와 verified `@openai/codex/bin/codex.js`만 실행하고 `.cmd` shim·raw transcript·prompt persistence가 없음을 확인했습니다.
- 로컬 관찰 — 첫 사용과 복귀: 빈 home은 onboarding과 `프로젝트 등록`만 우선 보였고, local Git fixture 등록 뒤에는 `오늘의 개발 상태를 확인합니다`, 프로젝트/저장소/확인 항목/Provider 지표, 다음 행동과 Assurance 가치를 보였습니다.
- 로컬 관찰 — 회복과 근거: optional Claude/Gemini는 한 Provider 카드의 `미설정`으로 보이며 전역 오류가 되지 않았습니다. `진단` 링크는 `provider-statuses`로 초점을 옮겼고, finding 링크는 정확한 finding 카드로 초점을 옮겼습니다.
- 로컬 관찰 — 등록과 키보드: 등록 양식은 첫 입력으로 초점을 옮기고, 성공 뒤 선택된 project card가 `aria-pressed=true`가 됐습니다. 해당 card의 `Enter` 동작을 확인했습니다. 등록 해제 dialog는 description을 연결하고 취소 뒤 opener가 남아 있지 않으면 `main-content`로 안전하게 복귀합니다.
- 로컬 관찰 — 화면: Browser console error는 없었습니다. 다음 v0.8.0 후보 스크린샷을 저장했습니다.
  - `artifacts/verification-v0.8.0-final/ui-first-use-final.png`
  - `artifacts/verification-v0.8.0-final/ui-return-final.png`
  - `artifacts/verification-v0.8.0-final/ui-return-narrow-final.png`
- 미실행 또는 미수용: in-app Browser driver가 native dialog `Esc`와 full `Tab`/`Space` traversal을 신뢰성 있게 전달하지 않아 PASS로 주장하지 않습니다. 회사 Jenkins/GitHub/Kubernetes endpoint, production, real credential mutation, second clean Windows device/assistive technology, non-empty UI Quality Run review, blocked/approval-required UI 전체 여정도 실행하지 않았습니다.
