package secrets

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
)

func TestLocalAuthenticationAndModes(t *testing.T) {
	ctx := context.Background()
	key := bytes.Repeat([]byte{7}, 32)
	for _, mode := range []string{"production", "", "typo"} {
		if _, err := NewLocal(key, mode); err == nil {
			t.Fatalf("accepted mode %q", mode)
		}
	}
	s, err := NewLocal(key, "controlled-demo")
	if err != nil {
		t.Fatal(err)
	}
	plain, aad := []byte("suite material"), []byte("suite-one")
	sealed, err := s.Seal(ctx, plain, aad)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := s.Seal(ctx, plain, aad)
	if bytes.Equal(sealed, again) {
		t.Fatal("nonce reuse")
	}
	got, err := s.Open(ctx, sealed, aad)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatal("round trip", err)
	}
	for i := range sealed {
		bad := bytes.Clone(sealed)
		bad[i] ^= 1
		if p, e := s.Open(ctx, bad, aad); e == nil || len(p) > 0 {
			t.Fatalf("tamper accepted at%d", i)
		}
	}
	if p, e := s.Open(ctx, sealed, []byte("suite-two")); e == nil || len(p) > 0 {
		t.Fatal("cross-suite substitution accepted")
	}
	if _, e := s.Seal(ctx, plain, nil); e == nil {
		t.Fatal("empty context accepted")
	}
	if _, e := s.Seal(ctx, make([]byte, 4097), aad); e == nil {
		t.Fatal("oversized material accepted")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := s.Open(canceled, sealed, aad); e == nil {
		t.Fatal("ignored cancellation")
	}
}

const testARN = "arn:aws:kms:us-west-2:123456789012:key/abc"

type fakeKMS struct {
	context      map[string]string
	plain        []byte
	key          string
	badKey       bool
	badAlgorithm bool
	disabled     bool
	calls        int
	returned     []byte
}

func (f *fakeKMS) DescribeKey(_ context.Context, in *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
	f.key = *in.KeyId
	return &kms.DescribeKeyOutput{KeyMetadata: &types.KeyMetadata{Arn: aws.String(testARN), KeySpec: types.KeySpecSymmetricDefault, KeyUsage: types.KeyUsageTypeEncryptDecrypt, KeyState: types.KeyStateEnabled, Enabled: !f.disabled}}, nil
}
func (f *fakeKMS) Encrypt(_ context.Context, in *kms.EncryptInput, _ ...func(*kms.Options)) (*kms.EncryptOutput, error) {
	f.calls++
	if *in.KeyId != testARN {
		return nil, errors.New("alias not pinned")
	}
	f.context = in.EncryptionContext
	f.plain = bytes.Clone(in.Plaintext)
	key := testARN
	if f.badKey {
		key += "-other"
	}
	alg := types.EncryptionAlgorithmSpecSymmetricDefault
	if f.badAlgorithm {
		alg = types.EncryptionAlgorithmSpecRsaesOaepSha256
	}
	return &kms.EncryptOutput{KeyId: aws.String(key), CiphertextBlob: []byte("opaque-kms-ciphertext"), EncryptionAlgorithm: alg}, nil
}
func (f *fakeKMS) Decrypt(_ context.Context, in *kms.DecryptInput, _ ...func(*kms.Options)) (*kms.DecryptOutput, error) {
	f.calls++
	if *in.KeyId != testARN || !reflect.DeepEqual(f.context, in.EncryptionContext) {
		return nil, errors.New("context mismatch")
	}
	key := testARN
	if f.badKey {
		key += "-other"
	}
	alg := types.EncryptionAlgorithmSpecSymmetricDefault
	if f.badAlgorithm {
		alg = types.EncryptionAlgorithmSpecRsaesOaepSha256
	}
	f.returned = bytes.Clone(f.plain)
	return &kms.DecryptOutput{KeyId: aws.String(key), Plaintext: f.returned, EncryptionAlgorithm: alg}, nil
}
func TestKMSPinsKeyAndBindsContext(t *testing.T) {
	ctx := context.Background()
	f := &fakeKMS{}
	s, err := newRemote(ctx, f, "alias/science-ladder")
	if err != nil {
		t.Fatal(err)
	}
	plain, aad := []byte("secret"), []byte("suite-id")
	sealed, err := s.Seal(ctx, plain, aad)
	if err != nil {
		t.Fatal(err)
	}
	if f.context["purpose"] != "science-ladder-hidden-suite-material-v1" || len(f.context["context-sha256"]) != 64 {
		t.Fatal("missing context binding")
	}
	got, err := s.Open(ctx, sealed, aad)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatal("roundtrip", err)
	}
	if got, err = s.Open(ctx, sealed, []byte("other-suite")); err == nil || len(got) > 0 {
		t.Fatal("wrong context accepted")
	}
	f.badKey = true
	if got, err = s.Open(ctx, sealed, aad); err == nil || len(got) > 0 {
		t.Fatal("wrong key accepted")
	}
	if !bytes.Equal(f.returned, make([]byte, len(plain))) {
		t.Fatal("bad-response plaintext not erased")
	}
	if got, err = s.Seal(ctx, plain, aad); err == nil || len(got) > 0 {
		t.Fatal("wrong encrypt key accepted")
	}
	f.badKey = false
	f.badAlgorithm = true
	if _, err = s.Open(ctx, sealed, aad); err == nil {
		t.Fatal("wrong algorithm accepted")
	}
	f.disabled = true
	if _, err = newRemote(ctx, f, "alias/foo"); err == nil {
		t.Fatal("disabled key accepted")
	}
}

func TestKMSRejectsArtifactCredentials(t *testing.T) {
	t.Setenv("HIDDEN_SUITE_AWS_ACCESS_KEY_ID", "")
	t.Setenv("HIDDEN_SUITE_AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "artifact-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "artifact-secret")
	if _, err := NewAWS(context.Background(), "us-west-2", "alias/suite"); err == nil {
		t.Fatal("artifact credentials accepted")
	}
}
