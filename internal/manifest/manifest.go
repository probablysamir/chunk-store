package manifest

import (
	"encoding/json"
	"os"
	"time"
)

type ChunkInfo struct {
	ID         string            `json:"id"`
	Hash       string            `json:"hash"`
	Index      int               `json:"index"`
	Encrypted  bool              `json:"encrypted"`
	CloudPaths []string          `json:"cloud_paths"` // Multiple cloud storage paths
	Providers  []string          `json:"providers"`   // Cloud providers storing this chunk
	Size       int64             `json:"size"`
	UploadTime string            `json:"upload_time"`
	CloudIDs   map[string]string `json:"cloud_ids,omitempty"` // Map of provider -> file ID (e.g., "gdrive" -> "1ABC123...")
}

type Manifest struct {
	OriginalName     string      `json:"original_name"`
	Chunks           []ChunkInfo `json:"chunks"`
	Encrypted        bool        `json:"encrypted"`
	CreatedTime      string      `json:"created_time"`
	TotalSize        int64       `json:"total_size"`
	ChunkCount       int         `json:"chunk_count"`
	DistributionMode string      `json:"distribution_mode"` // "local", "cloud", "hybrid"
}

func WriteManifest(chunks []ChunkInfo, path string, original string, encrypted bool) error {
	return WriteManifestWithMode(chunks, path, original, encrypted, "local")
}

func WriteManifestWithMode(chunks []ChunkInfo, path string, original string, encrypted bool, distributionMode string) error {
	m := Manifest{
		OriginalName:     original,
		Chunks:           chunks,
		Encrypted:        encrypted,
		CreatedTime:      time.Now().Format(time.RFC3339),
		ChunkCount:       len(chunks),
		DistributionMode: distributionMode,
	}

	// Calculate total size
	var totalSize int64
	for _, chunk := range chunks {
		totalSize += chunk.Size
	}
	m.TotalSize = totalSize

	data, err := json.MarshalIndent(m, "", "	")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func ReadManifest(path string) (Manifest, error) {
	var m Manifest

	data, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}

	err = json.Unmarshal(data, &m)
	return m, err
}
