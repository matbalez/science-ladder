package protocol

import (
	"errors"
	"reflect"
	"strings"
	"time"
)

//go:generate go run ../../protocol/generate.go

var SchemaTypes = map[string]any{"challenge-candidate": Candidate{}, "challenge-manifest": Manifest{}, "challenge-lock": Lock{}, "runner-job": RunnerJob{}, "run-receipt": RunReceipt{}, "validator-result": ValidatorResult{}, "envelope": Envelope{}, "artifact-tree": ArtifactTree{}}

// Schema exports the strict wire shape from the same Go types as server and CLI.
// Relational rules (for example ordered milestones) also require ValidateManifest.
func Schema(name string) (map[string]any, error) {
	value, ok := SchemaTypes[name]
	if !ok {
		return nil, errors.New("unknown protocol schema")
	}
	definitions := map[string]any{}
	var describe func(reflect.Type) map[string]any
	describe = func(t reflect.Type) map[string]any {
		if t.Kind() == reflect.Pointer {
			return describe(t.Elem())
		}
		if t == reflect.TypeOf(time.Time{}) {
			return map[string]any{"type": "string", "format": "date-time"}
		}
		switch t.Kind() {
		case reflect.String:
			return map[string]any{"type": "string", "maxLength": 131072}
		case reflect.Bool:
			return map[string]any{"type": "boolean"}
		case reflect.Int, reflect.Int64:
			return map[string]any{"type": "integer", "minimum": 0, "maximum": 9007199254740991}
		case reflect.Interface:
			return map[string]any{}
		case reflect.Slice:
			if t.Elem().Kind() == reflect.Uint8 {
				return map[string]any{"type": "string", "contentEncoding": "base64", "maxLength": 8192}
			}
			return map[string]any{"type": "array", "items": describe(t.Elem()), "maxItems": 4096}
		case reflect.Map:
			return map[string]any{"type": "object", "additionalProperties": describe(t.Elem()), "maxProperties": 128}
		case reflect.Struct:
			if _, exists := definitions[t.Name()]; !exists {
				definitions[t.Name()] = map[string]any{}
				properties := map[string]any{}
				required := []string{}
				for index := 0; index < t.NumField(); index++ {
					field := t.Field(index)
					tag := field.Tag.Get("json")
					parts := strings.Split(tag, ",")
					if parts[0] == "" || parts[0] == "-" {
						continue
					}
					key := parts[0]
					property := describe(field.Type)
					if !strings.Contains(tag, ",omitempty") {
						required = append(required, key)
					}
					if strings.HasSuffix(key, "Ticks") {
						property = map[string]any{"type": "string", "pattern": `^-?(0|[1-9][0-9]*)$`, "maxLength": 160}
					}
					if strings.HasSuffix(key, "Digest") || key == "digest" {
						property = map[string]any{"type": "string", "pattern": `^sha256:[0-9a-f]{64}$`}
					}
					if key == "apiVersion" {
						property = map[string]any{"const": APIVersion}
					}
					if key == "economicMode" {
						property = map[string]any{"const": "none"}
					}
					if key == "deploymentMode" {
						property = map[string]any{"enum": []string{"controlled-demo", "production"}}
					}
					if key == "verificationPolicy" {
						property = map[string]any{"enum": []string{VerificationPlatform, VerificationIndependent}}
					}
					if key == "verificationStatus" {
						property = map[string]any{"enum": []string{StatusPlatformVerified, StatusIndependentlyReplicated}}
					}
					properties[key] = property
				}
				definitions[t.Name()] = map[string]any{"type": "object", "additionalProperties": false, "properties": properties, "required": required}
			}
			return map[string]any{"$ref": "#/$defs/" + t.Name()}
		}
		return map[string]any{}
	}
	root := describe(reflect.TypeOf(value))
	properties := func(typ string) map[string]any {
		definition, ok := definitions[typ].(map[string]any)
		if !ok {
			return nil
		}
		result, _ := definition["properties"].(map[string]any)
		return result
	}
	set := func(typ, key string, rule map[string]any) {
		if p := properties(typ); p != nil {
			p[key] = rule
		}
	}
	for typ, kind := range map[string]string{"Candidate": "ChallengeCandidate", "Manifest": "ChallengeManifest", "ValidatorResult": "ValidatorResult", "RunnerJob": "ValidationJob", "RunReceipt": "ValidationRunReceipt", "Lock": "ChallengeLockReceipt", "ArtifactTree": "ScienceLadderArtifactTree"} {
		set(typ, "kind", map[string]any{"const": kind})
	}
	set("Candidate", "promptVersion", map[string]any{"enum": []string{ScoutVersion, "1.0.0"}})
	set("Candidate", "disposition", map[string]any{"enum": []string{"viable", "needs_work", "rejected"}})
	if c, ok := definitions["Candidate"].(map[string]any); ok {
		c["allOf"] = []any{map[string]any{"if": map[string]any{"properties": map[string]any{"disposition": map[string]any{"const": "viable"}}}, "then": map[string]any{"required": []string{"manifest"}}}}
	}
	set("Metric", "direction", map[string]any{"enum": []string{"maximize", "minimize"}})
	set("Metric", "quantum", map[string]any{"type": "string", "pattern": `^[+]?(0|[1-9][0-9]*)(\.[0-9]+)?$`, "maxLength": 152})
	set("Validator", "profile", map[string]any{"const": "artifact-checker-v1"})
	set("Suite", "visibility", map[string]any{"enum": []string{"public", "hidden"}})
	set("Source", "url", map[string]any{"type": "string", "format": "uri", "pattern": "^https://"})
	set("Manifest", "slug", map[string]any{"type": "string", "pattern": "^[a-z0-9]+(-[a-z0-9]+)*$", "maxLength": 100})
	set("Manifest", "title", map[string]any{"type": "string", "minLength": 5, "maxLength": 160})
	set("Manifest", "safetyClassification", map[string]any{"enum": []string{"low-risk-computational", "review-required"}})
	set("SubmissionContract", "maxBytes", map[string]any{"type": "integer", "minimum": 1, "maximum": 64 << 20})
	set("SubmissionContract", "maxFiles", map[string]any{"type": "integer", "minimum": 1, "maximum": 4096})
	set("Resources", "class", map[string]any{"enum": []string{"cpu-small", "cpu-medium"}})
	set("Resources", "vCpu", map[string]any{"type": "integer", "minimum": 1, "maximum": 4})
	set("Resources", "memoryMb", map[string]any{"type": "integer", "minimum": 128, "maximum": 8192})
	set("Resources", "timeoutSeconds", map[string]any{"type": "integer", "minimum": 1, "maximum": 600})
	set("Resources", "maxOutputBytes", map[string]any{"type": "integer", "minimum": 1024, "maximum": 65536})
	set("RunnerJob", "purpose", map[string]any{"enum": []string{"preflight", "artifact_prepare", "submission", "confirmation"}})
	set("Envelope", "payloadType", map[string]any{"const": PayloadType})
	set("ArtifactTree", "version", map[string]any{"const": 1})
	set("ArtifactEntry", "type", map[string]any{"const": "file"})
	set("ArtifactEntry", "mode", map[string]any{"const": "0644"})
	return map[string]any{"$schema": "https://json-schema.org/draft/2020-12/schema", "$id": "https://github.com/matbalez/science-ladder/blob/main/protocol/schemas/" + name + "-v1.schema.json", "title": "Science Ladder " + reflect.TypeOf(value).Name() + " v1", "$ref": root["$ref"], "$defs": definitions}, nil
}
