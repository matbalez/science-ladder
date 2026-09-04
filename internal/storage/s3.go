// Package storage provides immutable content-addressed object storage.
package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"io"
	"regexp"
	"time"
)

var digestRE = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

type Store struct {
	client *s3.Client
	bucket string
}

func New(ctx context.Context, bucket, region, endpoint string) (*Store, error) {
	if bucket == "" {
		return nil, errors.New("S3_BUCKET is required")
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		}
	})
	return &Store{client, bucket}, nil
}
func Digest(b []byte) string { sum := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(sum[:]) }
func key(digest string) (string, error) {
	if !digestRE.MatchString(digest) {
		return "", errors.New("invalid content digest")
	}
	return "cas/sha256/" + digest[7:9] + "/" + digest[7:], nil
}
func (s *Store) Put(ctx context.Context, data []byte, media string) (string, error) {
	digest := Digest(data)
	k, _ := key(digest)
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(k), Body: bytes.NewReader(data), ContentLength: aws.Int64(int64(len(data))), ContentType: aws.String(media), IfNoneMatch: aws.String("*"), Metadata: map[string]string{"sha256": digest[7:]}})
	if err != nil {
		head, e := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(k)})
		if e == nil && head.ContentLength != nil && *head.ContentLength == int64(len(data)) && head.Metadata["sha256"] == digest[7:] {
			return digest, nil
		}
		return "", fmt.Errorf("store immutable object: %w", err)
	}
	return digest, nil
}
func (s *Store) Get(ctx context.Context, digest string, max int64) ([]byte, error) {
	k, err := key(digest)
	if err != nil {
		return nil, err
	}
	r, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(k)})
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	b, err := io.ReadAll(io.LimitReader(r.Body, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, errors.New("object exceeds size limit")
	}
	if Digest(b) != digest {
		return nil, errors.New("object digest mismatch")
	}
	return b, nil
}
func (s *Store) SignedRead(ctx context.Context, digest string, ttl time.Duration) (string, error) {
	k, err := key(digest)
	if err != nil {
		return "", err
	}
	r, err := s3.NewPresignClient(s.client).PresignGetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(k)}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return r.URL, nil
}
func (s *Store) SignedWrite(ctx context.Context, digest string, size int64, ttl time.Duration) (string, error) {
	k, err := key(digest)
	if err != nil {
		return "", err
	}
	if size <= 0 || size > 1<<30 {
		return "", errors.New("prepared object size exceeds policy")
	}
	r, err := s3.NewPresignClient(s.client).PresignPutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(k), ContentLength: aws.Int64(size), IfNoneMatch: aws.String("*")}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return r.URL, nil
}
func (s *Store) Verify(ctx context.Context, digest string, size int64) error {
	k, err := key(digest)
	if err != nil {
		return err
	}
	if size <= 0 || size > 1<<30 {
		return errors.New("prepared object size exceeds policy")
	}
	r, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(k)})
	if err != nil {
		return err
	}
	defer r.Body.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(r.Body, size+1))
	if err != nil {
		return err
	}
	if n != size || "sha256:"+hex.EncodeToString(h.Sum(nil)) != digest {
		return errors.New("uploaded prepared object digest or size mismatch")
	}
	return nil
}
