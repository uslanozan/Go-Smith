// scripts/generate_schema.go

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"github.com/invopop/jsonschema"
	"github.com/uslanozan/Go-Smith/models"
)

type SharedDTOs struct {
	Request        models.OrchestratorTaskRequest `json:"request"`
	StartResponse  models.TaskStartResponse       `json:"start_response"`
	StatusResponse models.TaskStatusResponse      `json:"status_response"`
}

func main() {
	r := new(jsonschema.Reflector)
	r.ExpandedStruct = true 
	schema := r.Reflect(&SharedDTOs{})

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		panic(err)
	}

	outputDir := "schemas"
	outputFile := filepath.Join(outputDir, "task_schema.json")

	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		os.Mkdir(outputDir, os.ModePerm)
	}

	err = os.WriteFile(outputFile, data, 0644)
	if err != nil {
		panic(err)
	}

	absPath, _ := filepath.Abs(outputFile)
	fmt.Println("✅ Schema başarıyla oluşturuldu:")
	fmt.Println("📂 Konum:", absPath)
}