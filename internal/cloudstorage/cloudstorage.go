package cloudstorage

import (
	"fmt"
	"path/filepath"
)

// CloudProvider represents different cloud storage services
type CloudProvider string

const (
	GoogleDrive CloudProvider = "gdrive"
	Dropbox     CloudProvider = "dropbox"
	OneDrive    CloudProvider = "onedrive"
	MEGACloud   CloudProvider = "mega"
	IPFS        CloudProvider = "ipfs"
	Local       CloudProvider = "local"
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
	Providers        []CloudProvider `json:"providers"`
	ReplicationCount int             `json:"replication_count"` // How many copies per chunk
	LoadBalancing    string          `json:"load_balancing"`    // "round_robin", "random", "size_based"
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
		ReplicationCount: 1,
		LoadBalancing:    "round_robin",
	}
}

// CustomCloudStrategy creates a strategy with specific providers
func CustomCloudStrategy(providers []CloudProvider) CloudDistributionStrategy {
	if len(providers) == 0 {
		// Default to Google Drive only if no providers specified
		providers = []CloudProvider{GoogleDrive}
	}
	
	return CloudDistributionStrategy{
		Providers:        providers,
		ReplicationCount: 1,
		LoadBalancing:    "round_robin",
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
