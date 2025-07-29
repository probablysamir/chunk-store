package cloudstorage

import (
	"fmt"
	"path/filepath"

	"github.com/probablysamir/chunk-store/internal/config"
)

// CloudProvider type is imported from config package
type CloudProvider = config.CloudProvider

// Cloud provider constants
const (
	GoogleDrive = config.GoogleDrive
	Dropbox     = config.Dropbox
	OneDrive    = config.OneDrive
	MEGACloud   = config.MEGACloud
	IPFS        = config.IPFS
	Local       = config.Local
)

// CloudChunkInfo extends chunk info with cloud storage details
type CloudChunkInfo struct {
	ID          string        `json:"id"`
	Hash        string        `json:"hash"`
	Index       int           `json:"index"`
	Provider    CloudProvider `json:"provider"`
	CloudPath   string        `json:"cloud_path"`   // Path/ID in cloud service
	BackupPaths []string      `json:"backup_paths"` // Backup locations
	Size        int64         `json:"size"`
	UploadTime  string        `json:"upload_time"`
	Encrypted   bool          `json:"encrypted"`
}

// CloudDistributionStrategy defines how to distribute chunks
type CloudDistributionStrategy struct {
	Providers           []CloudProvider `json:"providers"`
	ReplicationCount    int             `json:"replication_count"` // How many copies per chunk
	LoadBalancing       string          `json:"load_balancing"`    // "round_robin", "random", "size_based"
	GoogleDriveAccounts int             `json:"google_drive_accounts"` // Number of Google Drive accounts to cycle through
}

// DefaultCloudStrategy returns a basic distribution strategy
func DefaultCloudStrategy() CloudDistributionStrategy {
	return CloudDistributionStrategy{
		Providers: []CloudProvider{
			GoogleDrive,
			Dropbox,
			OneDrive,
			MEGACloud,
		},
		ReplicationCount:    1,
		LoadBalancing:       "round_robin",
		GoogleDriveAccounts: 1,
	}
}

// CustomCloudStrategy creates a strategy with specific providers
func CustomCloudStrategy(providers []CloudProvider) CloudDistributionStrategy {
	if len(providers) == 0 {
		// Default to Google Drive only if no providers specified
		providers = []CloudProvider{GoogleDrive}
	}
	
	return CloudDistributionStrategy{
		Providers:           providers,
		ReplicationCount:    1,
		LoadBalancing:       "round_robin",
		GoogleDriveAccounts: 1,
	}
}

// CustomCloudStrategyWithAccounts creates a strategy with specific providers and Google Drive account count
func CustomCloudStrategyWithAccounts(providers []CloudProvider, gdriveAccounts int) CloudDistributionStrategy {
	if len(providers) == 0 {
		providers = []CloudProvider{GoogleDrive}
	}
	
	return CloudDistributionStrategy{
		Providers:           providers,
		ReplicationCount:    1,
		LoadBalancing:       "round_robin",
		GoogleDriveAccounts: gdriveAccounts,
	}
}

// GoogleDriveOnlyStrategy creates a strategy that only uses Google Drive
func GoogleDriveOnlyStrategy() CloudDistributionStrategy {
	return CustomCloudStrategy([]CloudProvider{GoogleDrive})
}

// GetChunkDestination determines where to store a chunk based on strategy
func (cds *CloudDistributionStrategy) GetChunkDestination(chunkIndex int) []CloudProvider {
	if len(cds.Providers) == 0 {
		return []CloudProvider{Local}
	}

	destinations := make([]CloudProvider, 0, cds.ReplicationCount)

	switch cds.LoadBalancing {
	case "round_robin":
		for i := 0; i < cds.ReplicationCount; i++ {
			providerIndex := (chunkIndex + i) % len(cds.Providers)
			destinations = append(destinations, cds.Providers[providerIndex])
		}
	case "random":
		// Implementation for random distribution would go here
		fallthrough
	default:
		// Default to round-robin
		for i := 0; i < cds.ReplicationCount; i++ {
			providerIndex := (chunkIndex + i) % len(cds.Providers)
			destinations = append(destinations, cds.Providers[providerIndex])
		}
	}

	return destinations
}

// GenerateCloudPath creates a cloud-safe path for the chunk
func GenerateCloudPath(provider CloudProvider, chunkID string) string {
	switch provider {
	case GoogleDrive:
		return fmt.Sprintf("distributed-chunks/%s.chunk", chunkID)
	case Dropbox:
		return fmt.Sprintf("/Apps/DistributedChunks/%s.chunk", chunkID)
	case OneDrive:
		return fmt.Sprintf("DistributedChunks/%s.chunk", chunkID)
	case MEGACloud:
		return fmt.Sprintf("chunks/%s.chunk", chunkID)
	case IPFS:
		return chunkID // IPFS uses content-based addressing
	default:
		return filepath.Join("chunks", chunkID+".chunk")
	}
}
