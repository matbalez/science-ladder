package protocol

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	"gopkg.in/yaml.v3"
)

const MaxDocumentBytes = 1 << 20

// CanonicalJSON validates bounded I-JSON input, including duplicate keys, then
// applies RFC 8785. Protocol decimal scores and ticks are strings, never floats.
func CanonicalJSON(data []byte) ([]byte, error) {
	if len(data) == 0 || len(data) > MaxDocumentBytes || !utf8.Valid(data) {
		return nil, errors.New("invalid document size or UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	nodes := 0
	if err := checkJSON(decoder, 0, &nodes, 128*1024); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errors.New("trailing JSON value")
	}
	return jsoncanonicalizer.Transform(data)
}

func checkJSON(decoder *json.Decoder, depth int, nodes *int, stringLimit int) error {
	*nodes++
	if depth > 32 || *nodes > 20000 {
		return errors.New("JSON structure exceeds limits")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	switch value := token.(type) {
	case json.Delim:
		switch value {
		case '{':
			seen := map[string]bool{}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok || seen[key] || len(key) > 1024 {
					return errors.New("duplicate or invalid object key")
				}
				seen[key] = true
				if err := checkJSON(decoder, depth+1, nodes, stringLimit); err != nil {
					return err
				}
			}
			end, err := decoder.Token()
			if err != nil || end != json.Delim('}') {
				return errors.New("unterminated object")
			}
		case '[':
			for decoder.More() {
				if err := checkJSON(decoder, depth+1, nodes, stringLimit); err != nil {
					return err
				}
			}
			end, err := decoder.Token()
			if err != nil || end != json.Delim(']') {
				return errors.New("unterminated array")
			}
		default:
			return errors.New("unexpected JSON delimiter")
		}
	case string:
		if len(value) > stringLimit {
			return errors.New("JSON string exceeds limit")
		}
	case json.Number:
		if len(value) > 100 {
			return errors.New("JSON number exceeds limit")
		}
		if _, err := strconv.ParseFloat(string(value), 64); err != nil {
			return errors.New("JSON number outside finite domain")
		}
		if !strings.ContainsAny(string(value), ".eE") {
			n, ok := new(big.Int).SetString(string(value), 10)
			if !ok || new(big.Int).Abs(n).Cmp(big.NewInt(9007199254740991)) > 0 {
				return errors.New("JSON integers beyond exact I-JSON range must be strings")
			}
		}
	}
	return nil
}

// DecodeStrictBounded validates large snapshot envelopes without canonicalizing their bytes.
// Raw snapshot digests bind their exact encoding; embedded protocol objects are separately canonical.
func DecodeStrictBounded(data []byte, destination any, maximum int) error {
	if maximum < 1 || maximum > 128<<20 || len(data) == 0 || len(data) > maximum || !utf8.Valid(data) {
		return errors.New("snapshot document exceeds limits")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	nodes := 0
	if err := checkJSON(decoder, 0, &nodes, maximum); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("trailing snapshot data")
	}
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	return decoder.Decode(destination)
}

func DecodeStrict(data []byte, destination any) error {
	canonical, err := CanonicalJSON(data)
	if err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(canonical))
	d.DisallowUnknownFields()
	d.UseNumber()
	return d.Decode(destination)
}

func DigestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func Digest(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	data, err = CanonicalJSON(data)
	if err != nil {
		return "", err
	}
	return DigestBytes(data), nil
}
func ValidDigest(value string) bool {
	if len(value) != 71 || value[:7] != "sha256:" {
		return false
	}
	_, err := hex.DecodeString(value[7:])
	return err == nil && value == string(bytes.ToLower([]byte(value)))
}

// AuthoringJSON accepts JSON or the documented strict YAML subset. YAML aliases,
// anchors, merge keys, tags, timestamps, duplicate keys and floats are rejected.
func AuthoringJSON(data []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed) > MaxDocumentBytes || !utf8.Valid(trimmed) {
		return nil, errors.New("invalid authoring document")
	}
	if trimmed[0] == '{' || trimmed[0] == '[' {
		return CanonicalJSON(trimmed)
	}
	var document yaml.Node
	d := yaml.NewDecoder(bytes.NewReader(trimmed))
	if err := d.Decode(&document); err != nil {
		return nil, err
	}
	var extra yaml.Node
	if err := d.Decode(&extra); err != io.EOF {
		return nil, errors.New("exactly one YAML document required")
	}
	nodes := 0
	value, err := yamlValue(&document, 0, &nodes)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return CanonicalJSON(encoded)
}

func yamlValue(n *yaml.Node, depth int, nodes *int) (any, error) {
	*nodes++
	if depth > 32 || *nodes > 20000 {
		return nil, errors.New("YAML exceeds structure limit")
	}
	if n.Anchor != "" || n.Kind == yaml.AliasNode || n.Style&yaml.TaggedStyle != 0 {
		return nil, errors.New("YAML anchors, aliases and explicit tags forbidden")
	}
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) != 1 {
			return nil, errors.New("invalid YAML document")
		}
		return yamlValue(n.Content[0], depth+1, nodes)
	case yaml.MappingNode:
		result := map[string]any{}
		for i := 0; i < len(n.Content); i += 2 {
			key := n.Content[i]
			if key.Tag != "!!str" || key.Anchor != "" || len(key.Value) > 1024 || key.Value == "<<" {
				return nil, errors.New("invalid YAML key")
			}
			if _, ok := result[key.Value]; ok {
				return nil, fmt.Errorf("duplicate YAML key %q", key.Value)
			}
			value, err := yamlValue(n.Content[i+1], depth+1, nodes)
			if err != nil {
				return nil, err
			}
			result[key.Value] = value
		}
		return result, nil
	case yaml.SequenceNode:
		result := []any{}
		for _, child := range n.Content {
			value, err := yamlValue(child, depth+1, nodes)
			if err != nil {
				return nil, err
			}
			result = append(result, value)
		}
		return result, nil
	case yaml.ScalarNode:
		if len(n.Value) > 128*1024 {
			return nil, errors.New("YAML scalar exceeds limit")
		}
		switch n.Tag {
		case "!!str":
			return n.Value, nil
		case "!!null":
			return nil, nil
		case "!!bool":
			if n.Value == "true" {
				return true, nil
			}
			if n.Value == "false" {
				return false, nil
			}
			return nil, errors.New("canonical true/false required")
		case "!!int":
			v, err := strconv.ParseInt(n.Value, 10, 64)
			if err != nil || strconv.FormatInt(v, 10) != n.Value {
				return nil, errors.New("canonical decimal integer required")
			}
			return v, nil
		default:
			return nil, fmt.Errorf("YAML tag %s forbidden; quote decimal scores and timestamps", n.Tag)
		}
	}
	return nil, errors.New("unsupported YAML node")
}

func ParseCandidate(data []byte) (Candidate, error) {
	var value Candidate
	j, err := AuthoringJSON(data)
	if err != nil {
		return value, err
	}
	if err = DecodeStrict(j, &value); err != nil {
		return value, err
	}
	return value, ValidateCandidate(value)
}
func ParseManifest(data []byte) (Manifest, error) {
	var value Manifest
	j, err := AuthoringJSON(data)
	if err != nil {
		return value, err
	}
	if err = DecodeStrict(j, &value); err != nil {
		return value, err
	}
	return value, ValidateManifest(value)
}
