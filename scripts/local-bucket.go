//go:build ignore

// Run only through scripts/dev-services.py; this cannot target a remote store.
package main

import (
	"context"
	"log"
	"net/url"
	"os"
	"time"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)
func main(){
	u,err:=url.Parse(os.Getenv("S3_ENDPOINT"));if err!=nil||u.Scheme!="http"||u.Hostname()!="127.0.0.1"||os.Getenv("DEPLOYMENT_MODE")!="local"{log.Fatal("local bucket initialization requires a loopback development endpoint")}
	ctx,cancel:=context.WithTimeout(context.Background(),20*time.Second);defer cancel()
	cfg,err:=config.LoadDefaultConfig(ctx,config.WithRegion("us-east-1"));if err!=nil{log.Fatal("load local configuration")}
	client:=s3.NewFromConfig(cfg,func(o *s3.Options){o.BaseEndpoint=aws.String(u.String());o.UsePathStyle=true})
	bucket:=aws.String("science-ladder-local")
	if _,err=client.HeadBucket(ctx,&s3.HeadBucketInput{Bucket:bucket});err!=nil{
		if _,err=client.CreateBucket(ctx,&s3.CreateBucketInput{Bucket:bucket});err!=nil{log.Fatal("create local bucket: ",err)}
	}
	_,err=client.PutBucketVersioning(ctx,&s3.PutBucketVersioningInput{Bucket:bucket,VersioningConfiguration:&types.VersioningConfiguration{Status:types.BucketVersioningStatusEnabled}})
	if err!=nil{log.Fatal("enable local object versioning: ",err)}
	log.Print("Local private artifact bucket is ready with object versioning.")
}
