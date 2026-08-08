//go:build !darwin

package app

func enableNonCanonicalInput() (func(), bool, error) {
	return func() {}, false, nil
}
