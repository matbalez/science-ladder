package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateSuiteDocumentIsBoundedInertAndComplete(t *testing.T) {
	root := t.TempDir()
	original := []byte(`{"test":1}`)
	if err := os.WriteFile(filepath.Join(root, "test.json"), original, 0644); err != nil {
		t.Fatal(err)
	}
	document, err := privateSuiteDocument(root, "CC0-1.0")
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Files map[string][]byte `json:"files"`
	}
	if err := json.Unmarshal(document, &decoded); err != nil || string(decoded.Files["test.json"]) != string(original) {
		t.Fatal("suite document changed", err)
	}
	if err := os.WriteFile(filepath.Join(root, "payload.json"), []byte("#!/bin/sh\necho unsafe"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := privateSuiteDocument(root, "CC0-1.0"); err == nil {
		t.Fatal("active content accepted")
	}
}
