package platform

import (
	"github.com/matbalez/science-ladder/internal/buildinfo"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	RootPublicKeyFile, RootKeyID, KeyHistoryFile, ReleaseAttestationFile, SourceCommit                                                                                                                                                                                                       string
	DeploymentMode, ReceiptKMSKeyID, ReceiptKMSRegion                                                                                                                                                                                                                                        string
	DatabaseURL, ListenAddr, PublicOrigin, S3Bucket, S3Region, S3Endpoint, GitHubClientID, GitHubClientSecret, GitHubAppID, GitHubAppPrivateKey, GitHubWebhookSecret, OpenAIKey, OpenAIModel, ReceiptKeyID, ReceiptPrivateKey, RunnerListenAddr, RunnerTLSCert, RunnerTLSKey, RunnerClientCA string
	OperatorGitHubID                                                                                                                                                                                                                                                                         int64
	ActiveLimit                                                                                                                                                                                                                                                                              int
	HTTPTimeout                                                                                                                                                                                                                                                                              time.Duration
}

func LoadConfig() Config {
	get := func(k, d string) string {
		if s := os.Getenv(k); s != "" {
			return s
		}
		return d
	}
	id, _ := strconv.ParseInt(get("OPERATOR_GITHUB_ID", "0"), 10, 64)
	return Config{SourceCommit: buildinfo.Commit, RootPublicKeyFile: os.Getenv("ROOT_PUBLIC_KEY_FILE"), RootKeyID: get("ROOT_KEY_ID", "root-v1"), KeyHistoryFile: os.Getenv("KEY_HISTORY_FILE"), ReleaseAttestationFile: os.Getenv("OFFICIAL_RELEASE_ATTESTATION_FILE"), DeploymentMode: get("DEPLOYMENT_MODE", "local"), ReceiptKMSKeyID: os.Getenv("RECEIPT_KMS_KEY_ID"), ReceiptKMSRegion: os.Getenv("RECEIPT_KMS_REGION"), DatabaseURL: os.Getenv("DATABASE_URL"), ListenAddr: get("LISTEN_ADDR", ":8080"), PublicOrigin: strings.TrimRight(get("PUBLIC_ORIGIN", "http://localhost:3000"), "/"), S3Bucket: os.Getenv("S3_BUCKET"), S3Region: get("S3_REGION", "auto"), S3Endpoint: os.Getenv("S3_ENDPOINT"), GitHubClientID: os.Getenv("GITHUB_CLIENT_ID"), GitHubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"), GitHubAppID: os.Getenv("GITHUB_APP_ID"), GitHubAppPrivateKey: os.Getenv("GITHUB_APP_PRIVATE_KEY_PEM"), GitHubWebhookSecret: os.Getenv("GITHUB_WEBHOOK_SECRET"), OpenAIKey: os.Getenv("OPENAI_API_KEY"), OpenAIModel: get("OPENAI_REVIEW_MODEL", "gpt-6-astra"), ReceiptKeyID: get("RECEIPT_KEY_ID", "platform-v1"), ReceiptPrivateKey: os.Getenv("RECEIPT_PRIVATE_KEY"), RunnerListenAddr: os.Getenv("RUNNER_LISTEN_ADDR"), RunnerTLSCert: os.Getenv("RUNNER_TLS_CERT"), RunnerTLSKey: os.Getenv("RUNNER_TLS_KEY"), RunnerClientCA: os.Getenv("RUNNER_CLIENT_CA"), OperatorGitHubID: id, ActiveLimit: 3, HTTPTimeout: 45 * time.Second}
}
