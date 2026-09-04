//go:build ignore

// Inspect or enable versioning on the explicitly configured private bucket.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"os"
	"time"
)

func main() {
	enable := flag.Bool("enable", false, "enable bucket versioning (retains prior object versions)")
	flag.Parse()
	if os.Getenv("S3_BUCKET") == "" || os.Getenv("S3_ENDPOINT") == "" {
		fmt.Fprintln(os.Stderr, "S3_BUCKET and S3_ENDPOINT must explicitly identify the target")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(os.Getenv("S3_REGION")))
	if err != nil {
		fmt.Fprintln(os.Stderr, "load object-store configuration")
		os.Exit(1)
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) { o.BaseEndpoint = aws.String(os.Getenv("S3_ENDPOINT")); o.UsePathStyle = true })
	bucket := aws.String(os.Getenv("S3_BUCKET"))
	if *enable {
		_, err = client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{Bucket: bucket, VersioningConfiguration: &types.VersioningConfiguration{Status: types.BucketVersioningStatusEnabled}})
		if err != nil {
			var ae smithy.APIError
			code := "transport-error"
			if errors.As(err, &ae) {
				code = ae.ErrorCode()
			}
			fmt.Fprintln(os.Stderr, "could not enable object versioning:", code)
			os.Exit(1)
		}
	}
	status, err := client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: bucket})
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not inspect object versioning")
		os.Exit(1)
	}
	fmt.Printf("Bucket %s: versioning=%s\n", *bucket, status.Status)
}
