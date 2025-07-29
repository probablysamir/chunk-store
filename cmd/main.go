package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"syscall"

	"github.com/probablysamir/chunk-store/internal/chunker"
	"github.com/probablysamir/chunk-store/internal/cloudstorage"
	"github.com/probablysamir/chunk-store/internal/config"
	"github.com/probablysamir/chunk-store/internal/encryption"
	"golang.org/x/term"
)

// parseCloudProviders converts comma-separated provider string to slice
func parseCloudProviders(providersStr string) []config.CloudProvider {
	if providersStr == "" {
		return []config.CloudProvider{config.GoogleDrive}
	}

	providerNames := strings.Split(providersStr, ",")
	var providers []config.CloudProvider

	for _, name := range providerNames {
		name = strings.TrimSpace(name)
		switch strings.ToLower(name) {
		case "gdrive", "googledrive", "google-drive":
			providers = append(providers, config.GoogleDrive)
		case "dropbox":
			providers = append(providers, config.Dropbox)
		case "onedrive", "one-drive":
			providers = append(providers, config.OneDrive)
		case "mega":
			providers = append(providers, config.MEGACloud)
		case "ipfs":
			providers = append(providers, config.IPFS)
		default:
			fmt.Printf("Warning: Unknown provider '%s', ignoring\n", name)
		}
	}

	if len(providers) == 0 {
		providers = []config.CloudProvider{config.GoogleDrive}
	}

	return providers
}

func main() {
	mode := flag.String("mode", "", "split or assemble")
	input := flag.String("in", "", "input file path")
	out := flag.String("out", "", "output directory or file")
	manifestPath := flag.String("manifest", "manifest.json", "manifest file path")
	chunksPath := flag.String("chunkspath", "chunks", "chunks file path")
	encrypt := flag.Bool("encrypt", false, "enable encryption for split mode")
	decrypt := flag.Bool("decrypt", false, "enable decryption for assemble mode")
	cloudMode := flag.Bool("cloud", false, "enable cloud distribution mode")
	cloudDownload := flag.Bool("cloud-download", false, "download chunks from cloud for assembly")
	cloudCleanup := flag.Bool("cloud-cleanup", false, "remove local chunks after successful cloud upload")
	cloudProviders := flag.String("cloud-providers", "gdrive", "comma-separated list of cloud providers to use (gdrive,dropbox,onedrive,mega,ipfs)")
	configFile := flag.String("config", "config.json", "path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Printf("Warning: Failed to load config file: %v", err)
		log.Println("Using default configuration...")
		cfg = config.DefaultConfig()
	}

	var encConfig *encryption.EncryptionConfig

	if *encrypt || *decrypt {
		fmt.Print("Enter encryption/decryption password: ")
		password, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			log.Fatal("Failed to read password:", err)
		}
		fmt.Println()

		encConfig = encryption.CreateEncryptionConfig(string(password), *encrypt || *decrypt)
	} else {
		encConfig = encryption.CreateEncryptionConfig("", false)
	}

	switch *mode {
	case "split":
		if *decrypt {
			log.Fatal("Cannot use -decrypt flag with split mode")
		}

		// Use configurable chunk size from config
		err := chunker.SplitFileWithChunkSize(*input, *out, *manifestPath, encConfig, cfg.ChunkConfig.ChunkSize)
		if err != nil {
			log.Fatal("Split failed:", err)
		}
		if *encrypt {
			fmt.Printf("File split and encrypted (chunk size: %.1f MB)\n", float64(cfg.ChunkConfig.ChunkSize)/(1024*1024))
		} else {
			fmt.Printf("File split successfully (chunk size: %.1f MB)\n", float64(cfg.ChunkConfig.ChunkSize)/(1024*1024))
		}

		// Upload to cloud if requested
		if *cloudMode {
			fmt.Println("Uploading to cloud...")
			providers := parseCloudProviders(*cloudProviders)
			strategy := cloudstorage.CustomCloudStrategy(providers)

		fmt.Printf("Using providers: %v\n", providers)

		uploader, err := cloudstorage.CreateCloudUploader(strategy, cfg)
		if err != nil {
			log.Fatal("Cloud uploader setup failed:", err)
		}

		err = uploader.UploadChunks(*out, *manifestPath)
		if err != nil {
			log.Fatal("Upload failed:", err)
		}
			fmt.Println("Upload complete!")

			// Clean up local chunks if requested
			if *cloudCleanup {
				fmt.Println("Cleaning up local chunks...")
				err = chunker.CleanupChunks(*out)
				if err != nil {
					log.Printf("Warning: Failed to cleanup chunks: %v", err)
				}
			}
		}
	case "assemble":
		if *encrypt {
			log.Fatal("Cannot use -encrypt flag with assemble mode")
		}

		// Download from cloud if requested
		if *cloudDownload {
			fmt.Println("Downloading from cloud...")
			providers := parseCloudProviders(*cloudProviders)
			strategy := cloudstorage.CustomCloudStrategy(providers)

			uploader, err := cloudstorage.CreateCloudUploader(strategy, cfg)
			if err != nil {
				log.Fatal("Cloud setup failed:", err)
			}

			err = uploader.DownloadChunks(*manifestPath, *chunksPath)
			if err != nil {
				log.Fatal("Download failed:", err)
			}
			fmt.Println("Download complete!")
		}

		err := chunker.AssembleFile(*manifestPath, *chunksPath, *out, encConfig)
		if err != nil {
			log.Fatal("Assemble failed:", err)
		}
		if *decrypt {
			fmt.Println("File assembled and decrypted")
		} else {
			fmt.Println("File assembled successfully")
		}
	default:
		fmt.Println("Usage:")
		fmt.Println("  Split:    -mode split -in input_file -out output_dir [-encrypt] [-cloud]")
		fmt.Println("  Assemble: -mode assemble -out output_file [-decrypt] [-cloud-download]")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  -config:          Configuration file path (default: config.json)")
		fmt.Println("  -encrypt:         Encrypt chunks when splitting")
		fmt.Println("  -decrypt:         Decrypt chunks when assembling")
		fmt.Println("  -cloud:           Upload chunks to cloud after splitting")
		fmt.Println("  -cloud-download:  Download chunks from cloud before assembling")
		fmt.Println("  -cloud-cleanup:   Remove local chunks after successful cloud upload")
		fmt.Println("  -cloud-providers: Comma-separated providers (default: gdrive)")
		fmt.Println()
		fmt.Println("Configuration:")
		fmt.Println("  Create config.json to customize chunk size, multiple accounts, etc.")
		fmt.Println("  Use -config flag to specify custom config file location")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  ./chunk-store -mode split -in file.mkv -out chunks -cloud")
		fmt.Println("  ./chunk-store -mode split -in file.mkv -out chunks -cloud -config my-config.json")
		fmt.Println("  ./chunk-store -mode split -in file.mkv -out chunks -cloud -cloud-cleanup")
		fmt.Println("  ./chunk-store -mode assemble -out file.mkv -cloud-download -decrypt")
		fmt.Println()
		fmt.Println("Supported providers:")
		fmt.Println("  ✓ Google Drive (multiple accounts supported)")
		fmt.Println("  - Dropbox (planned)")
		fmt.Println("  - OneDrive (planned)")
		fmt.Println("  - MEGA (planned)")
		fmt.Println("  - IPFS (planned)")
	}
}
