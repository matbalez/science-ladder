package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSigningBoundary(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	h := handler("host-a", key)
	d := sha256.Sum256([]byte("test host result"))
	digest := base64.StdEncoding.EncodeToString(d[:])
	good := `{"keyId":"host-a","algorithm":"ECDSA_SHA_256","digest":"` + digest + `"}`
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/v1/sign", strings.NewReader(good)))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	var result struct {
		Signature string `json:"signature"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	sig, _ := base64.StdEncoding.DecodeString(result.Signature)
	if !ecdsa.VerifyASN1(&key.PublicKey, d[:], sig) {
		t.Fatal("signature does not verify")
	}
	for _, bad := range []string{strings.Replace(good, "host-a", "platform-root", 1), strings.Replace(good, "ECDSA_SHA_256", "RAW", 1), strings.Replace(good, digest, "AA==", 1), strings.Replace(good, `"keyId":"host-a"`, `"keyId":"host-a","keyId":"host-a"`, 1), good + `{}`, strings.Repeat("x", 1025)} {
		w = httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("POST", "/v1/sign", strings.NewReader(bad)))
		if w.Code < 400 || strings.Contains(w.Body.String(), "signature") {
			t.Fatal("invalid request released a signature")
		}
	}
}
