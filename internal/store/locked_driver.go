package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"sync"

	"modernc.org/sqlite"
)

const storageDriverName = "dev-control-room-sqlite"

var registerStorageDriver sync.Once

func openStorageDatabase(path string) (*sql.DB, error) {
	registerStorageDriver.Do(func() {
		sql.Register(storageDriverName, &storageDriver{base: &sqlite.Driver{}})
	})
	return sql.Open(storageDriverName, path)
}

type storageDriver struct {
	base driver.Driver
}

func (d *storageDriver) Open(name string) (driver.Conn, error) {
	connection, err := d.base.Open(name)
	if err != nil {
		return nil, err
	}
	return &storageConn{base: connection, lockPath: storageLockPath(name)}, nil
}

type storageConn struct {
	base     driver.Conn
	lockPath string

	stateMu sync.Mutex
	inTx    bool
	txLock  *storageLock
}

func (c *storageConn) isInTransaction() bool {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	return c.inTx
}

func (c *storageConn) setTransaction(lock *storageLock) {
	c.stateMu.Lock()
	c.inTx = true
	c.txLock = lock
	c.stateMu.Unlock()
}

func (c *storageConn) endTransaction(lock *storageLock) {
	c.stateMu.Lock()
	if c.txLock == lock || lock == nil {
		lock = c.txLock
		c.txLock = nil
		c.inTx = false
	}
	c.stateMu.Unlock()
	_ = lock.Close()
}

func (c *storageConn) withLock(ctx context.Context, fn func() error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if hasStorageLock(ctx) || c.isInTransaction() || c.lockPath == "" {
		return retryStorageOperation(ctx, fn)
	}
	lock, err := acquireStorageLock(ctx, c.lockPath)
	if err != nil {
		return err
	}
	defer func() { _ = lock.Close() }()
	return retryStorageOperation(ctx, fn)
}

func (c *storageConn) withResultLock(ctx context.Context, fn func() (driver.Result, error)) (driver.Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if hasStorageLock(ctx) || c.isInTransaction() || c.lockPath == "" {
		return retryStorageResult(ctx, fn)
	}
	lock, err := acquireStorageLock(ctx, c.lockPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = lock.Close() }()
	return retryStorageResult(ctx, fn)
}

func retryStorageOperation(ctx context.Context, fn func() error) error {
	for attempt := 0; ; attempt++ {
		err := fn()
		if !isSQLiteBusyError(err) {
			return err
		}
		if retryErr := waitForStorageRetry(ctx, attempt); retryErr != nil {
			return retryErr
		}
	}
}

func retryStorageResult(ctx context.Context, fn func() (driver.Result, error)) (driver.Result, error) {
	for attempt := 0; ; attempt++ {
		result, err := fn()
		if !isSQLiteBusyError(err) {
			return result, err
		}
		if retryErr := waitForStorageRetry(ctx, attempt); retryErr != nil {
			return nil, retryErr
		}
	}
}

func retryStorageRows(ctx context.Context, fn func() (driver.Rows, error)) (driver.Rows, error) {
	for attempt := 0; ; attempt++ {
		rows, err := fn()
		if !isSQLiteBusyError(err) {
			return rows, err
		}
		if retryErr := waitForStorageRetry(ctx, attempt); retryErr != nil {
			return nil, retryErr
		}
	}
}

func (c *storageConn) Prepare(query string) (driver.Stmt, error) {
	return c.prepare(context.Background(), query)
}

func (c *storageConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	return c.prepare(ctx, query)
}

func (c *storageConn) prepare(ctx context.Context, query string) (driver.Stmt, error) {
	var statement driver.Stmt
	err := c.withLock(ctx, func() error {
		var err error
		if preparer, ok := c.base.(driver.ConnPrepareContext); ok {
			statement, err = preparer.PrepareContext(ctx, query)
		} else {
			statement, err = c.base.Prepare(query)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return &storageStmt{base: statement, conn: c}, nil
}

func (c *storageConn) Close() error {
	c.endTransaction(nil)
	return c.base.Close()
}

func (c *storageConn) Begin() (driver.Tx, error) {
	return c.begin(context.Background(), driver.TxOptions{})
}

func (c *storageConn) BeginTx(ctx context.Context, options driver.TxOptions) (driver.Tx, error) {
	return c.begin(ctx, options)
}

func (c *storageConn) begin(ctx context.Context, options driver.TxOptions) (driver.Tx, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var lock *storageLock
	var err error
	if !hasStorageLock(ctx) && c.lockPath != "" {
		lock, err = acquireStorageLock(ctx, c.lockPath)
		if err != nil {
			return nil, err
		}
	}
	var transaction driver.Tx
	err = retryStorageOperation(ctx, func() error {
		if beginTx, ok := c.base.(driver.ConnBeginTx); ok {
			var err error
			transaction, err = beginTx.BeginTx(ctx, options)
			return err
		}
		if options != (driver.TxOptions{}) {
			return driver.ErrSkip
		}
		var err error
		transaction, err = c.base.Begin()
		return err
	})
	if err != nil {
		_ = lock.Close()
		return nil, err
	}
	c.setTransaction(lock)
	return &storageTx{base: transaction, conn: c, lock: lock}, nil
}

func (c *storageConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	execer, ok := c.base.(driver.Execer)
	if !ok {
		return nil, driver.ErrSkip
	}
	return c.withResultLock(context.Background(), func() (driver.Result, error) {
		return execer.Exec(query, args)
	})
}

func (c *storageConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	execer, ok := c.base.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	return c.withResultLock(ctx, func() (driver.Result, error) {
		return execer.ExecContext(ctx, query, args)
	})
}

func (c *storageConn) Query(query string, args []driver.Value) (driver.Rows, error) {
	queryer, ok := c.base.(driver.Queryer)
	if !ok {
		return nil, driver.ErrSkip
	}
	return c.queryRows(context.Background(), func() (driver.Rows, error) {
		return queryer.Query(query, args)
	})
}

func (c *storageConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	queryer, ok := c.base.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	return c.queryRows(ctx, func() (driver.Rows, error) {
		return queryer.QueryContext(ctx, query, args)
	})
}

func (c *storageConn) queryRows(ctx context.Context, fn func() (driver.Rows, error)) (driver.Rows, error) {
	if hasStorageLock(ctx) || c.isInTransaction() || c.lockPath == "" {
		return retryStorageRows(ctx, fn)
	}
	lock, err := acquireStorageLock(ctx, c.lockPath)
	if err != nil {
		return nil, err
	}
	rows, err := retryStorageRows(ctx, fn)
	if err != nil {
		_ = lock.Close()
		return nil, err
	}
	return &storageRows{Rows: rows, lock: lock}, nil
}

func (c *storageConn) Ping(ctx context.Context) error {
	pinger, ok := c.base.(driver.Pinger)
	if !ok {
		return nil
	}
	return c.withLock(ctx, func() error { return pinger.Ping(ctx) })
}

func (c *storageConn) ResetSession(ctx context.Context) error {
	resetter, ok := c.base.(driver.SessionResetter)
	if !ok {
		return nil
	}
	return c.withLock(ctx, func() error { return resetter.ResetSession(ctx) })
}

func (c *storageConn) IsValid() bool {
	if validator, ok := c.base.(driver.Validator); ok {
		return validator.IsValid()
	}
	return true
}

func (c *storageConn) CheckNamedValue(value *driver.NamedValue) error {
	if checker, ok := c.base.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(value)
	}
	converted, err := driver.DefaultParameterConverter.ConvertValue(value.Value)
	if err != nil {
		return err
	}
	value.Value = converted
	return nil
}

type storageTx struct {
	base driver.Tx
	conn *storageConn
	lock *storageLock
}

func (tx *storageTx) Commit() error {
	err := tx.base.Commit()
	tx.conn.endTransaction(tx.lock)
	if isSQLiteBusyError(err) {
		return &StorageBusyError{operation: "transaction commit"}
	}
	return err
}

func (tx *storageTx) Rollback() error {
	err := tx.base.Rollback()
	tx.conn.endTransaction(tx.lock)
	if isSQLiteBusyError(err) {
		return &StorageBusyError{operation: "transaction rollback"}
	}
	return err
}

type storageStmt struct {
	base driver.Stmt
	conn *storageConn
}

func (s *storageStmt) Close() error {
	return s.conn.withLock(context.Background(), s.base.Close)
}

func (s *storageStmt) NumInput() int { return s.base.NumInput() }

func (s *storageStmt) Exec(args []driver.Value) (driver.Result, error) {
	if _, ok := s.base.(driver.StmtExecContext); ok {
		named := make([]driver.NamedValue, len(args))
		for index, arg := range args {
			named[index] = driver.NamedValue{Ordinal: index + 1, Value: arg}
		}
		return s.ExecContext(context.Background(), named)
	}
	return s.conn.withResultLock(context.Background(), func() (driver.Result, error) {
		return s.base.Exec(args)
	})
}

func (s *storageStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	execer, ok := s.base.(driver.StmtExecContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	return s.conn.withResultLock(ctx, func() (driver.Result, error) {
		return execer.ExecContext(ctx, args)
	})
}

func (s *storageStmt) Query(args []driver.Value) (driver.Rows, error) {
	if _, ok := s.base.(driver.StmtQueryContext); ok {
		named := make([]driver.NamedValue, len(args))
		for index, arg := range args {
			named[index] = driver.NamedValue{Ordinal: index + 1, Value: arg}
		}
		return s.QueryContext(context.Background(), named)
	}
	return s.conn.queryRows(context.Background(), func() (driver.Rows, error) {
		return s.base.Query(args)
	})
}

func (s *storageStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	queryer, ok := s.base.(driver.StmtQueryContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	return s.conn.queryRows(ctx, func() (driver.Rows, error) {
		return queryer.QueryContext(ctx, args)
	})
}

type storageRows struct {
	driver.Rows
	lock *storageLock
	once sync.Once
}

func (r *storageRows) release() {
	r.once.Do(func() {
		if r.lock != nil {
			_ = r.lock.Close()
		}
	})
}

func (r *storageRows) Next(dest []driver.Value) error {
	err := r.Rows.Next(dest)
	if errors.Is(err, io.EOF) || err != nil {
		r.release()
	}
	return err
}

func (r *storageRows) Close() error {
	err := r.Rows.Close()
	r.release()
	return err
}

func (r *storageRows) HasNextResultSet() bool {
	if next, ok := r.Rows.(driver.RowsNextResultSet); ok {
		return next.HasNextResultSet()
	}
	return false
}

func (r *storageRows) NextResultSet() error {
	next, ok := r.Rows.(driver.RowsNextResultSet)
	if !ok {
		r.release()
		return io.EOF
	}
	err := next.NextResultSet()
	if errors.Is(err, io.EOF) || err != nil {
		r.release()
	}
	return err
}

func (r *storageRows) ColumnTypeDatabaseTypeName(index int) string {
	if columns, ok := r.Rows.(driver.RowsColumnTypeDatabaseTypeName); ok {
		return columns.ColumnTypeDatabaseTypeName(index)
	}
	return ""
}

func (r *storageRows) ColumnTypeLength(index int) (length int64, ok bool) {
	if columns, ok := r.Rows.(driver.RowsColumnTypeLength); ok {
		return columns.ColumnTypeLength(index)
	}
	return 0, false
}

func (r *storageRows) ColumnTypeNullable(index int) (nullable, ok bool) {
	if columns, ok := r.Rows.(driver.RowsColumnTypeNullable); ok {
		return columns.ColumnTypeNullable(index)
	}
	return false, false
}

func (r *storageRows) ColumnTypePrecisionScale(index int) (precision, scale int64, ok bool) {
	if columns, ok := r.Rows.(driver.RowsColumnTypePrecisionScale); ok {
		return columns.ColumnTypePrecisionScale(index)
	}
	return 0, 0, false
}

func (r *storageRows) ColumnTypeScanType(index int) reflect.Type {
	if columns, ok := r.Rows.(driver.RowsColumnTypeScanType); ok {
		return columns.ColumnTypeScanType(index)
	}
	return nil
}
