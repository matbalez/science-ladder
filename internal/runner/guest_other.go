//go:build !linux

package runner

import "errors"

func GuestInit() error { return errors.New("guest init requires Linux") }
