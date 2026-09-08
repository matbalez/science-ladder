//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

func main() {
	root := "protocol/schemas"
	if _, err := os.Stat(root); err != nil {
		root = "../../protocol/schemas"
	}
	for name := range protocol.SchemaTypes {
		for _, version := range []string{"v1", "v2"} {
			if version == "v2" && (name == "artifact-tree" || name == "envelope") {
				continue
			}
			var schema map[string]any
			var err error
			if version == "v1" {
				schema, err = protocol.Schema(name)
			} else {
				schema, err = protocol.SchemaV2(name)
			}
			if err != nil {
				panic(err)
			}
			data, err := json.MarshalIndent(schema, "", "  ")
			if err != nil {
				panic(err)
			}
			if err := os.WriteFile(filepath.Join(root, name+"-"+version+".schema.json"), append(data, '\n'), 0644); err != nil {
				panic(err)
			}
		}
	}
}
