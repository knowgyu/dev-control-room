# Dev Control Room v0.15.2

이번 patch 릴리즈는 v0.15.1의 reproducible dogfood measurement foundation을
실제 제품 workflow에 연결하고, 그 foundation에서 확인된 세 가지 경계를
보강합니다. 측정 결과는 품질 점수나 생산성 주장이 아니라, 제한된 명령과
선택적 관찰을 보존하는 근거 manifest입니다.

## 포함 내용

- v0.15.1에서 추가한 versioned `DogfoodMeasurementRun`/`Measurement` 계약,
  bounded summary, reproducibility envelope, PowerShell dogfood runner와
  contract validator를 유지하면서 release-prep 결과를 실제 API workflow로
  확인했습니다.
- 실제 manifest import/list/get/dashboard workflow를 제공합니다. JSON 파일을
  `POST /api/assurance/measurement-runs/import`로 검증·저장하고,
  `/api/assurance/measurement-runs`에서 목록을 조회하며,
  `/api/assurance/measurement-runs/{runID}`에서 상세 manifest를 조회하고,
  `/api/assurance/measurement-runs/dashboard`에서 안전한 최신 요약과
  비교 정보를 조회합니다. UI의 파일 선택과 검증 대시보드도 같은 경계를
  사용합니다.
- 저장소 masking 경계를 measurement 전용으로 보강했습니다. run,
  measurement, command identity/reference 필드는 masking 때문에 변하지
  않도록 복원하고, masking 후 JSON을 다시 decode·validate한 뒤에만
  저장합니다. token-shaped ID가 보존되고 자유 텍스트의 secret은 계속
  가려지는 regression test를 추가했습니다.
- safe text 검증은 문자열 앞부분만 확인하지 않고 embedded Windows drive,
  UNC, Unix absolute path token도 거부합니다. `go version ... windows/amd64`,
  `GET /api/health` 같은 안전한 version/HTTP command token은 허용합니다.
- measurement 최신 결과가 존재할 때 legacy Assurance empty state가 계속
  보이는 UI 모순을 수정했습니다. measurement load 완료 후 legacy DOM도
  다시 렌더링하며, embedded UI regression test가 이 상태 전환을 확인합니다.

## 실제 clean dogfood 결과

release-prep를 시작하기 전 clean tree인 commit
`3dec04eeffb74bb82b5896a4317b131bff7e124b`에서 native Windows PowerShell
7.6 dogfood을 실행했습니다. Run ID는
`dogfood-e878541d582343d091bffea3872f382d`이며, manifest는 12개 measurement,
overall status `pass`, required failures `none`, Go total statement coverage
`58.4%`를 기록했습니다. commit과 head는 모두 위 SHA이고 `dirtyState`는
`clean`입니다. 선택적 server probe는 실행하지 않았으므로 `/api/health`와
`/api/state`는 각각 `unknown`/`unavailable` 및 0 samples로 기록되었습니다.

재현 명령과 별도 contract 검증 명령은 다음과 같습니다.

```powershell
pwsh -NoProfile -File .\scripts\measure-dogfood.ps1 -OutputDirectory artifacts\dogfood-v0.15.2
pwsh -NoProfile -File .\scripts\verify-measurement-contract.ps1 -ManifestPath artifacts\dogfood-v0.15.2\dogfood-measurement.json
```

## 유지되는 경계

- 릴리즈 산출물은 Windows amd64 portable ZIP과 `SHA256SUMS`만 제공합니다.
  Windows arm64는 검증용 cross-build일 뿐 release asset이 아니며, Linux나
  arm64 release package는 만들지 않습니다.
- runner와 import 경계는 command output, secret, repository/output absolute
  path를 저장하거나 출력하지 않습니다. `unknown`/`unavailable`은 pass가
  아니며 비교 가능한 evidence를 얻지 못했다는 뜻입니다.
- 이번 patch는 aggregate score, 외부 CI authority, mutation runner,
  patch adoption, causal productivity claim을 추가하지 않습니다.
