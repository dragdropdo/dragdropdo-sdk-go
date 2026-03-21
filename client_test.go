package d3

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const API_BASE = "https://api-dev.dragdropdo.com"

func TestClient_UploadFile_MultipartFlow(t *testing.T) {
	// Create a temporary test file
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, "d3-test-upload.pdf")
	defer os.Remove(tmpFile)

	// Create a 6MB file
	sixMbContent := strings.Repeat("a", 6*1024*1024)
	if err := os.WriteFile(tmpFile, []byte(sixMbContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Mock server for part uploads (must be created first to get URL)
	partServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected PUT, got %s", r.Method)
			return
		}
		// Read and verify body
		body, _ := io.ReadAll(r.Body)
		if len(body) == 0 {
			t.Error("Expected non-empty body")
		}
		w.Header().Set("ETag", `"etag-part-1"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer partServer.Close()

	// Mock server for API requests
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/initiate-upload" {
			// Presigned URL request
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("Failed to decode request body: %v", err)
				return
			}

			response := map[string]interface{}{
				"data": map[string]interface{}{
					"file_key":       "file-key-123",
					"upload_id":      "upload-id-456",
					"presigned_urls": []string{partServer.URL + "/part1", partServer.URL + "/part2"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else if r.URL.Path == "/api/v1/complete-upload" {
			// Complete upload request
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("Failed to decode request body: %v", err)
				return
			}

			if body["file_key"] != "file-key-123" {
				t.Errorf("Expected file_key 'file-key-123', got '%v'", body["file_key"])
			}

			response := map[string]interface{}{
				"data": map[string]interface{}{
					"message":  "Upload completed successfully",
					"file_key": "file-key-123",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer apiServer.Close()

	// Create client with mock base URL
	client, err := NewDragdropdo(Config{
		APIKey:  "test-key",
		BaseURL: apiServer.URL,
		Timeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	result, err := client.UploadFile(UploadFileOptions{
		File:     tmpFile,
		FileName: "test.pdf",
		MimeType: "application/pdf",
		Parts:    2,
	})

	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	if result.FileKey != "file-key-123" {
		t.Errorf("Expected file_key 'file-key-123', got '%s'", result.FileKey)
	}
	if result.UploadID != "upload-id-456" {
		t.Errorf("Expected upload_id 'upload-id-456', got '%s'", result.UploadID)
	}
	if len(result.PresignedURLs) != 2 {
		t.Errorf("Expected 2 presigned URLs, got %d", len(result.PresignedURLs))
	}
}

func TestClient_CreateOperation_AndPollStatus(t *testing.T) {
	callCount := 0
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/do") {
			// Create operation
			if r.Method != "POST" {
				t.Errorf("Expected POST, got %s", r.Method)
				return
			}

			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("Failed to decode request body: %v", err)
				return
			}

			if body["action"] != "convert" {
				t.Errorf("Expected action 'convert', got '%v'", body["action"])
			}

			response := map[string]interface{}{
				"data": map[string]interface{}{
					"main_task_id": "task-123",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else if strings.HasPrefix(r.URL.Path, "/api/v1/status/") {
			// Get status
			if r.Method != "GET" {
				t.Errorf("Expected GET, got %s", r.Method)
				return
			}

			callCount++
			var response map[string]interface{}
			if callCount == 1 {
				// First call: queued
				response = map[string]interface{}{
					"data": map[string]interface{}{
						"operation_status": "queued",
						"files_data": []map[string]interface{}{
							{
								"file_key": "file-key-123",
								"status":   "queued",
							},
						},
					},
				}
			} else {
				// Second call: completed
				response = map[string]interface{}{
					"data": map[string]interface{}{
						"operation_status": "completed",
						"files_data": []map[string]interface{}{
							{
								"file_key":     "file-key-123",
								"status":       "completed",
								"download_link": "https://files.d3.com/output.png",
							},
						},
					},
				}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	client, err := NewDragdropdo(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Timeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create operation
	operation, err := client.Convert([]string{"file-key-123"}, "png", nil)
	if err != nil {
		t.Fatalf("Failed to create operation: %v", err)
	}

	if operation.MainTaskID != "task-123" {
		t.Errorf("Expected main_task_id 'task-123', got '%s'", operation.MainTaskID)
	}

	// Poll status
	status, err := client.PollStatus(PollStatusOptions{
		StatusOptions: StatusOptions{
			MainTaskID: operation.MainTaskID,
		},
		Interval: 5 * time.Millisecond,
		Timeout:  1 * time.Second,
	})

	if err != nil {
		t.Fatalf("Failed to poll status: %v", err)
	}

	if status.OperationStatus != "completed" {
		t.Errorf("Expected operation_status 'completed', got '%s'", status.OperationStatus)
	}

	if len(status.FilesData) == 0 || !strings.Contains(status.FilesData[0].DownloadLink, "files.d3.com") {
		t.Errorf("Expected download link, got '%s'", status.FilesData[0].DownloadLink)
	}
}

func TestClient_NewDragdropdo_Validation(t *testing.T) {
	// Test missing API key
	_, err := NewDragdropdo(Config{
		APIKey: "",
	})
	if err == nil {
		t.Error("Expected error for missing API key")
	}

	// Test valid client
	client, err := NewDragdropdo(Config{
		APIKey:  "test-key",
		BaseURL: "https://api-dev.dragdropdo.com",
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	if client == nil {
		t.Error("Client should not be nil")
	}
}

func TestClient_CheckSupportedOperation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/v1/supported-operation" {
			t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
			return
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
			return
		}

		if body["ext"] != "pdf" {
			t.Errorf("Expected ext 'pdf', got '%v'", body["ext"])
		}

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"supported":         true,
				"ext":               "pdf",
				"available_actions": []string{"convert", "compress", "merge"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := NewDragdropdo(Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	result, err := client.CheckSupportedOperation(SupportedOperationOptions{
		Ext: "pdf",
	})
	if err != nil {
		t.Fatalf("Failed to check supported operation: %v", err)
	}

	if !result.Supported {
		t.Error("Expected supported to be true")
	}

	if result.Ext != "pdf" {
		t.Errorf("Expected ext 'pdf', got '%s'", result.Ext)
	}
}

