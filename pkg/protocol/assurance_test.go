package protocol

import "testing"

func TestVerificationPolicyDefaultsPreserveHistoricLocks(t *testing.T) {
	if ManifestVerificationPolicy(Manifest{}) != VerificationPlatform {
		t.Fatal("new manifest default must be platform")
	}
	if LockVerificationPolicy(Lock{}) != VerificationIndependent {
		t.Fatal("historic two-host lock silently weakened")
	}
	for _, policy := range []string{VerificationPlatform, VerificationIndependent} {
		if !ValidVerificationPolicy(policy) || ManifestVerificationPolicy(Manifest{VerificationPolicy: policy}) != policy || LockVerificationPolicy(Lock{VerificationPolicy: policy}) != policy {
			t.Fatal(policy)
		}
	}
	for _, policy := range []string{"", "verified", "same-host-independent"} {
		if ValidVerificationPolicy(policy) {
			t.Fatal("unknown policy admitted")
		}
	}
}
