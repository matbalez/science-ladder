// Package signing adapts separately managed P-256 keys to protocol signing.
package signing

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
)

// FromPEM deliberately refuses raw application keys in official deployments.
func FromPEM(data []byte, deploymentMode string) (crypto.Signer, error) {
	if deploymentMode != "local" && deploymentMode != "controlled-demo" {
		return nil, errors.New("raw receipt keys are restricted to local or controlled-demo deployments")
	}
	block, rest := pem.Decode(data)
	if block == nil || strings.TrimSpace(string(rest)) != "" {
		return nil, errors.New("exactly one PEM private key is required")
	}
	var key any
	var err error
	switch block.Type {
	case "EC PRIVATE KEY":
		key, err = x509.ParseECPrivateKey(block.Bytes)
	case "PRIVATE KEY":
		key, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	default:
		return nil, errors.New("unsupported private key encoding")
	}
	if err != nil {
		return nil, errors.New("invalid private key")
	}
	p256, ok := key.(*ecdsa.PrivateKey)
	if !ok || p256.Curve != elliptic.P256() {
		return nil, errors.New("ECDSA P-256 private key required")
	}
	return p256, nil
}

type kmsAPI interface {
	GetPublicKey(context.Context, *kms.GetPublicKeyInput, ...func(*kms.Options)) (*kms.GetPublicKeyOutput, error)
	Sign(context.Context, *kms.SignInput, ...func(*kms.Options)) (*kms.SignOutput, error)
}

type remoteSigner struct {
	client kmsAPI
	keyARN string
	public *ecdsa.PublicKey
}

// NewAWS never sends or retrieves a private key. Dedicated signing credentials
// avoid confusing the S3-compatible artifact credentials with AWS credentials.
// With no static credentials, AWS profiles/workload identities are supported.
func NewAWS(ctx context.Context, region, keyID string) (crypto.Signer, error) {
	if region == "" || keyID == "" {
		return nil, errors.New("receipt KMS region and key ID are required")
	}
	opts := []func(*config.LoadOptions) error{config.WithRegion(region)}
	access, secret := os.Getenv("SIGNING_AWS_ACCESS_KEY_ID"), os.Getenv("SIGNING_AWS_SECRET_ACCESS_KEY")
	if access != "" || secret != "" {
		if access == "" || secret == "" {
			return nil, errors.New("incomplete dedicated signing credentials")
		}
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(access, secret, os.Getenv("SIGNING_AWS_SESSION_TOKEN"))))
	} else if os.Getenv("AWS_ACCESS_KEY_ID") != "" || os.Getenv("AWS_SECRET_ACCESS_KEY") != "" {
		return nil, errors.New("use separate SIGNING_AWS credentials; artifact-store credentials cannot authorize receipt signing")
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, errors.New("load signing workload credentials")
	}
	// Do not inherit S3 endpoint overrides for a signing service.
	client := kms.NewFromConfig(cfg, func(o *kms.Options) { o.BaseEndpoint = nil })
	return newRemote(ctx, client, keyID)
}

func newRemote(ctx context.Context, client kmsAPI, keyID string) (crypto.Signer, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	response, err := client.GetPublicKey(ctx, &kms.GetPublicKeyInput{KeyId: aws.String(keyID)})
	if err != nil {
		return nil, errors.New("retrieve receipt KMS public key")
	}
	if response == nil || response.KeyId == nil || !strings.Contains(*response.KeyId, ":key/") || response.KeySpec != types.KeySpecEccNistP256 || response.KeyUsage != types.KeyUsageTypeSignVerify {
		return nil, errors.New("receipt KMS key must be a P-256 SIGN_VERIFY key with an immutable ARN")
	}
	supported := false
	for _, algorithm := range response.SigningAlgorithms {
		supported = supported || algorithm == types.SigningAlgorithmSpecEcdsaSha256
	}
	public, err := x509.ParsePKIXPublicKey(response.PublicKey)
	p256, ok := public.(*ecdsa.PublicKey)
	if err != nil || !ok || p256.Curve != elliptic.P256() || !supported {
		return nil, errors.New("receipt KMS public key is not compatible with ECDSA P-256/SHA-256")
	}
	return &remoteSigner{client: client, keyARN: *response.KeyId, public: p256}, nil
}

func (s *remoteSigner) Public() crypto.PublicKey { return s.public }

func (s *remoteSigner) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	if opts == nil || opts.HashFunc() != crypto.SHA256 || len(digest) != 32 {
		return nil, errors.New("receipt KMS signer accepts only SHA-256 digests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	r, err := s.client.Sign(ctx, &kms.SignInput{
		KeyId: aws.String(s.keyARN), Message: digest, MessageType: types.MessageTypeDigest,
		SigningAlgorithm: types.SigningAlgorithmSpecEcdsaSha256,
	})
	if err != nil {
		return nil, errors.New("KMS signing request failed; the committed receipt remains pending")
	}
	if r == nil || r.KeyId == nil || *r.KeyId != s.keyARN || r.SigningAlgorithm != types.SigningAlgorithmSpecEcdsaSha256 || !ecdsa.VerifyASN1(s.public, digest, r.Signature) {
		return nil, errors.New("KMS returned a signature with the wrong key, algorithm, or digest")
	}
	return r.Signature, nil
}
