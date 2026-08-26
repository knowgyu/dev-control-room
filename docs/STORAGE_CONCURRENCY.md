# SQLite 동시 접근

## 범위

같은 `--home`을 사용하는 Dev Control Room의 `serve` 프로세스와 별도 CLI
프로세스는 하나의 `state.db`를 공유할 수 있습니다. 저장소 초기화, migration,
일반 읽기·쓰기, 트랜잭션은 모두 `internal/store`의 SQLite 연결 경계를
사용합니다.

## 직렬화와 재시도

파일 데이터베이스 옆에 비어 있는 companion 잠금 파일을 둡니다.

```text
state.db
state.db.lock
```

잠금 파일에는 데이터나 식별자를 쓰지 않으며 Unix에서는 `flock`, Windows에서는
`LockFileEx` 배타 잠금을 사용합니다. 잠금은 다음 범위에서만 유지됩니다.

- `Open`의 WAL·foreign-key 설정과 migration 단계
- 일반 `Exec`/`Query`/`Prepare` 작업
- `BeginTx`부터 `Commit` 또는 `Rollback`까지의 트랜잭션
- query rows가 닫히거나 EOF에 도달할 때까지의 결과 읽기

SQLite의 WAL, foreign-key enforcement, forward-only immutable migration 검증은
그대로 유지합니다. 잠금 파일을 삭제해서 stale lock을 해결하면 안 됩니다.
운영체제가 프로세스 종료 시 파일 잠금을 해제하므로 파일 자체는 남아도 다음
프로세스가 재사용할 수 있습니다.

잠금 대기는 최대 5초이며, SQLite가 외부 잠금으로 `busy/locked`를 반환하는
경우에도 짧은 bounded retry를 수행합니다. 한도를 넘으면 `StorageBusyError`
가 반환되고 `errors.As` 또는 `IsStorageBusy`로 구분할 수 있습니다. 이 오류의
메시지에는 경로, SQL 원문, 비밀, 운영체제 오류 원문이 포함되지 않습니다.

```go
var busy *store.StorageBusyError
if errors.As(err, &busy) {
    // 잠시 후 같은 명령을 다시 실행하거나 다른 프로세스를 종료합니다.
}
```

## 검증

저장소 패키지의 동시성 검증은 별도 프로세스로 같은 임시 `state.db`를 열어
초기화·읽기·쓰기를 반복합니다. 테스트는 사용자 홈, 실제 프로젝트, 외부
Provider를 사용하지 않습니다.

```powershell
go test -count=1 ./internal/store -run 'Test(OpenConcurrentInitializationIsSerialized|OpenReturnsContextDeadlineWhenStorageLockWaitExpires|StorageLockReturnsTypedBusyAfterBoundedWait|StorageServerAndCLIProcessesSerialize)$'
```

실제 Windows 실행에서는 `go test`와 함께 임시 디렉터리에서 동일한 바이너리의
`serve`와 CLI를 직접 시작해 확인해야 합니다. 이 문서와 저장소 테스트는 그
교차 프로세스 저장소 계약을 검증하지만, CLI/HTTP adapter가 사용자에게 어떤
친절한 조치 문구를 표시할지는 각 adapter의 별도 계약입니다.
