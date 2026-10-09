//go:build !linux

package profileimage

import "errors"

func limitWorkerMemory() error {
	return errors.New("profile image worker requires Linux resource limits")
}
