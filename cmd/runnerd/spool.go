package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

type delivery struct {
	JobID       string            `json:"jobId"`
	Envelope    protocol.Envelope `json:"envelope"`
	ResultToken string            `json:"resultToken"`
}

func persistResult(directory string, value delivery) (string, error) {
	if value.JobID == "" || strings.ContainsAny(value.JobID, "./\\\r\n ") {
		return "", errors.New("invalid spool job ID")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	filename := filepath.Join(directory, value.JobID+".json")
	if existing, err := os.ReadFile(filename); err == nil {
		if !bytes.Equal(existing, data) {
			return "", errors.New("conflicting result already persisted")
		}
		return filename, nil
	}
	temporary, err := os.CreateTemp(directory, ".pending-")
	if err != nil {
		return "", err
	}
	tempName := temporary.Name()
	defer os.Remove(tempName)
	if err := temporary.Chmod(0600); err != nil {
		temporary.Close()
		return "", err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return "", err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return "", err
	}
	if err := temporary.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tempName, filename); err != nil {
		return "", err
	}
	if err := syncDirectory(directory); err != nil {
		return "", err
	}
	return filename, nil
}

func syncDirectory(directory string) error {
	file, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}
func removePersisted(filename string) error {
	if err := os.Remove(filename); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(filename))
}

func replayResults(ctx context.Context, client *http.Client, base, directory string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".pending-") {
			continue
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			return errors.New("unexpected result-spool entry; operator review required")
		}
		filename := filepath.Join(directory, entry.Name())
		info, err := os.Lstat(filename)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 2<<20 {
			return errors.New("unsafe result-spool entry")
		}
		data, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		var value delivery
		if err := protocol.DecodeStrictBounded(data, &value, 2<<20); err != nil {
			return err
		}
		body := map[string]any{"envelope": value.Envelope, "resultToken": value.ResultToken}
		if err := request(ctx, client, "POST", base+"/internal/v1/runner/jobs/"+url.PathEscape(value.JobID)+"/result", body, nil); err != nil {
			return errors.New("persisted result needs reconciliation before accepting new work")
		}
		if err := removePersisted(filename); err != nil {
			return err
		}
	}
	return nil
}
