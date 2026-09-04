// witness is an independently operated append-only checkpoint observer.
package main

import (
	"context"
	"crypto"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/matbalez/science-ladder/internal/audit"
	"github.com/matbalez/science-ladder/internal/signing"
	"github.com/matbalez/science-ladder/pkg/protocol"
)

func main() {
	if e := run(); e != nil {
		slog.Error("witness stopped", "error", e)
		os.Exit(1)
	}
}
func run() error {
	listen := flag.String("listen", "127.0.0.1:8090", "HTTP listen address behind the operator's TLS proxy")
	journal := flag.String("journal", "work/witness/journal.ndjson", "independently retained append-only journal")
	platform := flag.String("platform", "", "optional HTTPS Science Ladder origin to observe continuously")
	rootFile := flag.String("root-key", "", "externally pinned root public PEM")
	rootID := flag.String("root-id", "root-v1", "externally pinned root key ID")
	historyFile := flag.String("history", "", "root-signed key-history envelope array")
	keyID := flag.String("key-id", "", "root-delegated witness key ID")
	keyFile := flag.String("key-file", "", "local demonstration PEM, permitted only with --mode controlled-demo")
	mode := flag.String("mode", "official", "official or controlled-demo")
	kmsKey := flag.String("kms-key", "", "nonexportable AWS KMS key identifier")
	kmsRegion := flag.String("kms-region", "", "AWS signing region")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	rootBytes, e := os.ReadFile(*rootFile)
	if e != nil {
		return errors.New("read pinned root key")
	}
	root, e := audit.ParsePublicKey(string(rootBytes))
	if e != nil {
		return e
	}
	historyBytes, e := os.ReadFile(*historyFile)
	if e != nil {
		return errors.New("read signed key history")
	}
	var chain []protocol.Envelope
	if e = protocol.DecodeStrictBounded(historyBytes, &chain, 8<<20); e != nil {
		return e
	}
	if len(chain) == 0 || len(chain) > 1000 {
		return errors.New("bounded nonempty key-history chain required")
	}
	var history *audit.History
	for _, envelope := range chain {
		h, e := audit.VerifyHistory(envelope, *rootID, root, history, time.Now())
		if e != nil {
			return e
		}
		history = &h
	}
	var key crypto.Signer
	if *kmsKey != "" {
		if *keyFile != "" {
			return errors.New("choose exactly one signing provider")
		}
		key, e = signing.NewAWS(ctx, *kmsRegion, *kmsKey)
	} else {
		if *mode != "controlled-demo" {
			return errors.New("official witnesses require a managed nonexportable key")
		}
		info, err := os.Lstat(*keyFile)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
			return errors.New("private witness key must be a mode-600 regular file")
		}
		b, err := os.ReadFile(*keyFile)
		if err != nil {
			return errors.New("read witness key")
		}
		key, e = signing.FromPEM(b, *mode)
	}
	if e != nil {
		return e
	}
	w, e := audit.OpenWitness(*journal, *keyID, key, *history)
	if e != nil {
		return e
	}
	defer w.Close()
	if *platform != "" {
		if e := startObserver(ctx, *platform, w); e != nil {
			return e
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(out http.ResponseWriter, r *http.Request) { out.Write([]byte("ok\n")) })
	mux.HandleFunc("GET /v1/checkpoints/latest", func(out http.ResponseWriter, r *http.Request) {
		bundle, ok := w.Latest()
		if !ok {
			http.Error(out, "no witnessed checkpoint", 404)
			return
		}
		out.Header().Set("Content-Type", "application/json")
		json.NewEncoder(out).Encode(bundle)
	})
	mux.HandleFunc("POST /v1/checkpoints", func(out http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(out, r.Body, audit.MaxBundleBytes))
		if err != nil {
			http.Error(out, "checkpoint bundle exceeds bound", 413)
			return
		}
		var bundle audit.Bundle
		if err = protocol.DecodeStrictBounded(body, &bundle, audit.MaxBundleBytes); err != nil {
			http.Error(out, "invalid checkpoint bundle", 400)
			return
		}
		signature, err := w.Observe(bundle, time.Now())
		if err != nil {
			http.Error(out, err.Error(), 409)
			return
		}
		out.Header().Set("Content-Type", "application/json")
		if signature.Payload == "" {
			out.WriteHeader(http.StatusAccepted)
			json.NewEncoder(out).Encode(map[string]any{"retained": true, "signed": false})
			return
		}
		json.NewEncoder(out).Encode(map[string]any{"envelope": signature})
	})
	server := &http.Server{Addr: *listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	go func() {
		<-ctx.Done()
		shutdown, c := context.WithTimeout(context.Background(), 10*time.Second)
		defer c()
		server.Shutdown(shutdown)
	}()
	slog.Info("witness ready", "listen", *listen, "mode", *mode, "keyId", *keyID)
	if e = server.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
		return e
	}
	return nil
}
