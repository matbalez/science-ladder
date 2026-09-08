package runner

import (
	"testing"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

func TestBrokerRejectsStageInjectionAndBudgetBypass(t *testing.T) {
	p := protocol.CandidateProgram{MaxRuns: 2}
	for _, tc := range []struct {
		request BrokerRequest
		built   bool
		runs    int
	}{{BrokerRequest{Action: "/bin/sh"}, false, 0}, {BrokerRequest{Action: "run"}, false, 0}, {BrokerRequest{Action: "build"}, true, 0}, {BrokerRequest{Action: "build", Input: []byte("hidden test")}, false, 0}, {BrokerRequest{Action: "run"}, true, 2}, {BrokerRequest{Action: "run", Input: make([]byte, maxBrokerInput+1)}, true, 0}} {
		if validateBrokerRequest(tc.request, tc.built, tc.runs, p) == nil {
			t.Fatal("broker stage or budget bypass accepted")
		}
	}
	if err := validateBrokerRequest(BrokerRequest{Action: "build"}, false, 0, p); err != nil {
		t.Fatal(err)
	}
	if err := validateBrokerRequest(BrokerRequest{Action: "run", Input: []byte("case")}, true, 1, p); err != nil {
		t.Fatal(err)
	}
}

func TestNativeConformanceManifestIsExplicitlyVersioned(t *testing.T) {
	m := nativeProbeManifest(protocol.DigestBytes([]byte("native runtime")), "probe.c", []string{"/usr/bin/gcc", "probe.c", "-o", "/work/probe"})
	if err := protocol.ValidateManifest(m); err != nil {
		t.Fatal(err)
	}
	if _, _, err := protocol.ArtifactFromFiles(map[string][]byte{"probe.c": []byte("int main(){return 0;}\n")}, m.Submission); err != nil {
		t.Fatal(err)
	}
	m.APIVersion = protocol.APIVersion
	if protocol.ValidateManifest(m) == nil {
		t.Fatal("native code silently enabled in a legacy manifest")
	}
	legacy := protocol.SubmissionContract{AllowedPaths: []string{"data/"}, AllowedExtensions: []string{".txt"}, MaxBytes: 1024, MaxFiles: 1, License: "MIT"}
	if _, _, err := protocol.ArtifactFromFiles(map[string][]byte{"data/probe.txt": []byte("#!/bin/sh\necho injected")}, legacy); err == nil {
		t.Fatal("legacy active-content restrictions weakened")
	}
}
