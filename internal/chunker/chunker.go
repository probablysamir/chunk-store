package chunker

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/probablysamir/chunk-store/internal/encryption"
	"github.com/probablysamir/chunk-store/internal/manifest"
	"github.com/schollz/progressbar/v3"
)

const DefaultChunkSize = 1 * 1024 * 1024

func SplitFile(path, outDir, manifestPath string, encConfig *encryption.EncryptionConfig) error {
	return SplitFileWithChunkSize(path, outDir, manifestPath, encConfig, DefaultChunkSize)
}

func SplitFileWithChunkSize(path, outDir, manifestPath string, encConfig *encryption.EncryptionConfig, chunkSize int64) error {
	inFile, err := os.Open(path)
	if err != nil {
		return err
	}
	defer inFile.Close()

	// Get file info for progress bar
	fileInfo, err := inFile.Stat()
	if err != nil {
		return err
	}
	fileSize := fileInfo.Size()

	// Create progress bar
	bar := progressbar.NewOptions64(fileSize,
		progressbar.OptionSetDescription("Splitting file..."),
		progressbar.OptionSetWidth(50),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			fmt.Println("\nSplitting done!")
		}),
	)

	os.MkdirAll(outDir, 0755)
	var chunks []manifest.ChunkInfo
	buf := make([]byte, chunkSize)
	index := 0

	for {
		n, err := inFile.Read(buf)
		// If entire file is read
		if n == 0 && err == io.EOF {
			break
		}

		if err != nil && err != io.EOF {
			return err
		}

		data := buf[:n]
		bar.Add(len(data))

		// Encrypt if needed
		encryptedData, err := encConfig.Encrypt(data)
		if err != nil {
			return fmt.Errorf("failed to encrypt chunk: %w", err)
		}

		// Hash the original data
		hash := sha256.Sum256(data)
		id := fmt.Sprintf("%x", hash[:8])

		chunkPath := filepath.Join(outDir, id+".chunk")
		err = os.WriteFile(chunkPath, encryptedData, 0644)
		if err != nil {
			return err
		}

		chunks = append(chunks, manifest.ChunkInfo{
			ID:         id,
			Hash:       fmt.Sprintf("%x", hash[:]),
			Index:      index,
			Encrypted:  encConfig.Enabled,
			Size:       int64(len(encryptedData)),
			CloudPaths: []string{}, // Will be populated when uploaded to cloud
			Providers:  []string{}, // Will be populated when uploaded to cloud
		})
		index++
	}
	return manifest.WriteManifest(chunks, manifestPath, filepath.Base(path), encConfig.Enabled)
}

func AssembleFile(manifestPath, chunksPath, outputPath string, encConfig *encryption.EncryptionConfig) error {
	m, err := manifest.ReadManifest(manifestPath)
	if err != nil {
		return err
	}

	// Check if encryption settings match
	if m.Encrypted && !encConfig.Enabled {
		return fmt.Errorf("file was encrypted but no decryption key provided")
	}
	if !m.Encrypted && encConfig.Enabled {
		return fmt.Errorf("file was not encrypted but decryption key provided")
	}

	// Sorting manifest json before fetching data
	sort.Slice(m.Chunks, func(i, j int) bool {
		return m.Chunks[i].Index < m.Chunks[j].Index
	})

	// Create progress bar for assembly
	bar := progressbar.NewOptions(len(m.Chunks),
		progressbar.OptionSetDescription("Assembling chunks into file..."),
		progressbar.OptionSetWidth(50),
		progressbar.OptionShowCount(),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionOnCompletion(func() {
			fmt.Println("\nFile assembly completed!")
		}),
	)

	// Make sure output directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	for _, c := range m.Chunks {
		chunkPath := filepath.Join(chunksPath, c.ID+".chunk")
		encryptedData, err := os.ReadFile(chunkPath)
		if err != nil {
			return err
		}

		// Decrypt if needed
		data, err := encConfig.Decrypt(encryptedData)
		if err != nil {
			return fmt.Errorf("failed to decrypt chunk %s: %w", c.ID, err)
		}

		// Verify hash matches
		hash := sha256.Sum256(data)
		hexHash := fmt.Sprintf("%x", hash[:])
		if c.Hash != hexHash {
			return fmt.Errorf("hash mismatch on chunk id: %s", c.ID)
		}

		_, err = outFile.Write(data)
		if err != nil {
			return err
		}

		bar.Add(1)
	}
	return nil
}

// CleanupChunks removes all chunk files from the specified directory
func CleanupChunks(chunksPath string) error {
	entries, err := os.ReadDir(chunksPath)
	if err != nil {
		return fmt.Errorf("failed to read chunks directory: %w", err)
	}

	var deletedCount int
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".chunk" {
			chunkPath := filepath.Join(chunksPath, entry.Name())
			if err := os.Remove(chunkPath); err != nil {
				return fmt.Errorf("failed to remove chunk %s: %w", entry.Name(), err)
			}
			deletedCount++
		}
	}

	fmt.Printf("Cleaned up %d chunk files\n", deletedCount)
	return nil
}
