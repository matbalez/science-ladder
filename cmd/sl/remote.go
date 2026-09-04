package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/matbalez/science-ladder/internal/runner"
)

type apiClient struct {
	base, token string
	http        *http.Client
}

func newClient(base string) (*apiClient, error) {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("valid --api URL required")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost")) {
		return nil, errors.New("API requires HTTPS outside localhost")
	}
	token := os.Getenv("SL_API_TOKEN")
	if token == "" {
		data, _ := os.ReadFile(tokenFile())
		var saved struct{ API, Token string }
		if json.Unmarshal(data, &saved) == nil && saved.API == strings.TrimRight(base, "/") {
			token = saved.Token
		}
	}
	return &apiClient{base: strings.TrimRight(base, "/"), token: token, http: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("API redirect refused") }}}, nil
}
func (c *apiClient) request(method, path string, body any, result any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return err
	}
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
	request.Header.Set("Content-Type", "application/json")
	if method != "GET" {
		var id [16]byte
		if _, err := rand.Read(id[:]); err != nil {
			return err
		}
		request.Header.Set("Idempotency-Key", hex.EncodeToString(id[:]))
	}
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("API returned HTTP %d: %s", response.StatusCode, string(data))
	}
	if result == nil {
		return nil
	}
	return json.Unmarshal(data, result)
}

func remoteCommand(args []string) error {
	command := args[0]
	rest := args[1:]
	if command == "auth" {
		if len(rest) == 0 || rest[0] != "login" {
			return errors.New("usage: sl auth login --api URL")
		}
		rest = rest[1:]
	}
	f := flag.NewFlagSet(command, flag.ContinueOnError)
	api := f.String("api", os.Getenv("SL_API_URL"), "API origin")
	version := f.String("version", "", "challenge version ID")
	repository := f.String("repository", "", "owner/repository")
	commit := f.String("commit", "", "full GitHub commit SHA")
	license := f.String("license", "MIT", "artifact license")
	model := f.String("model", "", "model attribution")
	harness := f.String("harness", "", "harness attribution")
	publish := f.Bool("publish", false, "publish a non-winning submission voluntarily")
	submission := f.String("submission", "", "submission ID")
	out := f.String("out", "", "export destination")
	if err := f.Parse(rest); err != nil {
		return err
	}
	client, err := newClient(*api)
	if err != nil {
		return err
	}
	switch command {
	case "auth":
		return login(client)
	case "submit":
		if *version == "" || *repository == "" || len(*commit) != 40 {
			return errors.New("version, repository and exact full 40-character commit required")
		}
		if _, err := hex.DecodeString(*commit); err != nil {
			return errors.New("commit must be hexadecimal")
		}
		var intent map[string]any
		body := map[string]any{"versionId": *version, "repository": *repository, "ref": *commit, "license": *license, "attribution": map[string]any{"model": *model, "harness": *harness}, "publish": *publish}
		if err := client.request("POST", "/v1/submission-intents", body, &intent); err != nil {
			return err
		}
		id, _ := intent["id"].(string)
		if id == "" {
			id, _ = intent["intentId"].(string)
		}
		if id == "" {
			return errors.New("API did not return submission intent ID")
		}
		if err := runner.WriteJSON(os.Stdout, intent); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "Source intent created; waiting for immutable source preparation.")
		deadline := time.Now().Add(10 * time.Minute)
		for time.Now().Before(deadline) {
			var status map[string]any
			if err := client.request("GET", "/v1/submission-intents/"+url.PathEscape(id), nil, &status); err != nil {
				return err
			}
			state, _ := status["status"].(string)
			if state == "ready" || state == "structurally_valid" {
				var accepted any
				if err := client.request("POST", "/v1/submission-intents/"+url.PathEscape(id)+"/accept", map[string]any{}, &accepted); err != nil {
					return err
				}
				return runner.WriteJSON(os.Stdout, accepted)
			}
			if state == "failed" || state == "rejected" {
				_ = runner.WriteJSON(os.Stdout, status)
				return errors.New("submission preparation failed")
			}
			time.Sleep(2 * time.Second)
		}
		return fmt.Errorf("intent %s remains pending; inspect it in the app before retrying", id)
	case "status":
		if *submission == "" {
			return errors.New("--submission required")
		}
		var status any
		if err := client.request("GET", "/v1/submissions/"+url.PathEscape(*submission), nil, &status); err != nil {
			return err
		}
		return runner.WriteJSON(os.Stdout, status)
	case "export":
		if *version == "" || *out == "" {
			return errors.New("--version and --out required")
		}
		var exported json.RawMessage
		if err := client.request("GET", "/v1/exports/challenge-versions/"+url.PathEscape(*version), nil, &exported); err != nil {
			return err
		}
		file, err := os.OpenFile(*out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = file.Write(append(exported, '\n'))
		return err
	}
	return errors.New("unknown remote command")
}

func tokenFile() string {
	root, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(root, "science-ladder", "session.json")
}
func login(client *apiClient) error {
	var session struct {
		ID              string    `json:"id"`
		DeviceSecret    string    `json:"deviceSecret"`
		UserCode        string    `json:"userCode"`
		VerificationURL string    `json:"verificationUrl"`
		ExpiresAt       time.Time `json:"expiresAt"`
	}
	if err := client.request("POST", "/v1/auth/cli-sessions", map[string]any{}, &session); err != nil {
		return err
	}
	if session.ID == "" || session.DeviceSecret == "" || session.VerificationURL == "" {
		return errors.New("incomplete device login response")
	}
	fmt.Printf("Approve code %s in your browser: %s\n", session.UserCode, session.VerificationURL)
	if runtime.GOOS == "darwin" {
		_ = exec.Command("open", session.VerificationURL).Start()
	} else if runtime.GOOS == "linux" {
		_ = exec.Command("xdg-open", session.VerificationURL).Start()
	}
	for time.Now().Before(session.ExpiresAt) {
		var result struct {
			Token     string    `json:"token"`
			ExpiresAt time.Time `json:"expiresAt"`
		}
		err := client.request("POST", "/v1/auth/cli-sessions/"+url.PathEscape(session.ID)+"/token", map[string]any{"deviceSecret": session.DeviceSecret}, &result)
		if err == nil && result.Token != "" {
			filename := tokenFile()
			if filename == "" {
				return errors.New("configuration directory unavailable")
			}
			if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
				return err
			}
			data, _ := json.Marshal(map[string]any{"API": client.base, "Token": result.Token, "expiresAt": result.ExpiresAt})
			if err := os.WriteFile(filename, data, 0600); err != nil {
				return err
			}
			if err := os.Chmod(filename, 0600); err != nil {
				return err
			}
			fmt.Println("Signed in. Scoped token stored in your private configuration directory.")
			return nil
		}
		time.Sleep(3 * time.Second)
	}
	return errors.New("device login expired")
}
