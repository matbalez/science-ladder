// Package secrets wraps small secret material, such as hidden-suite data keys.
// Suite files themselves use a separate randomly generated encryption key.
package secrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
)

type Sealer interface {
	Seal(context.Context, []byte, []byte) ([]byte, error)
	Open(context.Context, []byte, []byte) ([]byte, error)
}

const maxPlaintext = 4096

func validInput(ctx context.Context, aad []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(aad) == 0 || len(aad) > 4096 {
		return errors.New("secret context is required and must be at most 4096 bytes")
	}
	return nil
}

type localSealer struct{ aead cipher.AEAD }

// NewLocal is deliberately unavailable in an official deployment.
func NewLocal(key []byte, deploymentMode string) (Sealer, error) {
	if deploymentMode != "local" && deploymentMode != "controlled-demo" {
		return nil, errors.New("raw wrapping keys are restricted to local or controlled-demo deployments")
	}
	if len(key) != 32 {
		return nil, errors.New("secret wrapping requires a 32-byte key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &localSealer{aead: aead}, nil
}

func (s *localSealer) Seal(ctx context.Context, plaintext, aad []byte) ([]byte, error) {
	if err := validInput(ctx, aad); err != nil {
		return nil, err
	}
	if len(plaintext) == 0 || len(plaintext) > maxPlaintext {
		return nil, errors.New("secret material must contain 1 to 4096 bytes")
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	// Version byte and nonce are authenticated along with caller context.
	prefix := append([]byte{1}, nonce...)
	binding := append(append([]byte{}, aad...), prefix...)
	return s.aead.Seal(prefix, nonce, plaintext, binding), nil
}

func (s *localSealer) Open(ctx context.Context, ciphertext, aad []byte) ([]byte, error) {
	if err := validInput(ctx, aad); err != nil {
		return nil, err
	}
	n := 1 + s.aead.NonceSize()
	if len(ciphertext) <= n+s.aead.Overhead() || len(ciphertext) > n+s.aead.Overhead()+maxPlaintext || ciphertext[0] != 1 {
		return nil, errors.New("invalid wrapped secret")
	}
	binding := append(append([]byte{}, aad...), ciphertext[:n]...)
	plain, err := s.aead.Open(nil, ciphertext[1:n], ciphertext[n:], binding)
	if err != nil {
		return nil, errors.New("wrapped secret authentication failed")
	}
	return plain, nil
}

type kmsAPI interface {
	DescribeKey(context.Context, *kms.DescribeKeyInput, ...func(*kms.Options)) (*kms.DescribeKeyOutput, error)
	Encrypt(context.Context, *kms.EncryptInput, ...func(*kms.Options)) (*kms.EncryptOutput, error)
	Decrypt(context.Context, *kms.DecryptInput, ...func(*kms.Options)) (*kms.DecryptOutput, error)
}
type remoteSealer struct {
	client kmsAPI
	keyARN string
}

// NewAWS resolves aliases once and pins the immutable key ARN. Artifact-store
// credentials are intentionally not accepted as KMS credentials.
func NewAWS(ctx context.Context, region, keyID string) (Sealer, error) {
	if region == "" || keyID == "" {
		return nil, errors.New("hidden suite KMS region and key ID are required")
	}
	opts := []func(*config.LoadOptions) error{config.WithRegion(region)}
	access, secret := os.Getenv("HIDDEN_SUITE_AWS_ACCESS_KEY_ID"), os.Getenv("HIDDEN_SUITE_AWS_SECRET_ACCESS_KEY")
	if access != "" || secret != "" {
		if access == "" || secret == "" {
			return nil, errors.New("incomplete dedicated hidden suite credentials")
		}
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(access, secret, os.Getenv("HIDDEN_SUITE_AWS_SESSION_TOKEN"))))
	} else if os.Getenv("AWS_ACCESS_KEY_ID") != "" || os.Getenv("AWS_SECRET_ACCESS_KEY") != "" {
		return nil, errors.New("use separate HIDDEN_SUITE_AWS credentials; artifact-store credentials cannot authorize secret wrapping")
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, errors.New("load hidden suite workload credentials")
	}
	client := kms.NewFromConfig(cfg, func(o *kms.Options) { o.BaseEndpoint = nil })
	return newRemote(ctx, client, keyID)
}

func newRemote(ctx context.Context, client kmsAPI, keyID string) (Sealer, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	r, err := client.DescribeKey(ctx, &kms.DescribeKeyInput{KeyId: aws.String(keyID)})
	if err != nil {
		return nil, errors.New("inspect hidden suite KMS key")
	}
	if r == nil || r.KeyMetadata == nil {
		return nil, errors.New("missing hidden suite KMS key metadata")
	}
	m := r.KeyMetadata
	if m.Arn == nil || !strings.HasPrefix(*m.Arn, "arn:") || !strings.Contains(*m.Arn, ":key/") || m.KeySpec != types.KeySpecSymmetricDefault || m.KeyUsage != types.KeyUsageTypeEncryptDecrypt || m.KeyState != types.KeyStateEnabled || !m.Enabled {
		return nil, errors.New("hidden suite KMS key must be an enabled symmetric ENCRYPT_DECRYPT key with immutable ARN")
	}
	return &remoteSealer{client: client, keyARN: *m.Arn}, nil
}

func encryptionContext(aad []byte) map[string]string {
	digest := sha256.Sum256(aad)
	return map[string]string{"purpose": "science-ladder-hidden-suite-material-v1", "context-sha256": hex.EncodeToString(digest[:])}
}

func (s *remoteSealer) Seal(ctx context.Context, plaintext, aad []byte) ([]byte, error) {
	if err := validInput(ctx, aad); err != nil {
		return nil, err
	}
	if len(plaintext) == 0 || len(plaintext) > maxPlaintext {
		return nil, errors.New("secret material must contain 1 to 4096 bytes")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	r, err := s.client.Encrypt(ctx, &kms.EncryptInput{KeyId: aws.String(s.keyARN), Plaintext: plaintext, EncryptionAlgorithm: types.EncryptionAlgorithmSpecSymmetricDefault, EncryptionContext: encryptionContext(aad)})
	if err != nil {
		return nil, errors.New("hidden suite KMS wrapping failed")
	}
	if r == nil || r.KeyId == nil || *r.KeyId != s.keyARN || r.EncryptionAlgorithm != types.EncryptionAlgorithmSpecSymmetricDefault || len(r.CiphertextBlob) == 0 || len(r.CiphertextBlob) > 6144 {
		return nil, errors.New("invalid KMS wrapping response")
	}
	return r.CiphertextBlob, nil
}

func (s *remoteSealer) Open(ctx context.Context, ciphertext, aad []byte) ([]byte, error) {
	if err := validInput(ctx, aad); err != nil {
		return nil, err
	}
	if len(ciphertext) == 0 || len(ciphertext) > 6144 {
		return nil, errors.New("invalid wrapped secret")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	r, err := s.client.Decrypt(ctx, &kms.DecryptInput{KeyId: aws.String(s.keyARN), CiphertextBlob: ciphertext, EncryptionAlgorithm: types.EncryptionAlgorithmSpecSymmetricDefault, EncryptionContext: encryptionContext(aad)})
	if err != nil {
		return nil, errors.New("hidden suite KMS unwrapping failed")
	}
	if r == nil {
		return nil, errors.New("invalid KMS unwrapping response")
	}
	if r.KeyId == nil || *r.KeyId != s.keyARN || r.EncryptionAlgorithm != types.EncryptionAlgorithmSpecSymmetricDefault || len(r.Plaintext) == 0 || len(r.Plaintext) > maxPlaintext {
		clear(r.Plaintext)
		return nil, errors.New("invalid KMS unwrapping response")
	}
	return r.Plaintext, nil
}
