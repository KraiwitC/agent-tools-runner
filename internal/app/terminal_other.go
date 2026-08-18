//go:build !darwin && !linux

package app

func enableNonCanonicalInput() (func(), bool, error) {
	return func() {}, false, nil
}
