package runner

import (
	"archive/zip"
	"bytes"
	"crypto"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

func TestOfficialHostFailsClosed(t *testing.T) {
	if err := (Config{}).CheckHost(map[string]crypto.PublicKey{}); err == nil {
		t.Fatal("unconfigured host admitted")
	}
}

func TestBoundedGuestFrames(t *testing.T) {
	valid := []byte(`{"apiVersion":"science-ladder/v1","kind":"ValidatorResult","score":"1","gates":{"valid":true}}`)
	frame := "SL_RESULT " + base64.StdEncoding.EncodeToString(valid)
	data, outcome, err := parseGuestOutput([]byte("kernel boot\n" + frame + "\n"))
	if err != nil || !bytes.Equal(data, valid) || outcome != "" {
		t.Fatalf("%s %s %v", data, outcome, err)
	}
	for _, input := range []string{frame + "\n" + frame, frame + "\nSL_ERROR resource_limit", "SL_ERROR invented_category", "SL_RESULT %%%%", "arbitrary checker stdout", "SL_RESULT " + strings.Repeat("x", 100001)} {
		if _, _, err := parseGuestOutput([]byte(input)); err == nil {
			t.Errorf("accepted invalid frame %q", input[:min(len(input), 80)])
		}
	}
	_, outcome, err = parseGuestOutput([]byte("SL_ERROR resource_limit\n"))
	if err != nil || outcome != "resource_limit" {
		t.Fatal("lost bounded resource outcome")
	}
}

func TestSourceSnapshotRejectsHostileFiles(t *testing.T) {
	for _, files := range []map[string][]byte{{"../escape.py": nil}, {".GIT/config": nil}, {"safe.py": []byte("-----BEGIN PRIVATE KEY-----")}, {".env": []byte("TOKEN=x")}, {"Dockerfile": []byte("FROM ubuntu")}, {"checker.sh": nil}, {"CHECKER.SH": nil}, {"data.txt": []byte{0x7f, 'E', 'L', 'F'}}, {"native.DYLIB": nil}, {"A.py": nil, "a.py": nil}} {
		snapshot := SourceSnapshot{RepositoryID: 1, SourceCommit: strings.Repeat("a", 40), Files: files}
		data, _ := json.Marshal(snapshot)
		if _, err := ReadSourceSnapshot(data); err == nil {
			t.Errorf("accepted %+v", files)
		}
	}
	snapshot := SourceSnapshot{RepositoryID: 1, SourceCommit: strings.Repeat("a", 40), Files: map[string][]byte{"checker.py": []byte("# fixed direct checker"), "suite/large.json": bytes.Repeat([]byte(" "), 300000)}}
	data, _ := json.Marshal(snapshot)
	if _, err := ReadSourceSnapshot(data); err != nil {
		t.Fatal(err)
	}
}

func TestWheelLocksAndZipTraversal(t *testing.T) {
	if _, err := lockedWheelFiles(map[string][]byte{"requirements.lock": []byte("package>=1")}, "requirements.lock"); err == nil {
		t.Fatal("unlocked dependency accepted")
	}
	if _, err := lockedWheelFiles(map[string][]byte{"requirements.lock": []byte("thing==1 --hash=sha256:" + strings.Repeat("a", 64))}, "requirements.lock"); err == nil {
		t.Fatal("missing wheel accepted")
	}
	for _, filename := range []string{"../escape.py", "package/hook.pth", "package/native.so", "package/native.DLL", "package.data/purelib/a.py"} {
		var data bytes.Buffer
		writer := zip.NewWriter(&data)
		entry, err := writer.Create(filename)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = entry.Write([]byte("x"))
		_ = writer.Close()
		wheel := data.Bytes()
		lock := "thing==1 --hash=" + protocol.DigestBytes(wheel)
		files := map[string][]byte{"requirements.lock": []byte(lock), "wheels/thing-1-py3-none-any.whl": wheel}
		if _, err := lockedWheelFiles(files, "requirements.lock"); err == nil {
			t.Errorf("accepted wheel path %s", filename)
		}
	}
	if files, err := lockedWheelFiles(map[string][]byte{"requirements.lock": []byte("# stdlib only")}, "requirements.lock"); err != nil || len(files) != 0 {
		t.Fatal("standard-library recipe rejected")
	}
}

func TestJobRejectsExpiredAndWrongHost(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	config := Config{ExecutionProfileDigest: digest, RunnerEpoch: "epoch1", HostID: "host-a", HostGroup: "zone-a"}
	job := protocol.RunnerJob{APIVersion: protocol.APIVersion, Kind: "ValidationJob", ID: "test-job", CreatedAt: time.Now().Add(-time.Minute), ExpiresAt: time.Now().Add(-time.Second), Purpose: "submission", ExecutionProfileDigest: digest, RunnerEpoch: "epoch1", FencingToken: 1}
	if ValidateJob(job, config) == nil {
		t.Fatal("expired job accepted")
	}
	job.ExpiresAt = time.Now().Add(time.Minute)
	job.ExcludedHostIDs = []string{"host-a"}
	if ValidateJob(job, config) == nil {
		t.Fatal("anti-affinity violation accepted")
	}
	job.ExcludedHostIDs = nil
	job.RequiredHostGroup = "other-zone"
	if ValidateJob(job, config) == nil {
		t.Fatal("wrong host group accepted")
	}
}

func TestBufferFloodNeverAllocatesBeyondCap(t *testing.T) {
	buffer := &boundedBuffer{max: 64}
	data := bytes.Repeat([]byte("x"), 4096)
	n, err := buffer.Write(data)
	if err != nil || n != len(data) || buffer.b.Len() != 64 || !buffer.overflow {
		t.Fatal("flood was not bounded")
	}
	_, _ = buffer.Write(data)
	if buffer.b.Len() != 64 {
		t.Fatal("continued flood grew buffer")
	}
}
