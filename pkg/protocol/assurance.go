package protocol

const VerificationPlatform = "platform"
const VerificationIndependent = "independent"
const StatusPlatformVerified = "platform_verified"
const StatusIndependentlyReplicated = "independently_replicated"

func ValidVerificationPolicy(policy string) bool {
	return policy == VerificationPlatform || policy == VerificationIndependent
}

// ManifestVerificationPolicy resolves the authoring default for a NEW lock.
func ManifestVerificationPolicy(m Manifest) string {
	if m.VerificationPolicy == "" {
		return VerificationPlatform
	}
	return m.VerificationPolicy
}

// LockVerificationPolicy preserves the earlier two-host contract for historical
// locks that predate the explicit policy field. Existing locks never silently
// acquire the new single-host default.
func LockVerificationPolicy(lock Lock) string {
	if lock.VerificationPolicy == "" {
		return VerificationIndependent
	}
	return lock.VerificationPolicy
}

func JobVerificationPolicy(job RunnerJob) string {
	if job.VerificationPolicy == "" {
		return VerificationIndependent
	}
	return job.VerificationPolicy
}
