//go:build ignore

// Enable/inspect Tigris snapshot support using the vendor's documented API.
// Reference: github.com/tigrisdata/storage, packages/storage/src/lib/bucket.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
)

func main() {
	enable := flag.Bool("enable", false, "enable snapshot support for the specified bucket")
	flag.Parse()
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" || os.Getenv("S3_ENDPOINT") != "https://t3.storage.dev" {
		fail("explicit Tigris endpoint and bucket required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("auto"))
	if err != nil {
		fail("load storage credentials")
	}
	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		fail("retrieve storage credentials")
	}
	client := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	request := func(method, path string, body []byte) []byte {
		req, e := http.NewRequestWithContext(ctx, method, "https://t3.storage.dev/"+url.PathEscape(bucket)+path, bytes.NewReader(body))
		if e != nil {
			fail("build storage request")
		}
		req.Header.Set("Accept", "application/json")
		if len(body) > 0 {
			req.Header.Set("Content-Type", "application/json")
		}
		hash := sha256.Sum256(body)
		req.Header.Set("X-Amz-Content-Sha256", hex.EncodeToString(hash[:]))
		e = v4.NewSigner(func(o *v4.SignerOptions) { o.DisableURIPathEscaping = true }).SignHTTP(ctx, creds, req, hex.EncodeToString(hash[:]), "s3", "auto", time.Now())
		if e != nil {
			fail("sign storage request")
		}
		res, e := client.Do(req)
		if e != nil {
			fail("storage connection failed")
		}
		defer res.Body.Close()
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			var detail struct {
				Code string `xml:"Code"`
			}
			_ = xml.NewDecoder(io.LimitReader(res.Body, 8192)).Decode(&detail)
			fail(fmt.Sprintf("storage request returned HTTP %d (%s)", res.StatusCode, detail.Code))
		}
		b, e := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		if e != nil {
			fail("read storage response")
		}
		return b
	}
	if *enable {
		request(http.MethodPatch, "", []byte(`{"type":1}`))
	}
	var info struct {
		Name    string `json:"name"`
		Type    int    `json:"type"`
		Objects int    `json:"estimated_unique_rows"`
	}
	if err = json.Unmarshal(request(http.MethodGet, "?metadata&with-size=true", nil), &info); err != nil {
		fail("decode storage metadata")
	}
	if info.Name != bucket {
		fail("storage metadata identifies a different bucket")
	}
	fmt.Printf("Bucket %s: snapshot support=%t; objects=%d\n", info.Name, info.Type == 1, info.Objects)
	if *enable && info.Type != 1 {
		fail("snapshot support was not enabled")
	}
}
func fail(message string) { fmt.Fprintln(os.Stderr, message); os.Exit(1) }
