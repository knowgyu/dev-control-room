//go:build windows

package scheduler

// NewAdapter selects the native Task Scheduler implementation on Windows.
func NewAdapter() Adapter { return &NativeAdapter{} }
