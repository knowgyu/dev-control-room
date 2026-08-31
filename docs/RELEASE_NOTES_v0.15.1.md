# Dev Control Room v0.15.1

이번 patch 릴리즈는 제품 자체의 첫 production dogfood을 반복 가능하게
측정할 수 있는 foundation을 추가합니다. 측정 결과는 품질 점수나 생산성
주장이 아니라, 고정된 검증 명령과 제한된 선택적 관찰을 보존하는 근거
manifest입니다.

## 포함 내용

- 순수 Go `internal/measurement` 패키지에 `DogfoodMeasurementRun`과
  `Measurement` JSON 계약을 추가했습니다. quality, performance, process,
  runtime 범주와 pass/fail/unknown status, measured/estimated/inferred/
  unavailable provenance, bounded raw samples, min/p50/p95/max, optional
  baseline/delta, 재현성 envelope를 명시합니다.
- percentile 계산은 정렬된 bounded sample의 선형 보간으로 결정론적으로
  계산합니다. 비어 있는 pass evidence, non-finite 값, summary 불일치,
  128개 초과 raw sample, 절대 경로 형태의 identity를 fail-closed로
  거부합니다.
- PowerShell 7.6 `scripts/measure-dogfood.ps1`가 `gofmt`, 일반/race Go
  test, vet, module verification, build, embedded UI JavaScript syntax를
  고정 명령으로 측정하고 JSON manifest와 human report를 생성합니다.
  coverage와 loopback `/api/health`, `/api/state` latency는 optional이며,
  선택되지 않았거나 사용할 수 없으면 unknown/unavailable로 기록합니다.
- `scripts/verify-measurement-contract.ps1`가 manifest의 version, status,
  provenance, bounded samples, deterministic summaries, required gate,
  absolute-path 경계를 별도로 확인합니다.
- 측정 계약과 evidence boundary를
  `docs/DOGFOOD_MEASUREMENT.md`에 기록하고 README 및 handoff에 사용법과
  범위를 연결했습니다.

## 실제 clean dogfood 결과

커밋 `f193287a5749757b0e0e0cc7d37c14bdf153c3d5`에서 native Windows
PowerShell 7.6 환경으로 clean worktree dogfood을 실행했습니다. Run ID는
`dogfood-be829600397d4f0a841c4aa09b9a52e2`이며, 12개 measurement가 생성되고
required gate는 `pass`, Go total statement coverage는 `57.9%`였습니다.
선택적 loopback probe는 실행하지 않았으므로 `/api/health`와 `/api/state`
metric은 각각 `unknown`/`unavailable`/0 samples로 기록되었습니다.

## 유지되는 경계

- 릴리즈 산출물은 Windows amd64 portable ZIP과 `SHA256SUMS`만 제공합니다.
  arm64는 검증용 교차 빌드만 수행하며 Linux/arm64 릴리즈 asset은 만들지
  않습니다.
- 이번 foundation은 aggregate score, Prometheus/OpenTelemetry, 외부 CI
  authority, mutation runner, patch adoption, causal productivity claim을
  추가하지 않습니다.
- runner는 native Windows에서 검토된 고정 명령만 실행하며, 명령 output·
  secret·repository/output absolute path를 manifest와 report에 넣지
  않습니다. 외부 서버나 destructive Action을 호출하지 않습니다.
