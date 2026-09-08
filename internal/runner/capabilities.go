package runner

import (
	"errors"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

func validateCapabilitiesBinding(c Config, a HostAttestation) error {
	if c.Capabilities == nil && a.Capabilities == nil {
		return nil
	}
	if c.Capabilities == nil || a.Capabilities == nil {
		return errors.New("executor capabilities lack signed enrollment")
	}
	one, err := protocol.Digest(c.Capabilities)
	if err != nil {
		return err
	}
	two, err := protocol.Digest(a.Capabilities)
	if err != nil || one != two {
		return errors.New("configured capabilities differ from signed enrollment")
	}
	if c.Capabilities.RuntimeImageDigest != c.RuntimeImageDigest || c.Capabilities.OS != "linux" || c.Capabilities.Architecture != "amd64" || c.Capabilities.Accelerator != "none" {
		return errors.New("this Firecracker backend cannot implement the enrolled capabilities")
	}
	return nil
}

func matchConfiguredEvaluation(m protocol.Manifest, c Config) error {
	if m.Evaluation == nil {
		return nil
	}
	if c.Capabilities == nil {
		return errors.New("v2 evaluation requires explicitly enrolled executor capabilities")
	}
	return protocol.MatchExecutor(*m.Evaluation, m.Resources, m.Validator.RuntimeImageDigest, *c.Capabilities)
}
