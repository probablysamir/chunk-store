package cloudstorage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// GoogleDriveClient handles Google Drive API operations
type GoogleDriveClient struct {
	service   *drive.Service
	folderID  string
	tokenFile string
	credsFile string
	name      string // Account name for identification
	folderName string // Custom folder name
}

// CreateGoogleDriveClient creates a new Google Drive client
func CreateGoogleDriveClient(credsFile, tokenFile string) (*GoogleDriveClient, error) {
	return CreateGoogleDriveClientWithName(credsFile, tokenFile, "default", "distributed-chunks")
}

// CreateGoogleDriveClientWithName creates a new Google Drive client with custom name and folder
func CreateGoogleDriveClientWithName(credsFile, tokenFile, name, folderName string) (*GoogleDriveClient, error) {
	return &GoogleDriveClient{
		credsFile:  credsFile,
		tokenFile:  tokenFile,
		name:       name,
		folderName: folderName,
		folderID:   "", // Will be set when creating/finding the folder
	}, nil
}

// Initialize sets up the Google Drive service with authentication
func (gd *GoogleDriveClient) Initialize() error {
	ctx := context.Background()

	// Read credentials file
	b, err := os.ReadFile(gd.credsFile)
	if err != nil {
		return fmt.Errorf("can't read credentials file: %v", err)
	}

	// Parse credentials
	config, err := google.ConfigFromJSON(b, drive.DriveFileScope)
	if err != nil {
		return fmt.Errorf("credentials file format is invalid: %v", err)
	}

	// Get OAuth2 client
	client := gd.getClient(config)

	// Create Drive service
	srv, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("couldn't create Drive service: %v", err)
	}

	gd.service = srv

	// Create or find the distributed-chunks folder
	err = gd.setupFolder()
	if err != nil {
		return fmt.Errorf("google Drive setup failed (check credentials and API access): %v", err)
	}

	return nil
}

// getClient retrieves a token, saves the token, then returns the generated client
func (gd *GoogleDriveClient) getClient(config *oauth2.Config) *http.Client {
	// Try to load token from file
	tok, err := gd.tokenFromFile()
	if err != nil {
		// Get token from web if not found
		tok = gd.getTokenFromWeb(config)
		gd.saveToken(tok)
	}
	return config.Client(context.Background(), tok)
}

// getTokenFromWeb requests a token from the web
func (gd *GoogleDriveClient) getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	// Start local server to catch redirect
	codeChan := make(chan string)
	server := &http.Server{Addr: ":8080"}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code != "" {
			w.Write([]byte("Authentication successful! You can close this tab."))
			codeChan <- code
		} else {
			w.Write([]byte("Authentication failed. Please try again."))
		}
	})

	// Start server
	go func() {
		server.ListenAndServe()
	}()

	// Set redirect URL and generate auth URL
	config.RedirectURL = "http://localhost:8080"
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)

	fmt.Printf("Opening browser for Google Drive authentication...\n")
	fmt.Printf("If browser doesn't open, go to: %s\n", authURL)

	// Try to open browser
	openBrowser(authURL)

	// Wait for code
	select {
	case authCode := <-codeChan:
		server.Shutdown(context.Background())
		tok, err := config.Exchange(context.TODO(), authCode)
		if err != nil {
			fmt.Printf("Authentication failed: %v\n", err)
			return nil
		}
		fmt.Println("Authentication complete!")
		return tok

	case <-time.After(2 * time.Minute):
		server.Shutdown(context.Background())
		fmt.Println("Authentication timed out.")
		return nil
	}
}

// openBrowser opens the URL in the default browser
func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	}
	if err != nil {
		// Silent fallback - user will use the printed URL
	}
}

// tokenFromFile retrieves a token from a local file
func (gd *GoogleDriveClient) tokenFromFile() (*oauth2.Token, error) {
	f, err := os.Open(gd.tokenFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// saveToken saves a token to a file path
func (gd *GoogleDriveClient) saveToken(token *oauth2.Token) {
	fmt.Printf("Saving token to: %s\n", gd.tokenFile)
	f, err := os.OpenFile(gd.tokenFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		fmt.Printf("Can't save token: %v", err)
		return
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

// setupFolder creates or finds the distributed-chunks folder
func (gd *GoogleDriveClient) setupFolder() error {
	// Search for existing folder
	folderName := gd.folderName
	if folderName == "" {
		folderName = "distributed-chunks"
	}
	
	query := fmt.Sprintf("name='%s' and mimeType='application/vnd.google-apps.folder' and trashed=false", folderName)
	r, err := gd.service.Files.List().Q(query).Do()
	if err != nil {
		return fmt.Errorf("can't search for folder: %v", err)
	}

	if len(r.Files) > 0 {
		// Folder exists, use it
		gd.folderID = r.Files[0].Id
		fmt.Printf("Using existing Google Drive folder '%s' for account '%s': %s (ID: %s)\n", 
			folderName, gd.name, r.Files[0].Name, gd.folderID)
		return nil
	}

	// Create new folder
	folder := &drive.File{
		Name:     folderName,
		MimeType: "application/vnd.google-apps.folder",
	}

	file, err := gd.service.Files.Create(folder).Do()
	if err != nil {
		return fmt.Errorf("can't create folder: %v", err)
	}

	gd.folderID = file.Id
	fmt.Printf("Created Google Drive folder '%s' for account '%s': %s (ID: %s)\n", 
		folderName, gd.name, file.Name, gd.folderID)
	return nil
}

// UploadFile uploads a file to Google Drive
func (gd *GoogleDriveClient) UploadFile(localPath, cloudPath string) (string, error) {
	// Open local file
	file, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("unable to open file: %v", err)
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("unable to get file info: %v", err)
	}

	// Extract filename from cloudPath
	fileName := filepath.Base(cloudPath)

	// Create file metadata
	driveFile := &drive.File{
		Name:    fileName,
		Parents: []string{gd.folderID},
	}

	// Upload file
	res, err := gd.service.Files.Create(driveFile).Media(file).Do()
	if err != nil {
		return "", fmt.Errorf("unable to upload file: %v", err)
	}

	fmt.Printf("Uploaded to Google Drive account '%s': %s (ID: %s, Size: %d bytes)\n",
		gd.name, res.Name, res.Id, fileInfo.Size())

	return res.Id, nil
}

// DownloadFile downloads a file from Google Drive
func (gd *GoogleDriveClient) DownloadFile(fileID, localPath string) error {
	// Get file content
	resp, err := gd.service.Files.Get(fileID).Download()
	if err != nil {
		return fmt.Errorf("unable to download file: %v", err)
	}
	defer resp.Body.Close()

	// Create local file
	err = os.MkdirAll(filepath.Dir(localPath), 0755)
	if err != nil {
		return fmt.Errorf("unable to create directory: %v", err)
	}

	outFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("unable to create local file: %v", err)
	}
	defer outFile.Close()

	// Copy content
	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("unable to copy file content: %v", err)
	}

	fmt.Printf("Downloaded from Google Drive account '%s': %s\n", gd.name, localPath)
	return nil
}

// FindFileByName searches for a file by name in the distributed-chunks folder
func (gd *GoogleDriveClient) FindFileByName(fileName string) (string, error) {
	query := fmt.Sprintf("name='%s' and '%s' in parents and trashed=false", fileName, gd.folderID)
	r, err := gd.service.Files.List().Q(query).Do()
	if err != nil {
		return "", fmt.Errorf("unable to search for file: %v", err)
	}

	if len(r.Files) == 0 {
		return "", fmt.Errorf("file not found: %s", fileName)
	}

	return r.Files[0].Id, nil
}

// DeleteFile deletes a file from Google Drive
func (gd *GoogleDriveClient) DeleteFile(fileID string) error {
	err := gd.service.Files.Delete(fileID).Do()
	if err != nil {
		return fmt.Errorf("unable to delete file: %v", err)
	}
	return nil
}

// ListFiles lists all files in the distributed-chunks folder
func (gd *GoogleDriveClient) ListFiles() ([]*drive.File, error) {
	query := fmt.Sprintf("'%s' in parents and trashed=false", gd.folderID)
	r, err := gd.service.Files.List().Q(query).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to list files: %v", err)
	}
	return r.Files, nil
}
