//go:build linux

package profileimage

import "golang.org/x/sys/unix"

func limitWorkerMemory() error {
	// Bound the isolated child's address space; an allocation failure kills only
	// the worker. Go's soft heap limit alone is not an adversarial memory boundary.
	if err := unix.Setrlimit(unix.RLIMIT_AS, &unix.Rlimit{Cur: 1536 << 20, Max: 1536 << 20}); err != nil {
		return err
	}
	// Keep writable mappings below the API container's 1Gi budget, leaving room
	// for the parent server. Virtual-address and Go soft limits are insufficient.
	return unix.Setrlimit(unix.RLIMIT_DATA, &unix.Rlimit{Cur: 384 << 20, Max: 384 << 20})
}
