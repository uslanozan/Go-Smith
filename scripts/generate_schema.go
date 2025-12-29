// scripts/generate_schema.go

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath" // Windows/Mac uyumu için eklendi

	"github.com/invopop/jsonschema"
	// Modül adının go.mod dosyanla aynı olduğundan emin ol
	"github.com/uslanozan/Go-Smith/models"
)

// SharedDTOs: Request ve Response yapılarını tek çatı altında toplar
type SharedDTOs struct {
	Request        models.OrchestratorTaskRequest `json:"request"`
	StartResponse  models.TaskStartResponse       `json:"start_response"`
	StatusResponse models.TaskStatusResponse      `json:"status_response"`
}

func main() {
	r := new(jsonschema.Reflector)
	// Enum değerlerini (pending, running) string olarak basar
	r.ExpandedStruct = true 

	// Tüm yapıları kapsayan DTO'yu reflect ediyoruz
	schema := r.Reflect(&SharedDTOs{})

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		panic(err)
	}

	// DÜZELTME: Dosya yolunu işletim sistemine uygun hale getirdik.
	// Hedef: ProjeAnaDizini/schemas/task_schema.json
	outputDir := "schemas"
	outputFile := filepath.Join(outputDir, "task_schema.json")

	// Klasör yoksa oluştur (Root dizinde schemas klasörü arar)
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		os.Mkdir(outputDir, os.ModePerm)
	}

	err = os.WriteFile(outputFile, data, 0644)
	if err != nil {
		panic(err)
	}

	// Çalıştığı yolu göstermek için
	absPath, _ := filepath.Abs(outputFile)
	fmt.Println("✅ Schema başarıyla oluşturuldu:")
	fmt.Println("📂 Konum:", absPath)
}