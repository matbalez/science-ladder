package main

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/matbalez/science-ladder/internal/runner"
	"github.com/matbalez/science-ladder/pkg/protocol"
)

func main() {
	if filepath.Base(os.Args[0]) == "sl-init" || len(os.Args) > 1 && os.Args[1] == "guest-init" {
		if err := runner.GuestInit(); err != nil {
			fmt.Fprintln(os.Stderr, "SL_BOOT_ERROR:", err)
			os.Exit(1)
		}
		return
	}
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "runnerd:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "--help" {
		fmt.Print(help)
		return nil
	}
	if args[0] == "generate-host-encryption-key" {
		return generateHostEncryptionKey(args[1:])
	}
	if args[0] == "build-rootfs" {
		return buildRootfs(args[1:])
	}
	if args[0] == "local-preflight" {
		return localPreflight(args[1:])
	}
	if args[0] == "runtime-inventory" {
		return runtimeInventory(args[1:])
	}
	if args[0] == "dependency-inventory" {
		return dependencyInventory(args[1:])
	}
	if args[0] == "advisory-check" {
		return advisoryCheck(args[1:])
	}
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	configPath := f.String("config", "", "host configuration JSON")
	keysPath := f.String("keys", "", "trusted control-plane keys JSON")
	hostKeysPath := f.String("host-keys", "", "certified host public keys JSON")
	keyID := f.String("key-id", "", "host signing key ID")
	socket := f.String("signer-socket", "", "private host signing agent socket")
	encryptionKeyPath := f.String("encryption-key", "", "private base64 X25519 host decryption key")
	jobPath := f.String("job", "", "signed job JSON")
	out := f.String("out", "", "signed receipt destination")
	api := f.String("api", "", "runner API origin")
	certPath := f.String("tls-cert", "", "host client certificate")
	privatePath := f.String("tls-key", "", "host client TLS private key")
	caPath := f.String("tls-ca", "", "runner API CA certificate")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	data, err := os.ReadFile(*configPath)
	if err != nil {
		return err
	}
	var config runner.Config
	if err := protocol.DecodeStrict(data, &config); err != nil {
		return err
	}
	if args[0] == "config-digest" {
		digest, err := runner.ConfigBindingDigest(config)
		if err != nil {
			return err
		}
		fmt.Println(digest)
		return nil
	}
	keys, err := runner.ReadPublicKeys(*keysPath)
	if err != nil {
		return err
	}
	if err := config.CheckHost(keys); err != nil {
		return err
	}
	if args[0] == "host-check" {
		fmt.Println("Host configuration, pinned components and signed inventory attestation passed.")
		return nil
	}
	hostKeys, err := runner.ReadPublicKeys(*hostKeysPath)
	if err != nil {
		return err
	}
	public, ok := hostKeys[*keyID]
	if !ok {
		return errors.New("host signing key not found")
	}
	runtime := &runner.Runtime{Config: config, Keys: keys, KeyID: *keyID, Signer: &runner.SocketSigner{KeyID: *keyID, PublicKey: public, SocketPath: *socket}}
	if *encryptionKeyPath != "" {
		info, err := os.Stat(*encryptionKeyPath)
		if err != nil || info.Mode().Perm()&0077 != 0 {
			return errors.New("private mode 0600 host encryption key required")
		}
		data, err := os.ReadFile(*encryptionKeyPath)
		if err != nil {
			return err
		}
		key, err := base64.StdEncoding.Strict().DecodeString(strings.TrimSpace(string(data)))
		protocol.ZeroBytes(data)
		if err != nil || len(key) != 32 {
			return errors.New("invalid host X25519 private key")
		}
		runtime.EncryptionPrivateKey = key
		defer protocol.ZeroBytes(key)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if args[0] == "hardware-probe" {
		if *out == "" {
			return errors.New("--out required")
		}
		receipt, probeErr := runtime.HardwareProbe(ctx, os.Stderr)
		if receipt.Payload == "" {
			return probeErr
		}
		if err := writeNewJSON(*out, receipt); err != nil {
			return err
		}
		return probeErr
	}
	if args[0] == "run" {
		data, err := os.ReadFile(*jobPath)
		if err != nil {
			return err
		}
		var envelope protocol.Envelope
		if err := protocol.DecodeStrict(data, &envelope); err != nil {
			return err
		}
		receipt, err := runtime.Run(ctx, envelope)
		if err != nil {
			return err
		}
		encoded, err := json.MarshalIndent(receipt, "", "  ")
		if err != nil {
			return err
		}
		if *out == "" {
			return errors.New("--out required")
		}
		return os.WriteFile(*out, append(encoded, '\n'), 0600)
	}
	if args[0] != "serve" {
		return errors.New("unknown runnerd command")
	}
	client, err := mtlsClient(*certPath, *privatePath, *caPath)
	if err != nil {
		return err
	}
	u, err := url.Parse(*api)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Host == "" {
		return errors.New("runner API requires HTTPS")
	}
	base := strings.TrimRight(*api, "/")
	if err := replayResults(ctx, client, base, config.ResultSpool); err != nil {
		return err
	}
	for ctx.Err() == nil {
		var claim struct {
			Job         *protocol.Envelope `json:"job"`
			ResultToken string             `json:"resultToken"`
		}
		err := request(ctx, client, "POST", base+"/internal/v1/runner/jobs/claim", map[string]any{}, &claim)
		if err != nil {
			fmt.Fprintln(os.Stderr, "runner claim unavailable; retrying without accepting work")
			if !pause(ctx, 10*time.Second) {
				break
			}
			continue
		}
		if claim.Job == nil {
			if !pause(ctx, 3*time.Second) {
				break
			}
			continue
		}
		payload, err := protocol.Verify(*claim.Job, keys)
		if err != nil {
			fmt.Fprintln(os.Stderr, "rejected job signature")
			continue
		}
		var job protocol.RunnerJob
		if err := protocol.DecodeStrict(payload, &job); err != nil {
			fmt.Fprintln(os.Stderr, "rejected job schema")
			continue
		}
		var result protocol.Envelope
		if job.Purpose == "preflight" || job.Purpose == "artifact_prepare" {
			upload := uploader(client, base, job.ID, claim.ResultToken)
			result, err = runtime.Prepare(ctx, *claim.Job, upload)
		} else {
			result, err = runtime.Run(ctx, *claim.Job)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "execution failed before an authoritative result:", err)
			if !pause(ctx, 3*time.Second) {
				break
			}
			continue
		}
		spoolFile, err := persistResult(config.ResultSpool, delivery{JobID: job.ID, Envelope: result, ResultToken: claim.ResultToken})
		if err != nil {
			return err
		}
		body := map[string]any{"envelope": result, "resultToken": claim.ResultToken}
		for attempt := 0; attempt < 3; attempt++ {
			err = request(ctx, client, "POST", base+"/internal/v1/runner/jobs/"+url.PathEscape(job.ID)+"/result", body, nil)
			if err == nil {
				break
			}
			if !pause(ctx, time.Duration(attempt+1)*time.Second) {
				break
			}
		}
		if err != nil {
			return errors.New("could not deliver signed result; persisted receipt requires reconciliation before new work")
		}
		if err := removePersisted(spoolFile); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func mtlsClient(certPath, keyPath, caPath string) (*http.Client, error) {
	certificate, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	data, err := os.ReadFile(caPath)
	if err != nil {
		return nil, err
	}
	if !roots.AppendCertsFromPEM(data) {
		return nil, errors.New("invalid runner API CA")
	}
	return &http.Client{Timeout: 5 * time.Minute, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate}, RootCAs: roots}}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("runner API redirect rejected") }}, nil
}
func request(ctx context.Context, client *http.Client, method, address string, body any, result any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	r, err := http.NewRequestWithContext(ctx, method, address, bytes.NewReader(data))
	if err != nil {
		return err
	}
	r.Header.Set("Content-Type", "application/json")
	response, err := client.Do(r)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == 204 {
		return nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("runner API returned HTTP %d", response.StatusCode)
	}
	if result == nil {
		return nil
	}
	return json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(result)
}
func pause(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func uploader(client *http.Client, base, jobID, token string) runner.UploadObject {
	return func(ctx context.Context, role, filename string, ref protocol.ObjectRef) (protocol.ObjectRef, error) {
		var grant struct {
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
		}
		if err := request(ctx, client, "POST", base+"/internal/v1/runner/jobs/"+url.PathEscape(jobID)+"/objects", map[string]any{"role": role, "digest": ref.Digest, "size": ref.Size, "resultToken": token}, &grant); err != nil {
			return ref, err
		}
		u, err := url.Parse(grant.URL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
			return ref, errors.New("invalid object upload grant")
		}
		file, err := os.Open(filename)
		if err != nil {
			return ref, err
		}
		defer file.Close()
		req, err := http.NewRequestWithContext(ctx, "PUT", grant.URL, file)
		if err != nil {
			return ref, err
		}
		req.ContentLength = ref.Size
		for name, value := range grant.Headers {
			req.Header.Set(name, value)
		}
		uploadClient := &http.Client{Timeout: 5 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("object upload redirect refused") }}
		response, err := uploadClient.Do(req)
		if err != nil {
			return ref, err
		}
		defer response.Body.Close()
		if (response.StatusCode < 200 || response.StatusCode >= 300) && response.StatusCode != http.StatusPreconditionFailed {
			return ref, fmt.Errorf("object upload returned HTTP %d", response.StatusCode)
		}
		ref.URL = ""
		return ref, nil
	}
}

func localPreflight(args []string) error {
	f := flag.NewFlagSet("local-preflight", flag.ContinueOnError)
	manifestPath := f.String("manifest", "science-ladder.yaml", "manifest")
	snapshotPath := f.String("snapshot", "", "exact source snapshot JSON")
	output := f.String("out", "", "new output directory")
	tool := f.String("mksquashfs", "", "pinned mksquashfs executable")
	digest := f.String("mksquashfs-digest", "", "pinned tool digest")
	unsafe := f.Bool("unsafe-local", false, "nonofficial local build")
	scanConfig := f.String("scan-config", "", "platform scan policy config with pinned runtimeInventory, advisorySnapshot and advisoryKeys")
	if err := f.Parse(args); err != nil {
		return err
	}
	if !*unsafe || *output == "" {
		return errors.New("--unsafe-local and --out are required")
	}
	data, err := os.ReadFile(*manifestPath)
	if err != nil {
		return err
	}
	m, err := protocol.ParseManifest(data)
	if err != nil {
		return err
	}
	data, err = os.ReadFile(*snapshotPath)
	if err != nil {
		return err
	}
	snapshot, err := runner.ReadSourceSnapshot(data)
	if err != nil {
		return err
	}
	if err := os.Mkdir(*output, 0700); err != nil {
		return err
	}
	job := protocol.RunnerJob{Manifest: m, SourceSnapshot: &protocol.ObjectRef{Digest: protocol.DigestBytes(data), Size: int64(len(data))}}
	builder := runner.Builder{UnsafeLocal: true, MakeSquashFS: runner.PinnedFile{Path: *tool, Digest: *digest}}
	if *scanConfig != "" {
		data, err := os.ReadFile(*scanConfig)
		if err != nil {
			return err
		}
		var config runner.Config
		if err := protocol.DecodeStrict(data, &config); err != nil {
			return err
		}
		builder.Runtime = &runner.Runtime{Config: config}
	}
	report, err := builder.Preflight(context.Background(), job, snapshot, *output)
	_ = runner.WriteJSON(os.Stdout, map[string]any{"official": false, "report": report})
	return err
}

const help = `Science Ladder isolated validation daemon

  runnerd runtime-inventory --image IMAGE@sha256:DIGEST --out NEW_FILE
  runnerd dependency-inventory --runtime-inventory FILE --snapshot FILE --out NEW_FILE
  runnerd advisory-check --runtime-inventory FILE --snapshot UNSIGNED_DRAFT --out NEW_FILE
  runnerd generate-host-encryption-key --private-out NEW_PRIVATE_FILE --public-out NEW_PUBLIC_FILE
  runnerd build-rootfs --python-image IMAGE@sha256:DIGEST --tools-image IMAGE@sha256:DIGEST
      --guest-init ./runnerd-linux-amd64 --out NEW_DIRECTORY
  runnerd config-digest --config host.json
  runnerd host-check --config host.json --keys platform-keys.json
  runnerd serve --config host.json --keys platform-keys.json --host-keys host-keys.json
      --key-id HOST_KEY --signer-socket /run/science-ladder/sign.sock --encryption-key host-x25519.key
      --api https://RUNNER-API --tls-cert host.crt --tls-key host.key --tls-ca ca.crt
  runnerd run [same host/signing options] --job signed-job.json --out receipt.json
  runnerd hardware-probe [same host/signing options] --out NEW_RECEIPT_FILE
  runnerd local-preflight --unsafe-local --manifest science-ladder.yaml
      --snapshot source-snapshot.json --out NEW_DIRECTORY
      --mksquashfs /absolute/path --mksquashfs-digest sha256:...

Production execution requires an attested dedicated Linux amd64 KVM host and pinned
runtime components. Local container runs never emit authoritative receipts.
`

func buildRootfs(args []string) error {
	f := flag.NewFlagSet("build-rootfs", flag.ContinueOnError)
	python := f.String("python-image", "", "pinned platform Python OCI reference")
	tools := f.String("tools-image", "", "pinned filesystem-tools OCI reference")
	guest := f.String("guest-init", "", "compiled Linux amd64 runnerd")
	out := f.String("out", "", "new output directory")
	if err := f.Parse(args); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	return runner.BuildRootFS(ctx, *python, *tools, *guest, *out)
}

func generateHostEncryptionKey(args []string) error {
	f := flag.NewFlagSet("generate-host-encryption-key", flag.ContinueOnError)
	privateOut := f.String("private-out", "", "new private key file")
	publicOut := f.String("public-out", "", "new public key file")
	if err := f.Parse(args); err != nil {
		return err
	}
	if *privateOut == "" || *publicOut == "" {
		return errors.New("both output paths required")
	}
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	private, err := os.OpenFile(*privateOut, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = private.WriteString(base64.StdEncoding.EncodeToString(key.Bytes()) + "\n")
	_ = private.Close()
	if err != nil {
		return err
	}
	public, err := os.OpenFile(*publicOut, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer public.Close()
	_, err = public.WriteString(base64.StdEncoding.EncodeToString(key.PublicKey().Bytes()) + "\n")
	return err
}
