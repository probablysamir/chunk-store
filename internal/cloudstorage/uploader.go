package cloudstorage

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/probablysamir/chunk-store/internal/config"
	"github.com/probablysamir/chunk-store/internal/manifest"
	"github.com/schollz/progressbar/v3"
)

// CloudUploader handles uploading chunks to cloud services
type CloudUploader struct {
	Strategy       CloudDistributionStrategy
	googleDrives   map[string]*GoogleDriveClient // Map of account name to client
	config         *config.Config
}

// CreateCloudUploader creates uploader with configuration
func CreateCloudUploader(strategy CloudDistributionStrategy, cfg *config.Config) (*CloudUploader, error) {
	uploader := &CloudUploader{
		Strategy:     strategy,
		googleDrives: make(map[string]*GoogleDriveClient),
		config:       cfg,
	}

	// Set up Google Drive clients if needed
	if cfg.HasGoogleDriveProvider() {
		for _, account := range cfg.GetEnabledGoogleDriveAccounts() {
			gdrive, err := CreateGoogleDriveClientWithName(
				account.CredsFile,
				account.TokenFile,
				account.Name,
				account.FolderName,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to create Google Drive client for account '%s': %w", account.Name, err)
			}

			err = gdrive.Initialize()
			if err != nil {
				return nil, fmt.Errorf("failed to initialize Google Drive for account '%s': %w", account.Name, err)
			}

			uploader.googleDrives[account.Name] = gdrive
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
			var accountName string

			switch provider {
			case GoogleDrive:
				// Select Google Drive account based on chunk index
				accountName, fileID, err = cu.uploadToGoogleDriveMultiAccount(localPath, cloudPath, chunk.Index)
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
				// Also store account name for Google Drive
				if accountName != "" {
					cloudIDs[string(provider)+"_account"] = accountName
				}
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

// uploadToGoogleDriveMultiAccount uploads to one of the available Google Drive accounts using round-robin
func (cu *CloudUploader) uploadToGoogleDriveMultiAccount(localPath, cloudPath string, chunkIndex int) (string, string, error) {
	if len(cu.googleDrives) == 0 {
		return "", "", fmt.Errorf("no Google Drive clients initialized - check credentials")
	}

	// Get list of account names for round-robin selection
	var accountNames []string
	for name := range cu.googleDrives {
		accountNames = append(accountNames, name)
	}

	// Select account based on chunk index (round-robin)
	selectedAccount := accountNames[chunkIndex%len(accountNames)]
	client := cu.googleDrives[selectedAccount]

	// Upload to the selected account
	fileID, err := client.UploadFile(localPath, cloudPath)
	if err != nil {
		return "", "", fmt.Errorf("google Drive upload failed to account '%s': %w", selectedAccount, err)
	}

	return selectedAccount, fileID, nil
}

func (cu *CloudUploader) uploadToGoogleDriveWithID(localPath, cloudPath string) (string, error) {
	// Legacy method for backward compatibility
	accountName, fileID, err := cu.uploadToGoogleDriveMultiAccount(localPath, cloudPath, 0)
	if err != nil {
		return "", err
	}
	_ = accountName // Ignore account name in legacy method
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
				if len(cu.googleDrives) > 0 {
					// Try to get the specific account used for this chunk
					var targetClient *GoogleDriveClient
					if chunk.CloudIDs != nil {
						if accountName, exists := chunk.CloudIDs[string(provider)+"_account"]; exists {
							if client, found := cu.googleDrives[accountName]; found {
								targetClient = client
							}
						}
					}

					// If no specific account found, use the first available
					if targetClient == nil {
						for _, client := range cu.googleDrives {
							targetClient = client
							break
						}
					}

					// Use real Google Drive download with file ID
					if chunk.CloudIDs != nil {
						if fileID, exists := chunk.CloudIDs[string(provider)]; exists {
							err = targetClient.DownloadFile(fileID, localPath)
						} else {
							// Fallback: try to find file by name
							fileName := filepath.Base(cloudPath)
							fileID, findErr := targetClient.FindFileByName(fileName)
							if findErr == nil {
								err = targetClient.DownloadFile(fileID, localPath)
							} else {
								err = findErr
							}
						}
					} else {
						// Fallback: try to find file by name
						fileName := filepath.Base(cloudPath)
						fileID, findErr := targetClient.FindFileByName(fileName)
						if findErr == nil {
							err = targetClient.DownloadFile(fileID, localPath)
						} else {
							err = findErr
						}
					}
				} else {
					err = fmt.Errorf("no Google Drive clients initialized")
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
