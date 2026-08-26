# ADR-003: Unattended Approval Scope는 정확한 실행 계약이다

- 상태: Accepted
- 날짜: 2026-08-27

## Context

비대면으로 계속할 수 있는 작업은 편리하지만, Provider·도구·경로·기한이
바뀐 작업을 기존의 사람 승인으로 실행하면 승인 범위를 조용히 넓힐 수
있습니다. 단순한 `allow automation` 플래그나 프로젝트 단위 권한은 정확한
Worktree와 실행 시점의 입력을 증명하지 못합니다.

## Decision

- `UnattendedApprovalScope`는 draft → approved → revoked의 revisioned
  레코드로 저장합니다. 승인 시점에는 현재 관찰된 Project/Repository/
  Worktree만 바인딩합니다.
- Scope는 Action type/risk, Provider profile, technique, tool setup·version,
  tool/config·argument-schema digest, writable path, network policy, disk
  quota, deadline, mandatory prohibited operation을 모두 열거합니다.
- `commit`, `push`, PR 생성, CI 편집, remote dispatch, deletion, scope
  expansion은 기본 금지 집합이며 Scope 입력이 이를 약화할 수 없습니다.
- 각 ActionPlan은 Scope digest와 전체 match 결과·이유·시각을 보존합니다.
  Plan 생성, Admit 직전, Execute 직전에 현재 Scope와 exact request를 다시
  비교하고, mismatch·만료·revoke·관찰된 Worktree 변경은 fail closed합니다.
- 승인 actor는 요청 입력에서 받지 않습니다. 서비스가 승인 ceremony를
  통해 `local-user`를 기록하고, revoke 또는 실행 전 거부 시 이미 잡힌
  lease를 정리합니다.
- 저장은 기존 local SQLite assurance object와 revision CAS를 사용합니다.
  새 서버나 blanket permission은 추가하지 않으며, API mutation은 기존
  loopback mutation token 경계를 유지합니다.

## Consequences

비대면 자동화는 더 많은 명시적 계약을 작성해야 하지만, 승인된 작업과
실제로 실행된 작업의 차이를 digest·match 이유·Worktree 관찰·postcheck로
추적할 수 있습니다. Provider 권위, 실제 native resilience, AI patch
adoption은 이 Scope가 대신 증명하지 않으며 각각의 Phase 1 milestone과
사람 검증에 남습니다.

## Verification

- `docs/VERIFICATION_MILESTONE_A_APPROVAL_SCOPE.md`
- `internal/domain/approval_scope_test.go`
- `internal/store/approval_scope_test.go`
- `internal/action/approval_scope_test.go`
- `internal/app/approval_scope_test.go`

## Links

- [ADR-001: AI-assisted Code Assurance](ADR-001-ai-assisted-code-assurance.md)
- [AI Code Assurance plan](../AI_CODE_ASSURANCE_PLAN.md)
