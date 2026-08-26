# Assurance 효과 대시보드

Status: v0.10.0 effect-evidence hardening slice

이 문서는 “AI를 사용했다”가 아니라 “검증 결과가 어떤 변화로 이어졌는가”를
확인하는 화면과 데이터 경계를 정의합니다. 화면은 로컬 loopback UI에서만
제공하며, 근거 없는 숫자는 `확인 불가`로 표시합니다.

## 무엇을 증명하는가

효과 대시보드는 다음 질문에 답합니다.

1. 선택한 기간에 Quality Run과 Agent 실행이 얼마나 성공했는가.
2. 실제로 채택되고 재검증된 측정 효과가 몇 건인가.
3. 기준선이 있을 때 이전 동일 기간과 어떤 차이가 있는가.
4. 그 결과를 원본 실행, Finding, artifact, SHA-256, Trace ID까지 다시 열 수
   있는가.
5. 측정값·회귀 방지·사용자 추정·AI 추론·확인 불가를 섞지 않았는가.

## 지표와 경계

`GET /api/assurance/impact`는 기본 최근 30일, 최대 365일 범위를 사용합니다.
31일 이하는 일별, 그보다 긴 범위는 주별로 묶습니다. 현재 기간과 길이가 같은
이전 기간을 비교하며 이전 표본이 없으면 비교 상태를 `unavailable`로 둡니다.

| 지표 | 계산 | 의미 |
| --- | --- | --- |
| Quality Run | 선택 범위의 Run 수 | 반복 가능한 검증 실행량 |
| Quality 성공률 | 성공 Run / 전체 Run | 검증 실행의 통과 비율 |
| Agent 성공률 | 성공 invocation / 전체 invocation | Provider 실행 안정성 |
| 검증된 효과 | 측정 또는 회귀 방지 + 원본 연결 + artifact + 채택 commit + 채택 commit과 같은 HEAD의 성공한 재검증 run/commit | 재현 가능한 개선으로 세는 최소 조건 |
| 효과 채택률 | 채택 효과 / 효과 전체 | 결과가 실제 작업으로 이어진 비율 |
| 재검증률 | 재검증 효과 / 효과 전체 | 채택 후 다시 확인한 비율 |
| 근거 완결성 | 모든 evidence ID가 살아 있고 원본이 연결된 효과 / 효과 전체 | 결과를 다시 확인할 수 있는 비율 |
| 기록된 시간 절감 | `metricKey=time_saved`인 측정 효과 중 같은 단위의 합 | 재검증 가능한 측정 시간 변화. 다른 지표나 혼합 단위는 섞지 않음 |
| 예상 시간 절감 | `metricKey=time_saved`인 사용자 추정 효과 중 같은 단위의 합 | 운영자 추정치. 측정 효과나 검증된 개선으로 합산하지 않음 |

비율의 분모가 없거나 이전 기간의 표본이 없으면 0으로 대체하지 않습니다.
`증가`, `감소`, `변화 없음`, `비교 불가`를 구분합니다. Count 증가는 양적
변화일 뿐 자동으로 좋은 결과라고 주장하지 않습니다.

시간 절감은 단위를 변환하지 않습니다. 현재 기간에 단위가 섞였거나 이전 기간과
단위가 다르면 값 또는 기간 비교를 `확인 불가`로 표시합니다.

## Traceability 계약

각 실행은 다음 그래프를 남깁니다.

```text
Project / Repository / Worktree / Branch / HEAD
        ↓
AgentInvocation 또는 QualityRun
        ↓  (Trace ID, input/output/config digest)
masked Assurance Artifact
        ↓  (artifact ID, MIME, size, SHA-256, retention)
Assurance Effect
        ↓
baseline → observed value → adopted commit → successful reverification run
                                      ↘ same exact HEAD
```

`GET /api/assurance/traces/{effectID}`는 이 연결을 노드·링크·artifact 참조로
반환합니다. 공유 가능한 trace와 JSON 보고서에는 운영자 로컬 경로를 넣지
않습니다. 누락된 원본, Finding, artifact, trace 참조는 `missingRefs`로 남고
완결 효과로 세지 않습니다. 채택·재검증 boolean만 있고 commit 또는 재검증 run
참조가 없으면 기록은 남기지만 검증된 효과에는 포함하지 않습니다. 재검증 run이
성공하지 않았거나 그 HEAD가 채택 commit과 다르면 검증된 효과로 세지 않습니다.
Trace drill-down에는 원본 실행, 재검증 실행, 채택 commit, 재검증 commit을 각각
노드와 관계로 표시합니다.

`Trace ID`는 객체 ID와 별개의 연결 키입니다. 객체가 교체되어도 실행·근거·효과
사이의 연결을 추적할 수 있도록 입력/출력/설정 digest와 함께 기록합니다.

## 결과물 관리

- artifact는 저장 전에 마스킹하고 서버가 생성한 로컬 storage key에 저장합니다.
- 기본 quota는 512 MiB입니다. quota를 넘으면 새 결과를 조용히 버리지 않고
  저장을 거부합니다.
- `active`는 복구된 유효한 로컬 파일이 필요하고, `pinned`는 로컬 파일 또는
  검증된 archive로 유지할 수 있습니다.
- export는 staging → SHA-256 검증 → manifest 작성 → atomic rename 순서입니다.
- `assurance-manifest.json`은 artifact ID, 파일명, 크기, MIME, 원본 참조,
  SHA-256을 기록합니다.
- 고정된 근거는 확인 문구 없이 삭제할 수 없습니다. archive에서 복원할 때도
  manifest hash와 artifact hash를 다시 확인합니다.

API:

- `GET /api/assurance/impact`
- `GET /api/assurance/impact/export?format=json|csv`
- `GET /api/assurance/traces/{effectID}`
- `GET /api/assurance/artifacts/storage`
- `POST /api/assurance/artifacts/{artifactID}/retention`
- `POST /api/assurance/artifacts/{artifactID}/restore`

JSON 보고서는 선택한 현재 기간의 효과와 그 효과에 연결된 안전한 artifact 참조를
포함합니다. Storage 요약은 전체 로컬 quota 상태이며, 로컬 경로는 보고서에 넣지
않습니다.

Portable evidence packs can also be managed without the browser:

```powershell
dev-control-room assurance artifact list --json
dev-control-room assurance artifact export --ids artifact-1,artifact-2 --destination D:\assurance-pack --json
dev-control-room assurance artifact restore --id artifact-1 --json
```

## 정직한 한계

이 화면은 인과관계나 개인 생산성을 자동으로 판정하지 않습니다. 측정 효과가
완결되려면 원본 근거, 사람의 채택, 채택된 commit에서 성공한 재검증을 별도로
기록해야 합니다. 사용자 추정 시간은 별도 지표로 표시하며 측정값을 대신하지
않습니다. 현재 효과 기록 API는 이 메타데이터를 받을 수 있지만, AI가 patch를
채택하거나 commit/push하는 동작은 수행하지 않습니다.
