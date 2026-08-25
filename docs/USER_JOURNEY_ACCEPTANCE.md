# Phase 2 사용자 여정 검증

이 문서는 첫 사용, 복귀, Provider 복구의 확인 순서와 CLI 대응 명령을 고정합니다. 각 여정은 로컬 임시 데이터 디렉터리에서 실행합니다.

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

- 첫 사용: 빈 로컬 데이터 디렉터리에서 `첫 프로젝트를 등록합니다`, `프로젝트 등록`, `Provider 상태`를 확인했습니다.
- 복귀: 임시 Git 저장소 한 개를 등록한 상태에서 `오늘의 개발 상태를 확인합니다`, 프로젝트·저장소·확인 항목 수, 다음 행동을 확인했습니다.
- Provider 복구: `Codex 사용 가능`, `Claude 미설정`, `Gemini 미설정`을 한 Provider 그룹에서 확인했습니다. 기본 Agent Profile 경고는 중복 표시하지 않습니다.
- Assurance 결과: Quality Run·Agent 실행·효과 기록이 없을 때 `아직 검증 결과가 없습니다`를 표시합니다. 결과가 생기면 개수와 비용 상태를 표시합니다.
- 화면: 다음 스크린샷을 저장했습니다.
  - `artifacts/phase2-home-first-use.png`
  - `artifacts/phase2-home-established.png`
  - `artifacts/phase2-home-narrow.png`
  - `artifacts/phase2-diagnostics-providers.png`
- 키보드: `Enter`로 상단 `지금 점검` 동작을 실행하고 실행 중 비활성화·완료 복귀를 확인했습니다. in-app Browser의 `Tab` 포커스 이동은 초기 `main` 포커스 뒤 안정적으로 관찰되지 않아 전체 순회는 미실행으로 기록합니다. 소스의 skip link, `:focus-visible`, 명시적 label/heading은 정적 검사를 통과했습니다.
- Native Windows: PowerShell에서 Windows 경로로 서버·CLI·Go 테스트와 in-app Browser 화면을 실행했습니다. 실제 Codex 작업 호출, 사용자 인증, 별도 깨끗한 Windows 11 장치 수동 확인은 실행하지 않았습니다.
