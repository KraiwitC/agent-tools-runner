//go:build !darwin && !linux && !windows

package app

func enableNonCanonicalInput() (func(), bool, error) {
	return func() {}, false, nil
}
