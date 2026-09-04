package signing

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/matbalez/science-ladder/pkg/protocol"
)

type fakeKMS struct {
	key  *ecdsa.PrivateKey
	swap bool
	t    *testing.T
}

const testARN = "arn:aws:kms:us-east-1:000000000000:key/00000000-0000-0000-0000-000000000000"

func (f *fakeKMS) GetPublicKey(context.Context, *kms.GetPublicKeyInput, ...func(*kms.Options)) (*kms.GetPublicKeyOutput, error) {
	b, _ := x509.MarshalPKIXPublicKey(&f.key.PublicKey)
	return &kms.GetPublicKeyOutput{KeyId: aws.String(testARN), PublicKey: b, KeySpec: types.KeySpecEccNistP256, KeyUsage: types.KeyUsageTypeSignVerify, SigningAlgorithms: []types.SigningAlgorithmSpec{types.SigningAlgorithmSpecEcdsaSha256}}, nil
}
func (f *fakeKMS) Sign(_ context.Context, in *kms.SignInput, _ ...func(*kms.Options)) (*kms.SignOutput, error) {
	if *in.KeyId != testARN || in.MessageType != types.MessageTypeDigest || len(in.Message) != 32 || in.SigningAlgorithm != types.SigningAlgorithmSpecEcdsaSha256 {
		f.t.Fatal("KMS request violated pinned key/digest contract")
	}
	key := f.key
	if f.swap {
		key, _ = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	}
	sig, _ := ecdsa.SignASN1(rand.Reader, key, in.Message)
	return &kms.SignOutput{KeyId: aws.String(testARN), SigningAlgorithm: types.SigningAlgorithmSpecEcdsaSha256, Signature: sig}, nil
}
func TestKMSSignerDSSEAndRotationRejection(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	f := &fakeKMS{key: key, t: t}
	s, err := newRemote(context.Background(), f, "alias/receipt")
	if err != nil {
		t.Fatal(err)
	}
	env, err := protocol.Sign("receipt-v1", s, map[string]string{"kind": "receipt"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = protocol.Verify(env, map[string]crypto.PublicKey{"receipt-v1": s.Public()}); err != nil {
		t.Fatal(err)
	}
	f.swap = true
	if _, err = protocol.Sign("receipt-v1", s, map[string]string{"kind": "receipt"}); err == nil {
		t.Fatal("accepted signature from a different key")
	}
	if _, err = s.Sign(rand.Reader, make([]byte, 32), crypto.SHA512); err == nil {
		t.Fatal("accepted wrong hash algorithm")
	}
}
func TestRawKeyCannotEnableOfficialSigning(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, _ := x509.MarshalPKCS8PrivateKey(key)
	b := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	for _, mode := range []string{"", "production", "official"} {
		if _, err := FromPEM(b, mode); err == nil {
			t.Fatalf("accepted raw key in %q", mode)
		}
	}
	for _, mode := range []string{"local", "controlled-demo"} {
		if _, err := FromPEM(b, mode); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := FromPEM(append(b, b...), "local"); err == nil {
		t.Fatal("accepted multiple keys")
	}
}
