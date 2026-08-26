# ADR-002: Codex npm launcher 경계

- 상태: Accepted
- 날짜: 2026-08-26

## 결정

Codex npm launcher는 Windows `cmd.exe`나 임의 `.cmd`·`.bat` 파일을 실행하지 않습니다. Resolver가 PATH에서 찾은 launcher를 검증하고, 인접한 `@openai/codex/package.json`의 선언된 `bin/codex.js`를 확인한 뒤 `node.exe`와 검증된 JavaScript 경로, typed argv를 직접 실행합니다.

## 이유

명령 해석기와 사용자 입력의 결합을 제거하고, 실행 파일·스크립트·인자를 각각 검토 가능한 값으로 남깁니다. npm shim이 바뀌거나 다른 패키지를 가리키는 경우에는 실행하지 않고 상태를 `확인 필요`로 남깁니다.

## 검증 조건

- `node.exe`가 로컬 Windows 실행 파일이어야 합니다.
- 패키지 metadata가 존재하고 선언된 bin 경로가 상대 `bin/codex.js`여야 합니다.
- package root가 launcher와 인접한 `node_modules/@openai/codex`여야 합니다.
- argv는 구조화된 배열이며 셸 문자열로 재조합하지 않습니다.
- 검증 실패 시 native Provider 실행을 하지 않고 복구 안내만 제공합니다.

## v0.8.0 acceptance

`scripts/verify-real-codex.ps1`는 disposable local Git fixture에서 실제
authenticated Codex invocation을 실행해 46 assertions를 통과했습니다. 실행은
regular `node.exe`와 verified `@openai/codex/bin/codex.js`만 사용했고, raw
transcript를 저장하지 않으며 fixture Git status와 prompt persistence를
확인했습니다. npm `.cmd` shim은 발견 근거일 뿐 실행 대상이 아닙니다.

이 결과는 non-TTY/closed stdin, expired auth, provider approval prompt,
process-tree cancellation, crash/reboot, idempotent resume을 모두 받아들인다는
주장이 아닙니다. 이 resilience acceptance는
[#3](https://github.com/knowgyu/dev-control-room/issues/3)에 남습니다.
