# Dev Control Room 0.11.0

Windows 11용 로컬 우선 개발 제어실입니다. 등록한 프로젝트만 관찰하고,
근거가 있는 점검과 Action을 계획·승인·실행합니다. 서비스는 loopback에만
열리며 telemetry를 보내지 않습니다.

## 가장 짧은 사용 순서

1. `dev-control-room.exe`를 실행하고 <http://127.0.0.1:38471>을 엽니다.
2. `프로젝트`에서 폴더를 선택하고 등록할 Git 저장소를 명시적으로 고릅니다.
3. `진단`에서 도구, Agent Profile, GitHub/Jenkins/Kubernetes 연동을 설정합니다.
   credential 값이 아니라 `env:NAME` 또는 `credential_manager:NAME` 같은
   참조만 입력합니다.
4. `작업`에서 `기존 점검 찾기 → 제안 검토 → Checkset 만들기 → 적용 → 실행`을
   진행합니다.
5. Jenkins 대상 그룹이 필요하면 `진단 → Jenkins 대상 그룹`에서 완료된 build
   URL을 묶습니다. 이후 `작업`에서 외부 작업 또는 Stage/Production 계획을
   만들고 `Worktree 신뢰 → 승인 요청 → 전용 실행` 순서로 진행합니다.
6. 정리는 `진단 → 정리 후보`에서 안전 근거가 `검토 가능`인 경우에만 계획을
   만들 수 있습니다. 실행은 연결 Worktree와 정확한 branch를 다시 확인한 뒤
   Action Broker를 통과합니다.
7. `검증`에서 기간·프로젝트·Provider·모델을 선택하고 검증된 효과,
   이전 동일 기간 비교, 근거 완결성, trace, artifact 보관 상태를 확인합니다.
   보고서는 JSON/CSV로 내려받을 수 있습니다.

홈 화면에도 같은 4단계 흐름이 표시됩니다. 먼저 계획과 digest, 대상,
위험 등급을 확인하고 실행하세요.

## Windows 실행

압축을 풀고 아래처럼 실행합니다. 데이터는 기본적으로
`%LOCALAPPDATA%\DevControlRoom`에 저장됩니다.

```powershell
.\dev-control-room.exe
Start-Process http://127.0.0.1:38471
```

다른 데이터 폴더를 쓰려면 `serve --home`을 지정합니다.

```powershell
.\dev-control-room.exe serve --home "$env:TEMP\dev-control-room-fixture"
```

## 패키지 만들기

PowerShell 7.6과 Go가 설치된 Windows에서 다음 명령이 amd64/arm64 portable
ZIP과 SHA-256 목록을 만듭니다. 실제 Jenkins, production, Scheduler, 삭제
작업은 패키징에 포함되지 않습니다.

```powershell
pwsh -NoProfile -File .\scripts\package.ps1 -Version 0.11.0
```

이 명령은 `docs/RELEASE_NOTES_v0.11.0.md`와
`docs/VERIFICATION_v0.11.0.md`가 모두 있을 때 패키지를 만듭니다.

검증까지 포함한 후보 확인은 다음 명령을 먼저 실행합니다.

```powershell
pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full
```

## 0.11.0 범위와 경계

- 중단된 Agent 실행은 검증 대시보드에서 새 prompt를 입력해 명시적으로
  재시도할 수 있습니다. 원본 prompt는 저장하지 않으며, 새 실행은 원본
  실행 ID와 deterministic idempotency key로 연결합니다.
- CLI의 `assurance invocation retry`와 보호된 retry API도 같은 경계를
  사용합니다. 같은 retry 요청을 반복해도 Provider를 다시 실행하지 않습니다.
- Provider·진단 프로세스에는 상속 console 대신 명시적 EOF stdin을 전달합니다.
  Windows timeout/cancellation은 Job Object로 child process tree까지 종료합니다.
  `scripts/verify-native-resilience.ps1`가 native Windows에서 이 경계를 확인합니다.

- Established 홈에서 Quality Run·Agent·효과 기록뿐 아니라 검증된 효과,
  근거 완결성, 기록/예상 시간 절감을 함께 보여줍니다. 확인되지 않은 효과는
  0이나 측정값처럼 표시하지 않고 `확인 불가`로 남기며 상세 효과 추적으로
  이동할 수 있습니다.
- 0.10.0의 Assurance dashboard와 lifecycle CLI에 더해, 초기 focus를 빼앗지 않는
  첫 진입과 빈 추세의 간결한 상태를 제공합니다. 등록 양식은 닫은 뒤 연 버튼으로
  focus가 돌아옵니다.
- 0.9.1의 Assurance dashboard와 lifecycle CLI에 더해, 기간별 효과 비교·trace·JSON/CSV 보고서,
  manifest 기반 artifact 보관/복원, 반복 가능한 first-use/return/Provider
  recovery 여정, 등록된 local Quality Run을 포함합니다.
- 재시작 시 queue/running/cancelling 상태의 Assurance invocation을 자동 재실행하지
  않고 `interrupted`와 Resume Brief로 남깁니다.
- 효과 분류는 측정·회귀 방지·사용자 추정·AI 추론·확인 불가를 분리합니다. 기록된
  시간 절감과 예상 시간 절감도 별도 지표로 표시하며, verified effect는 원본·artifact·
  같은 채택 HEAD의 성공한 재검증까지 연결된 경우에만 셉니다.
- Trace ID만 있는 원본도 Provider/model 범위와 trace drill-down에 연결하며, 채택·
  재검증 커밋과 재검증 실행을 확인할 수 있습니다.
- 실행 파일을 더블클릭해도 시작 실패 원인·조치·다음 명령을 한국어로 보여주고,
  `troubleshoot`와 안전한 최근 진단 기록을 제공합니다. JSON 오류 계약도 유지합니다.
- 동일한 `--home`을 쓰는 서버·CLI·migration은 companion lock과 bounded retry로
  직렬화하며, timeout과 저장소 busy 상태를 구분합니다.
- Codex npm launcher는 로컬 `node.exe`와 검증된
  `@openai/codex\bin\codex.js`를 typed argv로 실행합니다. `cmd.exe`, 임의
  `.cmd`, `.bat`, bare `codex`는 실행하지 않습니다.
- 안전한 로컬 Git fixture에서 실제 authenticated Codex invocation과 native
  process resilience를 확인했지만, 회사 CI endpoint·credential·production,
  만료 인증·approval prompt, old Provider process 재개는 별도 acceptance와
  이슈 경계로 남습니다.

## 경계와 현재 확인 범위

- 대상 경로와 외부 endpoint는 설정한 값만 사용합니다.
- 비밀 값은 config, UI, CLI, HTTP, MCP, 로그, handoff에 저장하거나 출력하지
  않습니다.
- Production, 외부 Jenkins, destructive cleanup은 항상 별도 계획과 명시적
  승인이 필요합니다.
- `FallbackRunbookID`는 참조만 저장하며 자동으로 PowerShell을 이어 실행하지
  않습니다. 이어 실행은 별도 계약과 승인이 필요합니다.
- 0.11.0의 확인 범위는 자동화, native Windows process acceptance, 로컬
  Windows Browser 관찰, disposable local Git fixture의 실제 Codex invocation입니다.
  회사 Jenkins/GitHub/Kubernetes,
  proxy, production, second-device, full Tab/Space, native dialog Esc driver
  acceptance는 실행하지 않았습니다.

## 문서

- [현재 상태와 handoff](docs/HANDOFF.md)
- [효과 대시보드와 trace 계약](docs/ASSURANCE_EFFECT_DASHBOARD.md)
- [v0.11.0 검증 기록](docs/VERIFICATION_v0.11.0.md)
- [v0.11.0 릴리즈 노트](docs/RELEASE_NOTES_v0.11.0.md)
- [v0.10.3 검증 기록](docs/VERIFICATION_v0.10.3.md)
- [v0.10.3 릴리즈 노트](docs/RELEASE_NOTES_v0.10.3.md)
- [v0.10.2 검증 기록](docs/VERIFICATION_v0.10.2.md)
- [v0.10.2 릴리즈 노트](docs/RELEASE_NOTES_v0.10.2.md)
- [v0.10.1 검증 기록](docs/VERIFICATION_v0.10.1.md)
- [v0.10.1 릴리즈 노트](docs/RELEASE_NOTES_v0.10.1.md)
- [v0.10.0 검증 기록](docs/VERIFICATION_v0.10.0.md)
- [v0.10.0 릴리즈 노트](docs/RELEASE_NOTES_v0.10.0.md)
- [v0.9.1 검증 기록](docs/VERIFICATION_v0.9.1.md)
- [v0.9.1 릴리즈 노트](docs/RELEASE_NOTES_v0.9.1.md)
- [v0.9.0 검증 기록](docs/VERIFICATION_v0.9.0.md)
- [v0.9.0 릴리즈 노트](docs/RELEASE_NOTES_v0.9.0.md)
- [v0.8.0 검증 기록](docs/VERIFICATION_v0.8.0.md)
- [v0.8.0 릴리즈 노트](docs/RELEASE_NOTES_v0.8.0.md)
- [Windows acceptance 절차](docs/NATIVE_WINDOWS_SMOKE.md)
- [검증 playbook](docs/VERIFICATION_PLAYBOOK.md)
- [제품 계약](docs/PRODUCT.md)
- [구성·자격 증명 경계](docs/CONFIGURATION.md)
- [연동 설정](docs/INTEGRATIONS.md)
- [아키텍처](docs/ARCHITECTURE.md)
- [전체 구현 계획](docs/POST_MVP_EXECUTION_PLAN.md)
