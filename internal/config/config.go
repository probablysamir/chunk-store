package config

import (
	"encoding/json"
	"fmt"
	"os"
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

// GoogleDriveAccount represents a single Google Drive account configuration
type GoogleDriveAccount struct {
	Name        string `json:"name"`        // User-friendly name for the account
	CredsFile   string `json:"creds_file"`  // Path to credentials.json
	TokenFile   string `json:"token_file"`  // Path to token.json
	FolderName  string `json:"folder_name"` // Custom folder name (optional)
	Enabled     bool   `json:"enabled"`     // Whether this account is active
	Description string `json:"description"` // Optional description
}

// CloudConfig contains cloud storage configuration
type CloudConfig struct {
	GoogleDriveAccounts []GoogleDriveAccount `json:"google_drive_accounts"`
	Providers           []CloudProvider      `json:"providers"`
	ReplicationCount    int                  `json:"replication_count"`
	LoadBalancing       string               `json:"load_balancing"`
	// Future provider configurations will be added here as they are implemented
	// DropboxAccounts     []DropboxAccount     `json:"dropbox_accounts,omitempty"`
	// OneDriveAccounts    []OneDriveAccount    `json:"onedrive_accounts,omitempty"`
	// MEGAAccounts        []MEGAAccount        `json:"mega_accounts,omitempty"`
	// IPFSAccounts        []IPFSAccount        `json:"ipfs_accounts,omitempty"`
}

// ChunkConfig holds chunking configuration
type ChunkConfig struct {
	ChunkSize int64 `json:"chunk_size"` // Size in bytes (default: 1MB)
}

// Config represents the main configuration structure
type Config struct {
	ChunkConfig ChunkConfig `json:"chunk_config"`
	CloudConfig CloudConfig `json:"cloud_config"`
	Version     string      `json:"version"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		ChunkConfig: ChunkConfig{
			ChunkSize: 1 * 1024 * 1024, // 1MB
		},
		CloudConfig: CloudConfig{
			GoogleDriveAccounts: []GoogleDriveAccount{
				{
					Name:        "primary",
					CredsFile:   "credentials.json",
					TokenFile:   "token.json",
					FolderName:  "distributed-chunks",
					Enabled:     true,
					Description: "Primary Google Drive account",
				},
			},
			Providers:        []CloudProvider{GoogleDrive},
			ReplicationCount: 1,
			LoadBalancing:    "round_robin",
		},
		Version: "1.0",
	}
}

// LoadConfig loads configuration from a file
func LoadConfig(configPath string) (*Config, error) {
	// If config file doesn't exist, create default
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Printf("Config file not found, creating default config at %s\n", configPath)
		config := DefaultConfig()
		err := SaveConfig(config, configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		return config, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate configuration
	err = config.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// SaveConfig saves configuration to a file
func SaveConfig(config *Config, configPath string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(configPath)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	err = os.WriteFile(configPath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validate chunk size
	if c.ChunkConfig.ChunkSize <= 0 {
		return fmt.Errorf("chunk size must be positive")
	}

	// Validate replication count
	if c.CloudConfig.ReplicationCount < 1 {
		return fmt.Errorf("replication count must be at least 1")
	}

	// Validate load balancing strategy
	validStrategies := map[string]bool{
		"round_robin": true,
		"random":      true,
		"size_based":  true,
	}
	if !validStrategies[c.CloudConfig.LoadBalancing] {
		return fmt.Errorf("invalid load balancing strategy: %s", c.CloudConfig.LoadBalancing)
	}

	// Validate Google Drive accounts (only implemented provider for now)
	accountNames := make(map[string]bool)
	for i, account := range c.CloudConfig.GoogleDriveAccounts {
		if account.Name == "" {
			return fmt.Errorf("google drive account %d: name cannot be empty", i)
		}
		if accountNames[account.Name] {
			return fmt.Errorf("duplicate google drive account name: %s", account.Name)
		}
		accountNames[account.Name] = true

		if account.CredsFile == "" {
			return fmt.Errorf("google drive account %s: credentials file cannot be empty", account.Name)
		}
		if account.TokenFile == "" {
			return fmt.Errorf("google drive account %s: token file cannot be empty", account.Name)
		}
	}

	// Validate that enabled providers have corresponding account configurations
	for _, provider := range c.CloudConfig.Providers {
		switch provider {
		case GoogleDrive:
			if len(c.GetEnabledGoogleDriveAccounts()) == 0 {
				return fmt.Errorf("google drive provider is enabled but no accounts are configured")
			}
		case Dropbox, OneDrive, MEGACloud, IPFS:
			return fmt.Errorf("provider %s is not yet implemented", provider)
		default:
			return fmt.Errorf("unknown provider: %s", provider)
		}
	}

	return nil
}

// GetEnabledGoogleDriveAccounts returns only the enabled Google Drive accounts
func (c *Config) GetEnabledGoogleDriveAccounts() []GoogleDriveAccount {
	var enabled []GoogleDriveAccount
	for _, account := range c.CloudConfig.GoogleDriveAccounts {
		if account.Enabled {
			enabled = append(enabled, account)
		}
	}
	return enabled
}

// HasGoogleDriveProvider checks if Google Drive is in the providers list
func (c *Config) HasGoogleDriveProvider() bool {
	for _, provider := range c.CloudConfig.Providers {
		if provider == GoogleDrive {
			return true
		}
	}
	return false
}

// GetTotalEnabledAccounts returns the total number of enabled accounts across all providers
func (c *Config) GetTotalEnabledAccounts() int {
	total := 0
	total += len(c.GetEnabledGoogleDriveAccounts())
	// Future: add other providers when implemented
	// total += len(c.GetEnabledDropboxAccounts())
	// total += len(c.GetEnabledOneDriveAccounts())
	// etc.
	return total
}

// GetEnabledProvidersCount returns the number of different provider types that are enabled
func (c *Config) GetEnabledProvidersCount() int {
	count := 0
	if len(c.GetEnabledGoogleDriveAccounts()) > 0 {
		count++
	}
	// Future: add checks for other providers when implemented
	return count
}
