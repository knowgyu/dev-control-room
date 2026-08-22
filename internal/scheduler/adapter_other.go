//go:build !windows

package scheduler

// NewAdapter keeps non-Windows development and tests side-effect free.
func NewAdapter() Adapter { return &FakeAdapter{} }
