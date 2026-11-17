//go:build linux

package sandbox

// NewDefaultRunner returns the native Linux namespace and cgroup isolation runner.
func NewDefaultRunner() Runner {
	return NewLinuxNamespaceRunner()
}
