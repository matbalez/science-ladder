package platform

import (
	"net/http"
)

func (s *Server) routes(m *http.ServeMux) {
	m.HandleFunc("GET /v1/audit/events", s.wrap(false, s.publicAuditEvents))
	m.HandleFunc("GET /v1/audit/checkpoints", s.wrap(false, s.publicCheckpoints))
	m.HandleFunc("POST /v1/audit/checkpoints/{digest}/witness", s.wrap(false, s.witnessReceipt))
	m.HandleFunc("GET /v1/me", s.wrap(false, s.me))
	m.HandleFunc("GET /v1/auth/github", s.wrap(false, s.authStart))
	m.HandleFunc("GET /v1/auth/github/callback", s.wrap(false, s.authCallback))
	m.HandleFunc("POST /v1/auth/logout", s.wrap(false, s.logout))
	m.HandleFunc("POST /v1/auth/cli-sessions", s.wrap(false, s.cliStart))
	m.HandleFunc("POST /v1/auth/cli-sessions/{id}/approve", s.wrap(true, s.cliApprove))
	m.HandleFunc("POST /v1/auth/cli-sessions/{id}/token", s.wrap(false, s.cliToken))
	m.HandleFunc("POST /v1/webhooks/github", func(w http.ResponseWriter, r *http.Request) {
		if err := s.webhook(w, r); err != nil {
			writeError(w, err)
		}
	})
	m.HandleFunc("GET /v1/challenges", s.wrap(false, s.listChallenges))
	m.HandleFunc("GET /v1/challenges/{slug}", s.wrap(false, s.getChallenge))
	m.HandleFunc("GET /v1/dashboard", s.wrap(true, s.dashboard))
	m.HandleFunc("GET /v1/prompts/challenge-scout/{version}", s.wrap(false, s.scout))
	m.HandleFunc("POST /v1/prompts/challenge-scout/{version}/prefill", s.wrap(false, s.scout))
	m.HandleFunc("POST /v1/candidates/validate", s.wrap(false, s.validateCandidate))
	m.HandleFunc("POST /v1/candidates/import", s.wrap(true, s.importCandidate))
	m.HandleFunc("GET /v1/candidates/{id}", s.wrap(true, s.getCandidate))
	m.HandleFunc("POST /v1/challenges/{id}/versions", s.wrap(true, s.createVersion))
	m.HandleFunc("GET /v1/openapi.json", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "docs/openapi.json") })
	m.HandleFunc("POST /v1/challenges", s.wrap(true, s.createChallenge))
	m.HandleFunc("POST /v1/challenge-versions/{id}/preflights", s.wrap(true, s.startPreflight))
	m.HandleFunc("GET /v1/preflights/{id}", s.wrap(true, s.getPreflight))
	m.HandleFunc("POST /v1/challenge-versions/{id}/lock", s.wrap(true, s.lockChallenge))
	m.HandleFunc("POST /v1/challenge-versions/{id}/publish", s.wrap(true, s.publishChallenge))
	m.HandleFunc("POST /v1/submission-intents", s.wrap(true, s.createIntent))
	m.HandleFunc("GET /v1/submission-intents/{id}", s.wrap(true, s.getIntent))
	m.HandleFunc("POST /v1/submission-intents/{id}/accept", s.wrap(true, s.acceptIntent))
	m.HandleFunc("GET /v1/submissions/{id}", s.wrap(false, s.getSubmission))
	m.HandleFunc("POST /v1/submissions/{id}/publish", s.wrap(true, s.publishSubmission))
	m.HandleFunc("POST /v1/flags", s.wrap(true, s.createFlag))
	m.HandleFunc("GET /v1/editor/queue", s.wrap(true, s.editorQueue))
	m.HandleFunc("POST /v1/editor/decisions", s.wrap(true, s.editorDecision))
	m.HandleFunc("POST /v1/invites", s.wrap(true, s.invite))
	m.HandleFunc("GET /v1/receipts/{digest}", s.wrap(false, s.getReceipt))
	m.HandleFunc("GET /v1/artifacts/{digest}", s.wrap(false, s.getArtifact))
	m.HandleFunc("GET /v1/exports/challenge-versions/{id}", s.wrap(false, s.exportChallenge))
	m.HandleFunc("GET /v1/challenge-versions/{id}/events", s.wrap(false, s.events))
	m.HandleFunc("GET /.well-known/science-ladder-keys.json", s.wrap(false, s.keys))
}
