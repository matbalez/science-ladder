//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/matbalez/science-ladder/pkg/protocol"
)

func main(){root:="protocol/schemas";if _,err:=os.Stat(root);err!=nil{root="../../protocol/schemas"};for name:=range protocol.SchemaTypes{schema,err:=protocol.Schema(name);if err!=nil{panic(err)};data,err:=json.MarshalIndent(schema,"","  ");if err!=nil{panic(err)};if err:=os.WriteFile(filepath.Join(root,name+"-v1.schema.json"),append(data,'\n'),0644);err!=nil{panic(err)}}}
