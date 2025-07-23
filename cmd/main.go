package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"syscall"

	"github.com/probablysamir/chunk-store/internal/chunker"
	"github.com/probablysamir/chunk-store/internal/cloudstorage"
	"github.com/probablysamir/chunk-store/internal/encryption"
	"golang.org/x/term"
)

// parseCloudProviders converts comma-separated provider string to slice
func parseCloudProviders(providersStr string) []cloudstorage.CloudProvider {
	if providersStr == "" {
		return []cloudstorage.CloudProvider{cloudstorage.GoogleDrive}
	}

	providerNames := strings.Split(providersStr, ",")
	var providers []cloudstorage.CloudProvider

	for _, name := range providerNames {
		name = strings.TrimSpace(name)
		switch strings.ToLower(name) {
		case "gdrive", "googledrive", "google-drive":
			providers = append(providers, cloudstorage.GoogleDrive)
		case "dropbox":
			providers = append(providers, cloudstorage.Dropbox)
		case "onedrive", "one-drive":
			providers = append(providers, cloudstorage.OneDrive)
		case "mega":
			providers = append(providers, cloudstorage.MEGACloud)
		case "ipfs":
			providers = append(providers, cloudstorage.IPFS)
		default:
			fmt.Printf("Warning: Unknown provider '%s', ignoring\n", name)
		}
	}

	if len(providers) == 0 {
		providers = []cloudstorage.CloudProvider{cloudstorage.GoogleDrive}
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
	gdriveCredsPath := flag.String("gdrive-creds", "credentials.json", "path to Google Drive credentials file")
	gdriveTokenPath := flag.String("gdrive-token", "token.json", "path to Google Drive token file")
	flag.Parse()

	var encConfig *encryption.EncryptionConfig

	if *encrypt || *decrypt {
		fmt.Print("Enter encryption/decryption password: ")
		password, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			log.Fatal("Failed to read password:", err)
		}
		fmt.Println()

		encConfig = encryption.NewEncryptionConfig(string(password), true)
	} else {
		encConfig = encryption.NewEncryptionConfig("", false)
	}

	switch *mode {
	case "split":
		if *decrypt {
			log.Fatal("Cannot use -decrypt flag with split mode")
		}

		err := chunker.SplitFile(*input, *out, *manifestPath, encConfig)
		if err != nil {
			log.Fatal("Split failed:", err)
		}
		if *encrypt {
			fmt.Println("File split and encrypted")
		} else {
			fmt.Println("File split successfully")
		}

		// Upload to cloud if requested
		if *cloudMode {
			fmt.Println("Uploading to cloud...")
			providers := parseCloudProviders(*cloudProviders)
			strategy := cloudstorage.CustomCloudStrategy(providers)

			fmt.Printf("Using providers: %v\n", providers)

			uploader, err := cloudstorage.NewCloudUploader(strategy, *gdriveCredsPath, *gdriveTokenPath)
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

			uploader, err := cloudstorage.NewCloudUploader(strategy, *gdriveCredsPath, *gdriveTokenPath)
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
		fmt.Println("  -encrypt:         Encrypt chunks when splitting")
		fmt.Println("  -decrypt:         Decrypt chunks when assembling")
		fmt.Println("  -cloud:           Upload chunks to cloud after splitting")
		fmt.Println("  -cloud-download:  Download chunks from cloud before assembling")
		fmt.Println("  -cloud-cleanup:   Remove local chunks after successful cloud upload")
		fmt.Println("  -cloud-providers: Comma-separated providers (default: gdrive)")
		fmt.Println("  -gdrive-creds:    Google Drive credentials file")
		fmt.Println("  -gdrive-token:    Google Drive token file")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  ./chunk-store -mode split -in file.mkv -out chunks -cloud")
		fmt.Println("  ./chunk-store -mode split -in file.mkv -out chunks -cloud -cloud-cleanup")
		fmt.Println("  ./chunk-store -mode assemble -out file.mkv -cloud-download -decrypt")
		fmt.Println()
		fmt.Println("Supported providers:")
		fmt.Println("  ✓ Google Drive")
		fmt.Println("  - Dropbox (planned)")
		fmt.Println("  - OneDrive (planned)")
		fmt.Println("  - MEGA (planned)")
		fmt.Println("  - IPFS (planned)")
	}
}
