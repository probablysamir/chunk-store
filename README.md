# ChunkStore

Split large files into encrypted chunks and distribute them across multiple cloud storage providers. No single point of failure, no vendor lock-in.

## Why?

I was trying to backup some large video files but ran into two problems:
1. My Google Drive was almost full
2. I didn't want to trust a single cloud provider with everything

So I built this. It splits files into configurable chunks, encrypts them, and spreads them across different cloud services. When I need the file back, it downloads all the pieces and puts them back together.

## Features

- **Configurable chunking** - Split files into custom sizes (default: 100MB)
- **Multiple accounts** - Use multiple Google Drive accounts for load balancing
- **Encryption** - AES-256-GCM with your own password
- **Multi-cloud ready** - Designed for multiple cloud providers
- **Round-robin distribution** - Automatic load balancing across accounts
- **Integrity checks** - SHA-256 verification for each chunk
- **Progress tracking** - See upload/download progress
- **Configuration system** - JSON-based settings management
- **Local mode** - Works without cloud storage too

### Cloud providers

- ✅ **Google Drive** (multiple accounts supported)
- 🚧 Dropbox (planned)
- 🚧 OneDrive (planned)  
- 🚧 MEGA (planned)
- 🚧 IPFS (planned)

## Getting started

You'll need Go 1.23+ installed. For Google Drive, you'll also need API credentials (see setup below).

```bash
git clone https://github.com/probablysamir/chunk-store.git
cd chunk-store
go build -o chunk-store ./cmd
```

### Basic usage

Split a file:
```bash
./chunk-store -mode split -in bigfile.mkv -out chunks/
```

Put it back together:
```bash
./chunk-store -mode assemble -manifest manifest.json -out bigfile.mkv
```

With custom configuration:
```bash
./chunk-store -mode split -in movie.mkv -out chunks/ -config my-config.json
```

With encryption:
```bash
./chunk-store -mode split -in secret.pdf -out chunks/ -encrypt
./chunk-store -mode assemble -manifest manifest.json -out secret.pdf -decrypt
```

With Google Drive (multiple accounts):
```bash
./chunk-store -mode split -in movie.mkv -out chunks/ -cloud -encrypt
./chunk-store -mode assemble -manifest manifest.json -out movie.mkv -cloud-download -decrypt
```

Clean up local chunks after cloud upload:
```bash
./chunk-store -mode split -in movie.mkv -out chunks/ -cloud -cloud-cleanup -encrypt
```

## Configuration

ChunkStore uses a JSON configuration file for advanced settings. On first run, it creates a default `config.json`:

```json
{
  "chunk_config": {
    "chunk_size": 104857600
  },
  "cloud_config": {
    "google_drive_accounts": [
      {
        "name": "primary",
        "creds_file": "credentials.json",
        "token_file": "token.json",
        "folder_name": "distributed-chunks",
        "enabled": true,
        "description": "Primary Google Drive account"
      }
    ],
    "providers": ["gdrive"],
    "replication_count": 1,
    "load_balancing": "round_robin"
  },
  "version": "1.0"
}
```

### Multiple Google Drive Accounts

To use multiple Google Drive accounts for load balancing:

```json
{
  "cloud_config": {
    "google_drive_accounts": [
      {
        "name": "primary",
        "creds_file": "credentials.json",
        "token_file": "token.json",
        "folder_name": "distributed-chunks",
        "enabled": true,
        "description": "Primary Google Drive account"
      },
      {
        "name": "backup",
        "creds_file": "backup_credentials.json",
        "token_file": "backup_token.json", 
        "folder_name": "backup-chunks",
        "enabled": true,
        "description": "Backup Google Drive account"
      }
    ],
    "replication_count": 1,
    "load_balancing": "round_robin"
  }
}
```

### Configuration Options

- **chunk_size**: Size of each chunk in bytes (default: 100MB)
- **replication_count**: How many copies of each chunk to store
- **load_balancing**: `"round_robin"`, `"random"`, or `"size_based"`
- **enabled**: Enable/disable individual accounts
- **folder_name**: Custom folder name for each account

## Google Drive setup

To use Google Drive, you need API credentials for each account:

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select one
3. Enable the Google Drive API
4. Create OAuth2 credentials for a desktop app
5. Download the JSON file and rename it to `credentials.json` (or custom name)
6. Put it in the project directory

For multiple accounts, repeat this process and use different credential files:
- `credentials.json` for primary account
- `backup_credentials.json` for backup account
- etc.

First time you run with `-cloud`, it'll open your browser for OAuth. After that, it saves token files for future use.

## How it works

1. **Split** - File gets chopped into configurable chunks (default: 100MB) with unique IDs
2. **Encrypt** (optional) - Each chunk encrypted with AES-256-GCM
3. **Distribute** - Chunks distributed across multiple accounts using round-robin
4. **Upload** - Parallel uploads to different Google Drive accounts
5. **Manifest** - JSON file tracks where everything is stored
6. **Download** - Reverse the process to get your file back

### Load Balancing

With multiple Google Drive accounts, chunks are distributed using round-robin:
- Chunk 1 → Account A
- Chunk 2 → Account B  
- Chunk 3 → Account A
- Chunk 4 → Account B
- etc.

This provides both **load distribution** and **redundancy** across accounts.

## All the options

```
-mode string            "split" or "assemble"
-in string              Input file path (for splitting)
-out string             Output directory/file path
-config string          Configuration file path (default: "config.json")
-manifest string        Manifest file (default: "manifest.json")
-chunkspath string      Where chunks are stored (default: "chunks")
-encrypt                Encrypt chunks when splitting
-decrypt                Decrypt chunks when assembling
-cloud                  Upload to cloud after splitting
-cloud-download         Download from cloud before assembling
-cloud-cleanup          Remove local chunks after successful cloud upload
-cloud-providers        Which providers to use (default: "gdrive")
```

**Configuration-based options** (set in config.json):
- Chunk size (bytes)
- Multiple Google Drive accounts
- Load balancing strategy
- Replication count
- Account-specific settings

## Examples

Basic splitting:
```bash
# Split a big file (using config.json settings)
./chunk-store -mode split -in video.mkv -out ./chunks

# Put it back together  
./chunk-store -mode assemble -manifest manifest.json -out video.mkv
```

Custom configuration:
```bash
# Use custom config with different chunk size or accounts
./chunk-store -mode split -in video.mkv -out ./chunks -config my-config.json
```

Encrypted cloud backup with multiple accounts:
```bash
# Split, encrypt, upload across multiple Google Drive accounts
./chunk-store -mode split -in important.zip -out ./chunks -encrypt -cloud

# Split, encrypt, upload, and cleanup local chunks
./chunk-store -mode split -in important.zip -out ./chunks -encrypt -cloud -cloud-cleanup

# Download from multiple accounts, decrypt, assemble
./chunk-store -mode assemble -manifest manifest.json -out important.zip -cloud-download -decrypt
```

With custom chunk sizes:
```bash
# Edit config.json to set chunk_size: 52428800 (50MB chunks)
./chunk-store -mode split -in largefile.tar.gz -cloud
```

## Project structure

```
chunk-store/
├── cmd/main.go                  # CLI interface
├── internal/
│   ├── chunker/                 # File splitting/assembly
│   ├── encryption/              # AES-256-GCM crypto
│   ├── manifest/                # Metadata management  
│   ├── config/                  # Configuration system
│   └── cloudstorage/            # Cloud provider implementations
├── config.json                  # Main configuration file
├── config.json.example          # Example configuration
├── credentials.json             # Google Drive API creds (primary)
├── backup_credentials.json      # Google Drive API creds (backup)
├── token.json                   # OAuth token (primary)
├── backup_token.json           # OAuth token (backup)
└── manifest.json               # Generated chunk metadata
```

## Security stuff

- Uses AES-256-GCM encryption with PBKDF2 key derivation
- Each chunk gets its own nonce  
- SHA-256 checksums verify file integrity
- Multiple accounts provide redundancy
- Your cloud credentials stay local
- The manifest tracks chunk distribution across accounts
- Configuration supports enabling/disabling individual accounts

## Contributing

Want to add more cloud providers? The code is set up to make it pretty straightforward. Each provider just needs to implement the upload/download interface in `cloudstorage.go`.

## Why I made this

I had some big files to backup but didn't want to rely on just one cloud service. Plus I was hitting storage limits on my free accounts. This way I can split files across multiple services and even multiple accounts on the same service, while keeping everything encrypted and organized.

The latest version supports multiple Google Drive accounts, so you can distribute load across different accounts and get better upload speeds while staying within individual account quotas.

## Troubleshooting

**"Cloud uploader setup failed" or Google Drive initialization errors**
- Check that all credential files exist and are valid JSON
- Make sure Google Drive API is enabled in your Google Cloud project(s)
- Verify your OAuth2 credentials are for "Desktop application"
- Try deleting token files and re-authenticating
- Check that each account's credentials point to projects with Drive API enabled

**"Hash mismatch on chunk"**  
- A chunk got corrupted during storage or transfer
- Try re-downloading from cloud storage

**"Can't read client secret file"**
- Make sure credential files exist and have the right format
- Check the `config.json.example` file for reference
- Verify file paths in your configuration are correct

**"No enabled accounts configured"**
- Check your `config.json` and ensure at least one account has `"enabled": true`
- Verify the configuration file is being loaded correctly

**Multiple account authentication issues**
- Each Google Drive account needs its own credentials and token files
- Make sure each credential file is from a different Google Cloud project or the same project with multiple OAuth clients
- Delete specific token files to re-authenticate individual accounts

**Upload/download too slow**
- Google Drive API has rate limits that affect speed
- Consider adjusting chunk size in configuration (larger chunks = fewer API calls)
- Multiple accounts help distribute load but each still has individual limits

**Permission errors**
- Tool needs write permissions for output directory and config files
- On Linux/Mac, run `chmod +x chunk-store` after building

## License

MIT - see LICENSE file
