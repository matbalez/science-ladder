package runner

import (
	"bytes"
	"context"
	"crypto"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"time"
)

// SocketSigner delegates signing to a private local agent backed by a host-bound
// key or KMS. The daemon never receives the platform root signing credential.
// API: POST /v1/sign {keyId,digest,algorithm:"ECDSA_SHA_256"} -> {signature}.
type SocketSigner struct {
	KeyID      string
	PublicKey  crypto.PublicKey
	SocketPath string
}

func (s *SocketSigner) Public() crypto.PublicKey { return s.PublicKey }
func (s *SocketSigner) Sign(_ io.Reader, digest []byte, options crypto.SignerOpts) ([]byte, error) {
	if options.HashFunc() != crypto.SHA256 || len(digest) != 32 {
		return nil, errors.New("host signer accepts SHA-256 digests only")
	}
	info, err := os.Stat(s.SocketPath)
	if err != nil || info.Mode()&os.ModeSocket == 0 || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("private signing-agent socket required")
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", s.SocketPath)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
	body, _ := json.Marshal(map[string]string{"keyId": s.KeyID, "digest": base64.StdEncoding.EncodeToString(digest), "algorithm": "ECDSA_SHA_256"})
	response, err := client.Post("http://signer/v1/sign", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, errors.New("host signing agent rejected request")
	}
	var result struct {
		Signature string `json:"signature"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&result); err != nil {
		return nil, err
	}
	signature, err := base64.StdEncoding.Strict().DecodeString(result.Signature)
	if err != nil || len(signature) > 80 {
		return nil, errors.New("invalid host signer response")
	}
	return signature, nil
}
