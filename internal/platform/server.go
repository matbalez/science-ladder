package platform

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	logaudit "github.com/matbalez/science-ladder/internal/audit"
	secretbox "github.com/matbalez/science-ladder/internal/secrets"
	"github.com/matbalez/science-ladder/internal/signing"
	"github.com/matbalez/science-ladder/internal/storage"
	"github.com/matbalez/science-ladder/migrations"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type Server struct {
	SuiteSealer     SecretSealer
	TrustedRootKey  crypto.PublicKey
	ReleaseEnvelope *protocol.Envelope
	TrustHistory    *logaudit.History
	KeyHistory      []protocol.Envelope
	ReceiptSigner   crypto.Signer
	DB              *pgxpool.Pool
	Store           *storage.Store
	Config          Config
	HTTP            *http.Client
}
type User struct {
	SessionScopes []string `json:"-"`
	ID            string   `json:"id"`
	GitHubID      int64    `json:"githubId"`
	Login         string   `json:"login"`
	AvatarURL     string   `json:"avatarUrl"`
	Role          string   `json:"role"`
	Invited       bool     `json:"invited"`
	Quota         int      `json:"-"`
}
type apiError struct {
	Status        int
	Code, Message string
}

func (e *apiError) Error() string                 { return e.Message }
func fail(status int, code, message string) error { return &apiError{status, code, message} }
func New(ctx context.Context, c Config) (*Server, error) {
	if c.DeploymentMode != "local" && c.DeploymentMode != "controlled-demo" && c.DeploymentMode != "production" {
		return nil, errors.New("DEPLOYMENT_MODE must be local, controlled-demo, or production")
	}
	if c.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	db, err := pgxpool.New(ctx, c.DatabaseURL)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(ctx); err != nil {
		db.Close()
		return nil, err
	}
	var st *storage.Store
	if c.S3Bucket != "" {
		st, err = storage.New(ctx, c.S3Bucket, c.S3Region, c.S3Endpoint)
		if err != nil {
			db.Close()
			return nil, err
		}
	}
	var receiptSigner crypto.Signer
	if c.ReceiptKMSKeyID != "" {
		receiptSigner, err = signing.NewAWS(ctx, c.ReceiptKMSRegion, c.ReceiptKMSKeyID)
	} else if c.ReceiptPrivateKey != "" {
		receiptSigner, err = signing.FromPEM([]byte(c.ReceiptPrivateKey), c.DeploymentMode)
	}
	if err != nil {
		db.Close()
		return nil, err
	}
	server := &Server{DB: db, Store: st, Config: c, HTTP: &http.Client{Timeout: c.HTTPTimeout}, ReceiptSigner: receiptSigner}
	if c.HiddenSuiteKMSKeyID != "" {
		server.SuiteSealer, err = secretbox.NewAWS(ctx, c.HiddenSuiteKMSRegion, c.HiddenSuiteKMSKeyID)
		if err != nil {
			db.Close()
			return nil, err
		}
	} else if c.HiddenSuiteMasterKey != "" {
		key, e := base64.StdEncoding.Strict().DecodeString(c.HiddenSuiteMasterKey)
		if e != nil || len(key) != 32 {
			db.Close()
			return nil, errors.New("HIDDEN_SUITE_MASTER_KEY must be a base64 32-byte key")
		}
		server.SuiteSealer, err = secretbox.NewLocal(key, c.DeploymentMode)
		if err != nil {
			db.Close()
			return nil, err
		}
	}

	if err = server.loadTrust(); err != nil {
		db.Close()
		return nil, err
	}
	return server, nil
}
func (s *Server) Migrate(ctx context.Context) error { return migrations.Apply(ctx, s.DB) }
func ID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func secret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func hash(s string) string { b := sha256.Sum256([]byte(s)); return hex.EncodeToString(b[:]) }
func raw(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
func readJSON(r *http.Request, v any) error {
	d := json.NewDecoder(io.LimitReader(r.Body, 2<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return fail(400, "invalid_json", "Request is not valid JSON: "+err.Error())
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return fail(400, "invalid_json", "Exactly one JSON document is required")
	}
	return nil
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, err error) {
	var ae *apiError
	if errors.As(err, &ae) {
		respond(w, ae.Status, map[string]any{"error": map[string]any{"code": ae.Code, "message": ae.Message}})
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		respond(w, 404, map[string]any{"error": map[string]any{"code": "not_found", "message": "Resource not found"}})
		return
	}
	slog.Error("request failed", "error", err)
	respond(w, 500, map[string]any{"error": map[string]any{"code": "internal_error", "message": "The request could not be completed. Retry using the same idempotency key."}})
}
func (s *Server) user(r *http.Request) (*User, error) {
	tok := ""
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		tok = strings.TrimPrefix(h, "Bearer ")
	} else if c, e := r.Cookie("sl_session"); e == nil {
		tok = c.Value
	}
	if tok == "" {
		return nil, nil
	}
	u := &User{}
	err := s.DB.QueryRow(r.Context(), `SELECT u.id,u.github_id,u.login,u.avatar_url,u.role,u.invited,u.validation_quota,s.scopes FROM sessions s JOIN users u ON u.id=s.user_id WHERE token_hash=$1 AND expires_at>now()`, hash(tok)).Scan(&u.ID, &u.GitHubID, &u.Login, &u.AvatarURL, &u.Role, &u.Invited, &u.Quota, &u.SessionScopes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

type handler func(http.ResponseWriter, *http.Request, *User) error

func (s *Server) wrap(auth bool, fn handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		u, err := s.user(r)
		if err != nil {
			writeError(w, err)
			return
		}
		if auth && (u == nil || !u.Invited) {
			writeError(w, fail(403, "invitation_required", "Sign in with an invited GitHub account to continue"))
			return
		}
		if u != nil {
			for _, scope := range u.SessionScopes {
				if scope == "cli" && (strings.HasPrefix(r.URL.Path, "/v1/editor/") || r.URL.Path == "/v1/invites" || strings.HasSuffix(r.URL.Path, "/approve")) {
					writeError(w, fail(403, "token_scope_forbidden", "CLI tokens permit creator and solver actions; account administration and device approvals require a browser session"))
					return
				}
			}
		}
		if r.Method != "GET" && r.Method != "HEAD" && r.Header.Get("Authorization") == "" {
			origin := r.Header.Get("Origin")
			if origin != "" && origin != s.Config.PublicOrigin {
				writeError(w, fail(403, "origin_mismatch", "Request origin does not match this application"))
				return
			}
			if u != nil && origin == "" {
				writeError(w, fail(403, "origin_required", "Browser changes require an Origin header"))
				return
			}
		}
		if err = fn(w, r, u); err != nil {
			writeError(w, err)
		}
	}
}

var idemRE = regexp.MustCompile(`^[A-Za-z0-9._:-]{8,128}$`)

// mutate serializes retries by identity/key. The business transaction is also the
// idempotency transaction, so a process crash cannot duplicate the mutation.
func (s *Server) mutate(w http.ResponseWriter, r *http.Request, u *User, fn func(pgx.Tx) (int, any, error)) error {
	key := r.Header.Get("Idempotency-Key")
	if !idemRE.MatchString(key) {
		return fail(400, "idempotency_required", "Idempotency-Key must contain 8–128 safe characters")
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		return err
	}
	reqHash := hash(r.Method + " " + r.URL.Path + "\n" + string(body))
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	tx, err := s.DB.BeginTx(r.Context(), pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	_, err = tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, u.ID+":"+key)
	if err != nil {
		return err
	}
	var oldHash string
	var oldBody []byte
	var oldStatus *int
	err = tx.QueryRow(r.Context(), `SELECT request_hash,response,status_code FROM idempotency WHERE user_id=$1 AND key=$2`, u.ID, key).Scan(&oldHash, &oldBody, &oldStatus)
	if err == nil {
		if oldHash != reqHash {
			return fail(409, "idempotency_conflict", "This key was already used for different request content")
		}
		if oldStatus == nil {
			return fail(409, "request_pending", "The original request is still processing")
		}
		respond(w, *oldStatus, json.RawMessage(oldBody))
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO idempotency(user_id,key,request_hash) VALUES($1,$2,$3)`, u.ID, key, reqHash); err != nil {
		return err
	}
	status, response, err := fn(tx)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), `UPDATE idempotency SET response=$3,status_code=$4 WHERE user_id=$1 AND key=$2`, u.ID, key, raw(response), status); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	respond(w, status, response)
	return nil
}
func enqueue(ctx context.Context, tx pgx.Tx, kind, id string) error {
	_, err := tx.Exec(ctx, `INSERT INTO jobs(id,kind,resource_id) VALUES($1,$2,$3) ON CONFLICT(kind,resource_id) DO NOTHING`, ID(), kind, id)
	return err
}
func audit(ctx context.Context, tx pgx.Tx, version, kind string, payload any) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(6842077292)`); err != nil {
		return err
	}
	previous := "genesis"
	var sequence int64
	err := tx.QueryRow(ctx, `SELECT digest,sequence FROM audit_events ORDER BY sequence DESC LIMIT 1`).Scan(&previous, &sequence)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	canonical, err := protocol.CanonicalJSON(raw(payload))
	if err != nil {
		return err
	}
	digest := storage.Digest(append([]byte(previous+"\n"+kind+"\n"), canonical...))
	var v any
	if version != "" {
		v = version
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_events(sequence,version_id,kind,payload,previous_digest,digest) OVERRIDING SYSTEM VALUE VALUES($1,$2,$3,$4,$5,$6)`, sequence+1, v, kind, canonical, previous, digest)
	return err
}
func (s *Server) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, map[string]string{"status": "ok"}) })
	m.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.DB.Ping(ctx); err != nil {
			respond(w, 503, map[string]string{"status": "database_unavailable"})
			return
		}
		var migrated bool
		if err := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, migrations.Latest()).Scan(&migrated); err != nil || !migrated {
			respond(w, 503, map[string]string{"status": "migration_required"})
			return
		}
		respond(w, 200, map[string]string{"status": "ready"})
	})
	s.routes(m)
	return http.MaxBytesHandler(m, 2<<20)
}
