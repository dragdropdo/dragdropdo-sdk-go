package d3

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

// Dragdropdo represents a D3 API client
type Dragdropdo struct {
	apiKey   string
	baseURL  string
	timeout  time.Duration
	headers  map[string]string
	httpClient *resty.Client
}

// Config represents client configuration
type Config struct {
	APIKey  string
	BaseURL string
	Timeout time.Duration
	Headers map[string]string
}

// UploadFileOptions represents options for file upload
type UploadFileOptions struct {
	File      string
	FileName  string
	MimeType  string
	Parts     int
	OnProgress func(UploadProgress)
}

// UploadProgress represents upload progress information
type UploadProgress struct {
	CurrentPart   int
	TotalParts    int
	BytesUploaded int64
	TotalBytes    int64
	Percentage    int
}

// UploadResponse represents response from file upload
type UploadResponse struct {
	FileKey       string   `json:"file_key"`
	UploadID      string   `json:"upload_id"`
	PresignedURLs []string `json:"presigned_urls"`
	ObjectName    string   `json:"object_name,omitempty"`
	// CamelCase aliases for compatibility
	FileKeyAlias       string   `json:"fileKey,omitempty"`
	UploadIDAlias      string   `json:"uploadId,omitempty"`
	PresignedURLsAlias []string `json:"presignedUrls,omitempty"`
	ObjectNameAlias    string   `json:"objectName,omitempty"`
}

// SupportedOperationOptions represents options for checking supported operations
type SupportedOperationOptions struct {
	Ext        string
	Action     string
	Parameters map[string]interface{}
}

// SupportedOperationResponse represents response from supported operation check
type SupportedOperationResponse struct {
	Supported       bool                   `json:"supported"`
	Ext             string                 `json:"ext"`
	Action          string                 `json:"action,omitempty"`
	AvailableActions []string              `json:"available_actions,omitempty"`
	Parameters      map[string]interface{} `json:"parameters,omitempty"`
}

// OperationOptions represents options for creating an operation
type OperationOptions struct {
	Action     string
	FileKeys   []string
	Parameters map[string]interface{}
	Notes      map[string]string
}

// OperationResponse represents response from operation creation
type OperationResponse struct {
	MainTaskID string `json:"main_task_id"`
	// CamelCase alias
	MainTaskIDAlias string `json:"mainTaskId,omitempty"`
}

// StatusOptions represents options for getting status
type StatusOptions struct {
	MainTaskID string
	FileTaskID string
}

// FileTaskStatus represents status of a file task
type FileTaskStatus struct {
	FileKey      string `json:"file_key"`
	Status       string `json:"status"`
	DownloadLink string `json:"download_link,omitempty"`
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// StatusResponse represents response from status check
type StatusResponse struct {
	OperationStatus string           `json:"operation_status"`
	FilesData       []FileTaskStatus `json:"files_data"`
	// CamelCase aliases
	OperationStatusAlias string           `json:"operationStatus,omitempty"`
	FilesDataAlias       []FileTaskStatus `json:"filesData,omitempty"`
}

// PollStatusOptions represents options for polling status
type PollStatusOptions struct {
	StatusOptions
	Interval time.Duration
	Timeout  time.Duration
	OnUpdate func(StatusResponse)
}

// NewDragdropdo creates a new Dragdropdo Client instance
func NewDragdropdo(config Config) (*Dragdropdo, error) {
	if config.APIKey == "" {
		return nil, errors.New("API key is required")
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api-dev.dragdropdo.com"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	timeout := config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": fmt.Sprintf("Bearer %s", config.APIKey),
	}
	for k, v := range config.Headers {
		headers[k] = v
	}

	httpClient := resty.New().
		SetBaseURL(baseURL).
		SetTimeout(timeout).
		SetHeaders(headers)

	return &Dragdropdo{
		apiKey:     config.APIKey,
		baseURL:    baseURL,
		timeout:    timeout,
		headers:    headers,
		httpClient: httpClient,
	}, nil
}

// UploadFile uploads a file to D3 storage
func (c *Dragdropdo) UploadFile(options UploadFileOptions) (*UploadResponse, error) {
	if options.FileName == "" {
		return nil, errors.New("file_name is required")
	}

	fileInfo, err := os.Stat(options.File)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}
	fileSize := fileInfo.Size()

	// Calculate parts if not provided
	chunkSize := int64(5 * 1024 * 1024) // 5MB per part
	calculatedParts := options.Parts
	if calculatedParts == 0 {
		calculatedParts = int((fileSize + chunkSize - 1) / chunkSize)
	}
	if calculatedParts > 100 {
		calculatedParts = 100
	}
	if calculatedParts < 1 {
		calculatedParts = 1
	}

	// Detect MIME type if not provided
	detectedMimeType := options.MimeType
	if detectedMimeType == "" {
		ext := filepath.Ext(options.FileName)
		detectedMimeType = mime.TypeByExtension(ext)
		if detectedMimeType == "" {
			detectedMimeType = c.getMimeType(ext)
		}
		if detectedMimeType == "" {
			detectedMimeType = "application/octet-stream"
		}
	}

	// Step 1: Request presigned URLs
	var uploadResp struct {
		Data struct {
			FileKey       string   `json:"file_key"`
			UploadID      string   `json:"upload_id"`
			PresignedURLs []string `json:"presigned_urls"`
			ObjectName    string   `json:"object_name"`
		} `json:"data"`
	}

	_, err = c.httpClient.R().
		SetBody(map[string]interface{}{
			"file_name": options.FileName,
			"size":      fileSize,
			"mime_type": detectedMimeType,
			"parts":     calculatedParts,
		}).
		SetResult(&uploadResp).
		Post("/api/v1/initiate-upload")

	if err != nil {
		return nil, fmt.Errorf("failed to request presigned URLs: %w", err)
	}

	fileKey := uploadResp.Data.FileKey
	uploadID := uploadResp.Data.UploadID
	presignedURLs := uploadResp.Data.PresignedURLs
	objectName := uploadResp.Data.ObjectName

	if len(presignedURLs) != calculatedParts {
		return nil, fmt.Errorf("mismatch: requested %d parts but received %d presigned URLs", calculatedParts, len(presignedURLs))
	}

	if uploadID == "" {
		return nil, errors.New("upload ID not received from server")
	}

	// Step 2: Upload file parts and capture ETags
	chunkSizePerPart := (fileSize + int64(calculatedParts) - 1) / int64(calculatedParts)
	bytesUploaded := int64(0)
	uploadParts := []map[string]interface{}{}

	file, err := os.Open(options.File)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	for i := 0; i < calculatedParts; i++ {
		start := int64(i) * chunkSizePerPart
		end := start + chunkSizePerPart
		if end > fileSize {
			end = fileSize
		}
		partSize := end - start

		// Read chunk
		chunk := make([]byte, partSize)
		_, err = file.ReadAt(chunk, start)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("failed to read chunk: %w", err)
		}

		// Upload chunk
		req, err := http.NewRequest("PUT", presignedURLs[i], bytes.NewReader(chunk))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", detectedMimeType)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to upload chunk: %w", err)
		}
		resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("failed to upload part %d: status %d", i+1, resp.StatusCode)
		}

		// Extract ETag from response
		etag := resp.Header.Get("ETag")
		if etag == "" {
			etag = resp.Header.Get("etag")
		}
		if etag == "" {
			return nil, fmt.Errorf("failed to get ETag for part %d", i+1)
		}
		etag = strings.Trim(etag, "\"")

		uploadParts = append(uploadParts, map[string]interface{}{
			"etag":        etag,
			"part_number": i + 1,
		})

		bytesUploaded += partSize

		// Report progress
		if options.OnProgress != nil {
			options.OnProgress(UploadProgress{
				CurrentPart:   i + 1,
				TotalParts:    calculatedParts,
				BytesUploaded: bytesUploaded,
				TotalBytes:    fileSize,
				Percentage:    int((bytesUploaded * 100) / fileSize),
			})
		}
	}

	// Step 3: Complete the multipart upload
	var completeResp struct {
		Data struct {
			Message string `json:"message"`
			FileKey string `json:"file_key"`
		} `json:"data"`
	}

	_, err = c.httpClient.R().
		SetBody(map[string]interface{}{
			"file_key":  fileKey,
			"upload_id": uploadID,
			"object_name": objectName,
			"parts":     uploadParts,
		}).
		SetResult(&completeResp).
		Post("/api/v1/complete-upload")

	if err != nil {
		return nil, fmt.Errorf("failed to complete upload: %w", err)
	}

	return &UploadResponse{
		FileKey:       fileKey,
		UploadID:      uploadID,
		PresignedURLs: presignedURLs,
		ObjectName:    objectName,
		FileKeyAlias:  fileKey,
		UploadIDAlias: uploadID,
		PresignedURLsAlias: presignedURLs,
		ObjectNameAlias:    objectName,
	}, nil
}

// CheckSupportedOperation checks if an operation is supported for a file extension
func (c *Dragdropdo) CheckSupportedOperation(options SupportedOperationOptions) (*SupportedOperationResponse, error) {
	if options.Ext == "" {
		return nil, errors.New("extension (ext) is required")
	}

	var resp struct {
		Data SupportedOperationResponse `json:"data"`
	}

	body := map[string]interface{}{
		"ext": options.Ext,
	}
	if options.Action != "" {
		body["action"] = options.Action
	}
	if options.Parameters != nil {
		body["parameters"] = options.Parameters
	}

	_, err := c.httpClient.R().
		SetBody(body).
		SetResult(&resp).
		Post("/api/v1/supported-operation")

	if err != nil {
		return nil, fmt.Errorf("failed to check supported operation: %w", err)
	}

	return &resp.Data, nil
}

// CreateOperation creates a file operation
func (c *Dragdropdo) CreateOperation(options OperationOptions) (*OperationResponse, error) {
	if options.Action == "" {
		return nil, errors.New("action is required")
	}
	if len(options.FileKeys) == 0 {
		return nil, errors.New("at least one file key is required")
	}

	var resp struct {
		Data struct {
			MainTaskID string `json:"main_task_id"`
		} `json:"data"`
	}

	body := map[string]interface{}{
		"action":    options.Action,
		"file_keys": options.FileKeys,
	}
	if options.Parameters != nil {
		body["parameters"] = options.Parameters
	}
	if options.Notes != nil {
		body["notes"] = options.Notes
	}

	_, err := c.httpClient.R().
		SetBody(body).
		SetResult(&resp).
		Post("/api/v1/do")

	if err != nil {
		return nil, fmt.Errorf("failed to create operation: %w", err)
	}

	// Map snake_case to camelCase
	mainTaskID := resp.Data.MainTaskID
	return &OperationResponse{
		MainTaskID:      mainTaskID,
		MainTaskIDAlias: mainTaskID,
	}, nil
}

// Convenience methods

// Convert converts files to a different format
func (c *Dragdropdo) Convert(fileKeys []string, convertTo string, notes map[string]string) (*OperationResponse, error) {
	return c.CreateOperation(OperationOptions{
		Action:   "convert",
		FileKeys: fileKeys,
		Parameters: map[string]interface{}{
			"convert_to": convertTo,
		},
		Notes: notes,
	})
}

// Compress compresses files
func (c *Dragdropdo) Compress(fileKeys []string, compressionValue string, notes map[string]string) (*OperationResponse, error) {
	if compressionValue == "" {
		compressionValue = "recommended"
	}
	return c.CreateOperation(OperationOptions{
		Action:   "compress",
		FileKeys: fileKeys,
		Parameters: map[string]interface{}{
			"compression_value": compressionValue,
		},
		Notes: notes,
	})
}

// Merge merges multiple files
func (c *Dragdropdo) Merge(fileKeys []string, notes map[string]string) (*OperationResponse, error) {
	return c.CreateOperation(OperationOptions{
		Action:   "merge",
		FileKeys: fileKeys,
		Notes:    notes,
	})
}

// Zip creates a ZIP archive from files
func (c *Dragdropdo) Zip(fileKeys []string, notes map[string]string) (*OperationResponse, error) {
	return c.CreateOperation(OperationOptions{
		Action:   "zip",
		FileKeys: fileKeys,
		Notes:    notes,
	})
}

// Share shares files (generates shareable links)
func (c *Dragdropdo) Share(fileKeys []string, notes map[string]string) (*OperationResponse, error) {
	return c.CreateOperation(OperationOptions{
		Action:   "share",
		FileKeys: fileKeys,
		Notes:    notes,
	})
}

// LockPdf locks PDF with password
func (c *Dragdropdo) LockPdf(fileKeys []string, password string, notes map[string]string) (*OperationResponse, error) {
	return c.CreateOperation(OperationOptions{
		Action:   "lock",
		FileKeys: fileKeys,
		Parameters: map[string]interface{}{
			"password": password,
		},
		Notes: notes,
	})
}

// UnlockPdf unlocks PDF with password
func (c *Dragdropdo) UnlockPdf(fileKeys []string, password string, notes map[string]string) (*OperationResponse, error) {
	return c.CreateOperation(OperationOptions{
		Action:   "unlock",
		FileKeys: fileKeys,
		Parameters: map[string]interface{}{
			"password": password,
		},
		Notes: notes,
	})
}

// ResetPdfPassword resets PDF password
func (c *Dragdropdo) ResetPdfPassword(fileKeys []string, oldPassword, newPassword string, notes map[string]string) (*OperationResponse, error) {
	return c.CreateOperation(OperationOptions{
		Action:   "reset_password",
		FileKeys: fileKeys,
		Parameters: map[string]interface{}{
			"old_password": oldPassword,
			"new_password": newPassword,
		},
		Notes: notes,
	})
}

// GetStatus gets operation status
func (c *Dragdropdo) GetStatus(options StatusOptions) (*StatusResponse, error) {
	if options.MainTaskID == "" {
		return nil, errors.New("main_task_id is required")
	}

	url := fmt.Sprintf("/api/v1/status/%s", options.MainTaskID)
	if options.FileTaskID != "" {
		url += fmt.Sprintf("/%s", options.FileTaskID)
	}

	var resp struct {
		Data struct {
			OperationStatus string `json:"operation_status"`
			FilesData        []struct {
				FileKey      string `json:"file_key"`
				Status       string `json:"status"`
				DownloadLink string `json:"download_link,omitempty"`
				ErrorCode    string `json:"error_code,omitempty"`
				ErrorMessage string `json:"error_message,omitempty"`
			} `json:"files_data"`
		} `json:"data"`
	}

	_, err := c.httpClient.R().
		SetResult(&resp).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	// Map snake_case to camelCase
	filesData := make([]FileTaskStatus, len(resp.Data.FilesData))
	for i, file := range resp.Data.FilesData {
		filesData[i] = FileTaskStatus{
			FileKey:      file.FileKey,
			Status:       file.Status,
			DownloadLink: file.DownloadLink,
			ErrorCode:    file.ErrorCode,
			ErrorMessage: file.ErrorMessage,
		}
	}

	normalizedStatus := strings.ToLower(resp.Data.OperationStatus)
	for i := range filesData {
		filesData[i].Status = strings.ToLower(filesData[i].Status)
	}

	return &StatusResponse{
		OperationStatus:      normalizedStatus,
		FilesData:            filesData,
		OperationStatusAlias: normalizedStatus,
		FilesDataAlias:       filesData,
	}, nil
}

// PollStatus polls operation status until completion or failure
func (c *Dragdropdo) PollStatus(options PollStatusOptions) (*StatusResponse, error) {
	interval := options.Interval
	if interval == 0 {
		interval = 2 * time.Second
	}
	timeout := options.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	startTime := time.Now()

	for {
		// Check timeout
		if time.Since(startTime) > timeout {
			return nil, fmt.Errorf("polling timed out after %v", timeout)
		}

		// Get status
		status, err := c.GetStatus(options.StatusOptions)
		if err != nil {
			return nil, err
		}

		// Call update callback
		if options.OnUpdate != nil {
			options.OnUpdate(*status)
		}

		// Check if completed or failed
		if status.OperationStatus == "completed" || status.OperationStatus == "failed" {
			return status, nil
		}

		// Wait before next poll
		time.Sleep(interval)
	}
}

// getMimeType gets MIME type from file extension
func (c *Dragdropdo) getMimeType(ext string) string {
	mimeTypes := map[string]string{
		".pdf":  "application/pdf",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xls":  "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".zip":  "application/zip",
		".txt":  "text/plain",
		".mp4":  "video/mp4",
		".mp3":  "audio/mpeg",
	}

	return mimeTypes[strings.ToLower(ext)]
}

