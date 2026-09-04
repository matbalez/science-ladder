// host-signer keeps a verification host's signing credential out of runnerd.
// It exposes only a private Unix socket, never a network listener.
package main

import (
	"context"
	"crypto"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"syscall"
	"time"

	"github.com/matbalez/science-ladder/internal/signing"
	"github.com/matbalez/science-ladder/pkg/protocol"
)

func main() {
	if err := run(); err != nil {
		slog.Error("host signing service stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	socket := flag.String("socket", "/run/science-ladder-signer/sign.sock", "private Unix socket")
	keyID := flag.String("key-id", "", "host key ID")
	keyFile := flag.String("key-file", "", "mode-600 demonstration PEM")
	mode := flag.String("mode", "production", "production or controlled-demo")
	kmsKey := flag.String("kms-key", "", "host-specific AWS KMS key ID")
	kmsRegion := flag.String("kms-region", "", "AWS signing region")
	flag.Parse()
	if !regexp.MustCompile(`^[a-zA-Z0-9._-]{1,128}$`).MatchString(*keyID) {
		return errors.New("valid host key ID required")
	}
	if *mode != "production" && *mode != "controlled-demo" {
		return errors.New("unknown host signing mode")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	var key crypto.Signer
	var err error
	if *kmsKey != "" {
		if *keyFile != "" {
			return errors.New("choose exactly one signing provider")
		}
		key, err = signing.NewAWS(ctx, *kmsRegion, *kmsKey)
	} else {
		if *mode != "controlled-demo" {
			return errors.New("production host signing requires a managed nonexportable key")
		}
		info, e := os.Lstat(*keyFile)
		if e != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
			return errors.New("private mode-600 regular key file required")
		}
		b, e := os.ReadFile(*keyFile)
		if e != nil {
			return errors.New("read host signing key")
		}
		key, err = signing.FromPEM(b, *mode)
		clear(b)
	}
	if err != nil {
		return err
	}
	dir := filepath.Dir(*socket)
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 || !filepath.IsAbs(*socket) {
		return errors.New("existing private absolute socket directory required")
	}
	if _, err = os.Lstat(*socket); !os.IsNotExist(err) {
		return errors.New("socket path already exists or is inaccessible")
	}
	// Restrictive creation mode avoids an access window before chmod.
	oldMask := syscall.Umask(0077)
	listener, err := net.Listen("unix", *socket)
	syscall.Umask(oldMask)
	if err != nil {
		return errors.New("create private signing socket")
	}
	defer listener.Close()
	server := &http.Server{Handler: handler(*keyID, key), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 5 * time.Second, MaxHeaderBytes: 4096}
	go func() {
		<-ctx.Done()
		stop, c := context.WithTimeout(context.Background(), 10*time.Second)
		defer c()
		_ = server.Shutdown(stop)
	}()
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func handler(keyID string, key crypto.Signer) http.Handler {
	mux := http.NewServeMux()
	slots := make(chan struct{}, 8)
	mux.HandleFunc("POST /v1/sign", func(w http.ResponseWriter, r *http.Request) {
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			http.Error(w, "signer busy", 503)
			return
		}
		b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1024))
		if err != nil {
			http.Error(w, "bounded request required", 413)
			return
		}
		var in struct {
			KeyID     string `json:"keyId"`
			Digest    string `json:"digest"`
			Algorithm string `json:"algorithm"`
		}
		if protocol.DecodeStrictBounded(b, &in, 1024) != nil || in.KeyID != keyID || in.Algorithm != "ECDSA_SHA_256" {
			http.Error(w, "invalid signing request", 400)
			return
		}
		digest, err := base64.StdEncoding.Strict().DecodeString(in.Digest)
		if err != nil || len(digest) != 32 || base64.StdEncoding.EncodeToString(digest) != in.Digest {
			http.Error(w, "SHA256 digest required", 400)
			return
		}
		signature, err := key.Sign(rand.Reader, digest, crypto.SHA256)
		if err != nil {
			http.Error(w, "signing temporarily unavailable", 503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]string{"signature": base64.StdEncoding.EncodeToString(signature)})
	})
	return mux
}
