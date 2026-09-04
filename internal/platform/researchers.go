package platform

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"golang.org/x/net/idna"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// Researcher is curated bibliographic context, not a sponsorship or contact status.
type Researcher struct {
	Name       string `json:"name"`
	ProfileURL string `json:"profileUrl"`
	Connection string `json:"connection"`
	WorkTitle  string `json:"workTitle"`
	WorkURL    string `json:"workUrl"`
}

const researcherEditionJSON = `jsonb_build_object('id',re.id,'versionId',re.version_id,'researchers',re.researchers,'updatedAt',re.created_at,'updatedBy',jsonb_build_object('githubId',re.editor_github_id::text,'login',re.editor_login),'reason',re.reason)`
const researcherContextSQL = `(SELECT ` + researcherEditionJSON + ` FROM challenge_researcher_editions re WHERE re.version_id=v.id ORDER BY re.edition_sequence DESC LIMIT 1)`

func researcherText(value *string, minimum, maximum int) bool {
	*value = strings.TrimSpace(*value)
	n := utf8.RuneCountInString(*value)
	if !utf8.ValidString(*value) || n < minimum || n > maximum {
		return false
	}
	for _, r := range *value {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return false
		}
	}
	return true
}

// These links are never fetched by the platform. Restrict syntax to public HTTPS
// DNS names; this does not certify the destination or make DNS-based trust claims.
func researcherURL(value *string) bool {
	if !researcherText(value, 1, 2048) || strings.ContainsAny(*value, " \\<>\"'") {
		return false
	}
	u, err := url.Parse(*value)
	if err != nil || u.Scheme != "https" || u.Opaque != "" || u.User != nil || u.Host == "" || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	host, err := idna.Lookup.ToASCII(strings.ToLower(u.Hostname()))
	if err != nil || len(host) > 253 || !strings.Contains(host, ".") || strings.HasSuffix(host, ".") || net.ParseIP(host) != nil {
		return false
	}
	labels := strings.Split(host, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	last := labels[len(labels)-1]
	// WHATWG browsers also interpret dotted hexadecimal/octal numbers as IPv4,
	// even when net.ParseIP rejects them. Reject every numeric terminal label.
	if strings.Trim(last, "0123456789") == "" || strings.HasPrefix(last, "0x") && strings.Trim(last[2:], "0123456789abcdef") == "" {
		return false
	}
	switch last {
	case "localhost", "local", "internal", "lan", "home", "arpa", "onion", "test", "invalid":
		return false
	}
	decoded, err := url.PathUnescape(*value)
	if err != nil {
		return false
	}
	for _, c := range decoded {
		if unicode.IsControl(c) {
			return false
		}
	}
	// Canonical URLs omit the default HTTPS port.
	u.Host = host
	canonical := u.String()
	if len(canonical) > 2048 {
		return false
	}
	*value = canonical
	return true
}

func validateResearchers(researchers []Researcher, reason *string) error {
	if researchers == nil || len(researchers) > 6 || !researcherText(reason, 20, 2000) {
		return fail(422, "researcher_context_invalid", "Supply 0–6 researchers and a public editorial reason of 20–2,000 characters; use [] to clear the list")
	}
	seen := map[string]bool{}
	for i := range researchers {
		r := &researchers[i]
		if !researcherText(&r.Name, 1, 120) || !researcherText(&r.Connection, 1, 1000) || !researcherText(&r.WorkTitle, 1, 300) || !researcherURL(&r.ProfileURL) || !researcherURL(&r.WorkURL) {
			return fail(422, "researcher_invalid", "Each researcher needs a bounded name, connection, work title and public HTTPS profile/work links")
		}
		key := cases.Fold().String(norm.NFKC.String(r.Name))
		if seen[key] {
			return fail(422, "researcher_duplicate", "List each researcher once")
		}
		seen[key] = true
	}
	return nil
}

func (s *Server) editResearchers(w http.ResponseWriter, r *http.Request, u *User) error {
	if !editor(u) {
		return fail(403, "editor_required", "An editor role is required")
	}
	return s.mutate(w, r, u, func(tx pgx.Tx) (int, any, error) {
		var in struct {
			Researchers []Researcher `json:"researchers"`
			Reason      string       `json:"reason"`
		}
		if err := readJSON(r, &in); err != nil {
			return 0, nil, err
		}
		if err := validateResearchers(in.Researchers, &in.Reason); err != nil {
			return 0, nil, err
		}
		var version string
		// Serialize editions for the version without changing its immutable row.
		if err := tx.QueryRow(r.Context(), `SELECT id::text FROM challenge_versions WHERE id::text=$1 FOR UPDATE`, r.PathValue("id")).Scan(&version); err != nil {
			return 0, nil, err
		}
		id := ID()
		if _, err := tx.Exec(r.Context(), `INSERT INTO challenge_researcher_editions(id,version_id,editor_id,editor_github_id,editor_login,researchers,reason) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, version, u.ID, u.GitHubID, u.Login, raw(in.Researchers), in.Reason); err != nil {
			return 0, nil, err
		}
		var edition json.RawMessage
		if err := tx.QueryRow(r.Context(), `SELECT `+researcherEditionJSON+` FROM challenge_researcher_editions re WHERE re.id=$1`, id).Scan(&edition); err != nil {
			return 0, nil, err
		}
		digest, err := protocol.Digest(edition)
		if err != nil {
			return 0, nil, err
		}
		// Global audit is public even before publication: bind the edition without
		// leaking private draft text, names or the editorial reason.
		if err = audit(r.Context(), tx, version, "editorial.researchers", map[string]any{"editionId": id, "editionDigest": digest, "researcherCount": len(in.Researchers)}); err != nil {
			return 0, nil, err
		}
		return 201, edition, nil
	})
}
