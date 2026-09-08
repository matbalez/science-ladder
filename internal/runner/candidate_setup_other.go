//go:build !linux

package runner

import "errors"

func CandidateSetup([]string) error { return errors.New("candidate namespace setup requires Linux") }
