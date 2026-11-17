//go:build !windows && !linux

package sandbox

// NewDefaultRunner returns the cross-platform direct process runner.
func NewDefaultRunner() Runner {
	return NewDirectRunner()
}
