package cloudstorage

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/probablysamir/chunk-store/internal/manifest"
	"github.com/schollz/progressbar/v3"
)

// CloudUploader handles uploading chunks to cloud services
type CloudUploader struct {
	Strategy    CloudDistributionStrategy
	googleDrive *GoogleDriveClient
}

// NewCloudUploader creates uploader with strategy and credentials
func NewCloudUploader(strategy CloudDistributionStrategy, credsFile, tokenFile string) (*CloudUploader, error) {
	uploader := &CloudUploader{
		Strategy: strategy,
	}

	// Set up Google Drive if needed
	for _, provider := range strategy.Providers {
		if provider == GoogleDrive {
			gdrive, err := NewGoogleDriveClient(credsFile, tokenFile)
			if err != nil {
				return nil, fmt.Errorf("failed to create Google Drive client: %w", err)
			}

			err = gdrive.Initialize()
			if err != nil {
				return nil, fmt.Errorf("failed to initialize Google Drive: %w", err)
			}

			uploader.googleDrive = gdrive
			break
		}
	}

	return uploader, nil
}

// UploadChunks uploads all chunks from local storage to cloud services
func (cu *CloudUploader) UploadChunks(localChunksDir, manifestPath string) error {
	// Read the current manifest
	m, err := manifest.ReadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	// Create progress bar for uploads
	bar := progressbar.NewOptions(len(m.Chunks),
		progressbar.OptionSetDescription("Uploading to cloud..."),
		progressbar.OptionSetWidth(50),
		progressbar.OptionShowCount(),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionOnCompletion(func() {
			fmt.Println("\nUpload done!")
		}),
	)

	// Upload each chunk to designated cloud services
	for i, chunk := range m.Chunks {
		destinations := cu.Strategy.GetChunkDestination(chunk.Index)

		// Local chunk path
		localPath := filepath.Join(localChunksDir, chunk.ID+".chunk")

		var cloudPaths []string
		var providers []string
		cloudIDs := make(map[string]string)

		for _, provider := range destinations {
			cloudPath := GenerateCloudPath(provider, chunk.ID)

			var fileID string
			var err error

			switch provider {
			case GoogleDrive:
				if cu.googleDrive != nil {
					fileID, err = cu.uploadToGoogleDriveWithID(localPath, cloudPath)
				} else {
					err = fmt.Errorf("google Drive client not initialized")
				}
			case Dropbox:
				err = fmt.Errorf("dropbox not implemented yet")
			case OneDrive:
				err = fmt.Errorf("oneDrive not implemented yet")
			case MEGACloud:
				err = fmt.Errorf("mEGA not implemented yet")
			case IPFS:
				err = fmt.Errorf("iPFS not implemented yet")
			default:
				err = fmt.Errorf("unsupported cloud provider: %s", provider)
			}

			if err != nil {
				fmt.Printf("⚠️  Failed to upload chunk %s to %s: %v\n", chunk.ID, provider, err)
				continue
			}

			cloudPaths = append(cloudPaths, cloudPath)
			providers = append(providers, string(provider))

			// Store file ID if available
			if fileID != "" {
				cloudIDs[string(provider)] = fileID
			}
		}

		// Update chunk info with cloud details
		m.Chunks[i].CloudPaths = cloudPaths
		m.Chunks[i].Providers = providers
		m.Chunks[i].UploadTime = time.Now().Format(time.RFC3339)
		if len(cloudIDs) > 0 {
			m.Chunks[i].CloudIDs = cloudIDs
		}

		// Update progress bar
		bar.Add(1)
	}

	// Update distribution mode and save manifest
	return manifest.WriteManifestWithMode(m.Chunks, manifestPath, m.OriginalName, m.Encrypted, "cloud")
}

func (cu *CloudUploader) uploadToGoogleDriveWithID(localPath, cloudPath string) (string, error) {
	if cu.googleDrive == nil {
		return "", fmt.Errorf("google Drive not initialized - check credentials")
	}

	// Real Google Drive upload
	fileID, err := cu.googleDrive.UploadFile(localPath, cloudPath)
	if err != nil {
		return "", fmt.Errorf("google Drive upload failed: %w", err)
	}

	return fileID, nil
}

// DownloadChunks downloads chunks from cloud services for assembly
func (cu *CloudUploader) DownloadChunks(manifestPath, downloadDir string) error {
	m, err := manifest.ReadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	// Create progress bar for downloads
	bar := progressbar.NewOptions(len(m.Chunks),
		progressbar.OptionSetDescription("Downloading from cloud..."),
		progressbar.OptionSetWidth(50),
		progressbar.OptionShowCount(),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionOnCompletion(func() {
			fmt.Println("\nDownload done!")
		}),
	)

	// Create download directory
	os.MkdirAll(downloadDir, 0755)

	for _, chunk := range m.Chunks {
		if len(chunk.CloudPaths) == 0 {
			return fmt.Errorf("chunk %s has no cloud paths", chunk.ID)
		}

		// Try to download from the first available provider
		for i, cloudPath := range chunk.CloudPaths {
			if i >= len(chunk.Providers) {
				break
			}

			provider := CloudProvider(chunk.Providers[i])
			localPath := filepath.Join(downloadDir, chunk.ID+".chunk")

			var err error
			switch provider {
			case GoogleDrive:
				if cu.googleDrive != nil {
					// Use real Google Drive download with file ID
					if chunk.CloudIDs != nil {
						if fileID, exists := chunk.CloudIDs[string(provider)]; exists {
							err = cu.googleDrive.DownloadFile(fileID, localPath)
						} else {
							// Fallback: try to find file by name
							fileName := filepath.Base(cloudPath)
							fileID, findErr := cu.googleDrive.FindFileByName(fileName)
							if findErr == nil {
								err = cu.googleDrive.DownloadFile(fileID, localPath)
							} else {
								err = findErr
							}
						}
					} else {
						// Fallback: try to find file by name
						fileName := filepath.Base(cloudPath)
						fileID, findErr := cu.googleDrive.FindFileByName(fileName)
						if findErr == nil {
							err = cu.googleDrive.DownloadFile(fileID, localPath)
						} else {
							err = findErr
						}
					}
				} else {
					err = fmt.Errorf("google Drive client not initialized")
				}
			case Dropbox:
				err = fmt.Errorf("dropbox not implemented yet")
			case OneDrive:
				err = fmt.Errorf("oneDrive not implemented yet")
			case MEGACloud:
				err = fmt.Errorf("mega not implemented yet")
			case IPFS:
				err = fmt.Errorf("ipfs not implemented yet")
			default:
				err = fmt.Errorf("unsupported cloud provider: %s", provider)
			}

			if err != nil {
				fmt.Printf("Failed to download chunk %s from %s: %v\n", chunk.ID, provider, err)
				continue
			}

			break // Successfully downloaded, move to next chunk
		}

		// Update progress bar
		bar.Add(1)
	}

	return nil
}
