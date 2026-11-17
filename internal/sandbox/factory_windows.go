//go:build windows

package sandbox

// NewDefaultRunner returns the native Windows Job Object isolation runner.
func NewDefaultRunner() Runner {
	return NewWindowsJobRunner()
}
