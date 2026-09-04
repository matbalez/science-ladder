package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/matbalez/science-ladder/internal/audit"
	"github.com/matbalez/science-ladder/pkg/protocol"
)

func startObserver(ctx context.Context, origin string, w *audit.Witness) error {
	u, e := url.Parse(origin)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return errors.New("witness platform must be an HTTPS origin without credentials, path, query, or fragment")
	}
	base := strings.TrimRight(origin, "/")
	client := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		return errors.New("witness observer does not follow redirects")
	}}
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			if e := observeOne(ctx, client, base, w); e != nil && ctx.Err() == nil {
				slog.Warn("checkpoint observation paused; retained history is unchanged", "reason", e.Error())
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return nil
}

func observeOne(ctx context.Context, client *http.Client, base string, w *audit.Witness) error {
	query := "?after=0&limit=1"
	if latest, ok := w.Latest(); ok {
		payload, e := base64.StdEncoding.Strict().DecodeString(latest.Checkpoint.Payload)
		if e != nil {
			return e
		}
		digest := audit.Hash(payload)
		// Replay after every restart or lost HTTP response before advancing further.
		if len(latest.Witnesses) > 1 {
			return errors.New("unexpected local witness signatures")
		}
		// An authenticated historical catch-up record advances the cursor but
		// has no local vote to publish under the replacement witness's key.
		if len(latest.Witnesses) == 1 {
			if e = exchange(ctx, client, http.MethodPost, base+"/v1/audit/checkpoints/"+url.PathEscape(digest)+"/witness", map[string]any{"envelope": latest.Witnesses[0]}, nil); e != nil {
				return e
			}
		}
		query = "?afterDigest=" + url.QueryEscape(digest) + "&limit=1"
	}
	var response struct {
		Checkpoints []struct {
			ID             string       `json:"id"`
			Digest         string       `json:"digest"`
			Bundle         audit.Bundle `json:"bundle"`
			QuorumVerified bool         `json:"quorumVerified"`
			IssuedAt       time.Time    `json:"issuedAt"`
		} `json:"checkpoints"`
		DeploymentMode string `json:"deploymentMode"`
	}
	if e := exchange(ctx, client, http.MethodGet, base+"/v1/audit/checkpoints"+query, nil, &response); e != nil {
		return e
	}
	if len(response.Checkpoints) == 0 {
		return nil
	}
	if len(response.Checkpoints) > 1 {
		return errors.New("platform exceeded requested checkpoint page size")
	}
	item := response.Checkpoints[0]
	payload, e := base64.StdEncoding.Strict().DecodeString(item.Bundle.Checkpoint.Payload)
	if e != nil || audit.Hash(payload) != item.Digest {
		return errors.New("platform checkpoint digest mismatch")
	}
	if _, e = w.Observe(item.Bundle, time.Now()); e != nil {
		return e
	}
	slog.Info("checkpoint independently retained", "digest", item.Digest)
	return nil
}

func exchange(ctx context.Context, client *http.Client, method, target string, body any, destination any) error {
	var reader io.Reader
	if body != nil {
		b, e := json.Marshal(body)
		if e != nil {
			return e
		}
		reader = bytes.NewReader(b)
	}
	request, e := http.NewRequestWithContext(ctx, method, target, reader)
	if e != nil {
		return errors.New("construct witness exchange")
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, e := client.Do(request)
	if e != nil {
		return errors.New("platform temporarily unreachable")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("platform returned HTTP %d", response.StatusCode)
	}
	if destination == nil {
		io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return nil
	}
	b, e := io.ReadAll(io.LimitReader(response.Body, audit.MaxBundleBytes+1))
	if e != nil || len(b) > audit.MaxBundleBytes {
		return errors.New("checkpoint response exceeds bound")
	}
	return protocol.DecodeStrictBounded(b, destination, audit.MaxBundleBytes)
}
