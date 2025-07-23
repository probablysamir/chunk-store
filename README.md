# ChunkStore

Split large files into encrypted chunks and distribute them across multiple cloud storage providers. No single point of failure, no vendor lock-in.

## Why?

I was trying to backup some large video files but ran into two problems:
1. My Google Drive was almost full
2. I didn't want to trust a single cloud provider with everything

So I built this. It splits files into 1MB chunks, encrypts them, and spreads them across different cloud services. When I need the file back, it downloads all the pieces and puts them back together.

## Features

- **File chunking** - Splits files into 1MB pieces
- **Encryption** - AES-256-GCM with your own password
- **Multi-cloud** - Store chunks across different providers
- **Integrity checks** - SHA-256 verification for each chunk
- **Progress bars** - See what's happening
- **Local mode** - Works without cloud storage too

### Cloud providers

- ✅ Google Drive (working)
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

With encryption:
```bash
./chunk-store -mode split -in secret.pdf -out chunks/ -encrypt
./chunk-store -mode assemble -manifest manifest.json -out secret.pdf -decrypt
```

With Google Drive:
```bash
./chunk-store -mode split -in movie.mkv -out chunks/ -cloud -encrypt
./chunk-store -mode assemble -manifest manifest.json -out movie.mkv -cloud-download -decrypt
```

Clean up local chunks after cloud upload:
```bash
./chunk-store -mode split -in movie.mkv -out chunks/ -cloud -cloud-cleanup -encrypt
```

## Google Drive setup

To use Google Drive, you need API credentials:

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select one
3. Enable the Google Drive API
4. Create OAuth2 credentials for a desktop app
5. Download the JSON file and rename it to `credentials.json`
6. Put it in the project directory

First time you run with `-cloud`, it'll open your browser for OAuth. After that, it saves a `token.json` file for future use.

## How it works

1. **Split** - File gets chopped into 1MB chunks with unique IDs
2. **Encrypt** (optional) - Each chunk encrypted with AES-256-GCM
3. **Upload** - Chunks distributed to cloud services round-robin style  
4. **Manifest** - JSON file tracks where everything is stored
5. **Download** - Reverse the process to get your file back

## All the options

```
-mode string          "split" or "assemble"
-in string            Input file path (for splitting)
-out string           Output directory/file path
-manifest string      Manifest file (default: "manifest.json")
-chunkspath string    Where chunks are stored (default: "chunks")
-encrypt              Encrypt chunks when splitting
-decrypt              Decrypt chunks when assembling
-cloud                Upload to cloud after splitting
-cloud-download       Download from cloud before assembling
-cloud-cleanup        Remove local chunks after successful cloud upload
-cloud-providers      Which providers to use (default: "gdrive")
-gdrive-creds string  Google Drive credentials (default: "credentials.json")
-gdrive-token string  Google Drive token (default: "token.json")
```

## Examples

Basic splitting:
```bash
# Split a big file
./chunk-store -mode split -in video.mkv -out ./chunks

# Put it back together  
./chunk-store -mode assemble -manifest manifest.json -out video.mkv
```

Encrypted cloud backup:
```bash
# Split, encrypt, upload to Google Drive
./chunk-store -mode split -in important.zip -out ./chunks -encrypt -cloud

# Split, encrypt, upload, and cleanup local chunks
./chunk-store -mode split -in important.zip -out ./chunks -encrypt -cloud -cloud-cleanup

# Download, decrypt, assemble
./chunk-store -mode assemble -manifest manifest.json -out important.zip -cloud-download -decrypt
```

Multiple providers (when more are implemented):
```bash
./chunk-store -mode split -in file.tar.gz -cloud -cloud-providers gdrive,dropbox
```

## Project structure

```
chunk-store/
├── cmd/main.go                  # CLI interface
├── internal/
│   ├── chunker/                 # File splitting/assembly
│   ├── encryption/              # AES-256-GCM crypto
│   ├── manifest/                # Metadata management  
│   └── cloudstorage/            # Cloud provider stuff
├── credentials.json             # Google Drive API creds
├── token.json                   # OAuth token
└── manifest.json               # Generated chunk info
```

## Security stuff

- Uses AES-256-GCM encryption with PBKDF2 key derivation
- Each chunk gets its own nonce  
- SHA-256 checksums verify file integrity
- Your cloud credentials stay local
- The manifest doesn't contain sensitive info about your original file

## Contributing

Want to add more cloud providers? The code is set up to make it pretty straightforward. Each provider just needs to implement the upload/download interface in `cloudstorage.go`.

## Why I made this

I had some big files to backup but didn't want to rely on just one cloud service. Plus I was hitting storage limits on my free accounts. This way I can split files across multiple services while keeping everything encrypted and organized.

## Troubleshooting

**"Google Drive client not initialized"** or **"Google Drive setup failed"**
- Check that `credentials.json` exists and is valid JSON
- Make sure Google Drive API is enabled in your Google Cloud project
- Verify your OAuth2 credentials are for a "Desktop application"
- Try deleting `token.json` and re-authenticating

**"Hash mismatch on chunk"**  
- A chunk got corrupted during storage or transfer
- Try re-downloading from cloud storage

**"Can't read client secret file"**
- Make sure `credentials.json` exists and has the right format
- Check the `credentials.json.example` file for reference

**Permission errors**
- Tool needs write permissions for output directory
- On Linux/Mac, run `chmod +x chunk-store` after building

**Authentication issues**
- Delete `token.json` and run the command again to re-authenticate
- Make sure you're using the same Google account that has access to the API project

## License

MIT - see LICENSE file
