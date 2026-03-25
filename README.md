# DragDropDo Go SDK

Official Go client library for the D3 Business API. This library provides a simple and elegant interface for developers to interact with D3's file processing services.

## Features

- ✅ **File Upload** - Upload files with automatic multipart handling
- ✅ **Operation Support** - Check which operations are available for file types
- ✅ **File Operations** - Convert, compress, merge, zip, and more
- ✅ **Status Polling** - Built-in polling for operation status
- ✅ **Error Handling** - Comprehensive error types and messages
- ✅ **Progress Tracking** - Upload progress callbacks

## Installation

```bash
go get github.com/dragdropdo/dragdropdo-sdk-go
```

## Quick Start

```go
package main

import (
    "fmt"
    "time"
    "github.com/dragdropdo/dragdropdo-sdk-go"
)

func main() {
    // Initialize the client
    client, err := d3.NewDragdropdo(d3.Config{
        APIKey:  "your-api-key-here",
        BaseURL: "https://api.dragdropdo.com", 
        Timeout: 30 * time.Second,    
    })
    if err != nil {
        panic(err)
    }

    // Upload a file
    uploadResult, err := client.UploadFile(d3.UploadFileOptions{
        File:     "/path/to/document.pdf",
        FileName: "document.pdf",
        MimeType: "application/pdf",
    })
    if err != nil {
        panic(err)
    }

    fmt.Printf("File key: %s\n", uploadResult.FileKey)

    // Check if convert to PNG is supported
    supported, err := client.CheckSupportedOperation(d3.SupportedOperationOptions{
        Ext:    "pdf",
        Action: "convert",
        Parameters: map[string]interface{}{
            "convert_to": "png",
        },
    })
    if err != nil {
        panic(err)
    }

    if supported.Supported {
        // Convert PDF to PNG
        operation, err := client.Convert([]string{uploadResult.FileKey}, "png", nil)
        if err != nil {
            panic(err)
        }

        // Poll for completion
        status, err := client.PollStatus(d3.PollStatusOptions{
            StatusOptions: d3.StatusOptions{
                MainTaskID: operation.MainTaskID,
            },
            Interval: 2 * time.Second, // Check every 2 seconds
            OnUpdate: func(status d3.StatusResponse) {
                fmt.Printf("Status: %s\n", status.OperationStatus)
            },
        })
        if err != nil {
            panic(err)
        }

        if status.OperationStatus == "completed" {
            fmt.Println("Download links:")
            for _, file := range status.FilesData {
                fmt.Printf("  %s\n", file.DownloadLink)
            }
        }
    }
}
```

## API Reference

### Initialization

#### `NewDragdropdo(config Config) (*Dragdropdo, error)`

Create a new D3 client instance.

**Parameters:**

- `APIKey` (required) - Your D3 API key
- `BaseURL` (optional) - Base URL of the D3 API (default: `"https://api.d3.com"`)
- `Timeout` (optional) - Request timeout (default: `30 * time.Second`)
- `Headers` (optional) - Custom headers to include in all requests

**Example:**

```go
client, err := d3.NewDragdropdo(d3.Config{
    APIKey:  "your-api-key",
    BaseURL: "https://api.d3.com",
    Timeout: 30 * time.Second,
})
```

---

### File Upload

#### `UploadFile(options UploadFileOptions) (*UploadResponse, error)`

Upload a file to D3 storage. This method handles the complete upload flow including multipart uploads.

**Parameters:**

- `File` (required) - File path (string)
- `FileName` (required) - Original file name
- `MimeType` (optional) - MIME type (auto-detected if not provided)
- `Parts` (optional) - Number of parts for multipart upload (auto-calculated if not provided)
- `OnProgress` (optional) - Progress callback function

**Returns:** `*UploadResponse` with `FileKey` and `PresignedURLs`

**Example:**

```go
result, err := client.UploadFile(d3.UploadFileOptions{
    File:     "/path/to/file.pdf",
    FileName: "document.pdf",
    MimeType: "application/pdf",
    OnProgress: func(progress d3.UploadProgress) {
        fmt.Printf("Upload: %d%%\n", progress.Percentage)
    },
})
```

---

### Check Supported Operations

#### `CheckSupportedOperation(options SupportedOperationOptions) (*SupportedOperationResponse, error)`

Check which operations are supported for a file extension.

**Parameters:**

- `Ext` (required) - File extension (e.g., `"pdf"`, `"jpg"`)
- `Action` (optional) - Specific action to check (e.g., `"convert"`, `"compress"`)
- `Parameters` (optional) - Parameters for validation (e.g., `map[string]interface{}{"convert_to": "png"}`)

**Returns:** `*SupportedOperationResponse` with support information

**Example:**

```go
// Get all available actions for PDF
result, err := client.CheckSupportedOperation(d3.SupportedOperationOptions{
    Ext: "pdf",
})
fmt.Printf("Available actions: %v\n", result.AvailableActions)

// Check if convert to PNG is supported
result, err := client.CheckSupportedOperation(d3.SupportedOperationOptions{
    Ext:    "pdf",
    Action: "convert",
    Parameters: map[string]interface{}{
        "convert_to": "png",
    },
})
fmt.Printf("Supported: %t\n", result.Supported)
```

---

### Create Operations

#### `CreateOperation(options OperationOptions) (*OperationResponse, error)`

Create a file operation (convert, compress, merge, zip, etc.).

**Parameters:**

- `Action` (required) - Action to perform: `"convert"`, `"compress"`, `"merge"`, `"zip"`, `"lock"`, `"unlock"`, `"reset_password"`
- `FileKeys` (required) - Array of file keys from upload
- `Parameters` (optional) - Action-specific parameters
- `Notes` (optional) - User metadata

**Returns:** `*OperationResponse` with `MainTaskID`

**Example:**

```go
// Convert PDF to PNG
result, err := client.CreateOperation(d3.OperationOptions{
    Action:   "convert",
    FileKeys: []string{"file-key-123"},
    Parameters: map[string]interface{}{
        "convert_to": "png",
    },
    Notes: map[string]string{
        "userId": "user-123",
    },
})
```

#### Convenience Methods

The client also provides convenience methods for common operations:

**Convert:**

```go
client.Convert(fileKeys, convertTo, notes)
// Example: client.Convert([]string{"file-key-123"}, "png", nil)
```

**Compress:**

```go
client.Compress(fileKeys, compressionValue, notes)
// Example: client.Compress([]string{"file-key-123"}, "recommended", nil)
```

**Merge:**

```go
client.Merge(fileKeys, notes)
// Example: client.Merge([]string{"file-key-1", "file-key-2"}, nil)
```

**Zip:**

```go
client.Zip(fileKeys, notes)
// Example: client.Zip([]string{"file-key-1", "file-key-2"}, nil)
```

**Lock PDF:**

```go
client.LockPdf(fileKeys, password, notes)
// Example: client.LockPdf([]string{"file-key-123"}, "secure-password", nil)
```

**Unlock PDF:**

```go
client.UnlockPdf(fileKeys, password, notes)
// Example: client.UnlockPdf([]string{"file-key-123"}, "password", nil)
```

**Reset PDF Password:**

```go
client.ResetPdfPassword(fileKeys, oldPassword, newPassword, notes)
// Example: client.ResetPdfPassword([]string{"file-key-123"}, "old", "new", nil)
```

---

### Get Status

#### `GetStatus(options StatusOptions) (*StatusResponse, error)`

Get the current status of an operation.

**Parameters:**

- `MainTaskID` (required) - Main task ID from operation creation
- `FileKey` (optional) - Input file key for specific file status

**Returns:** `*StatusResponse` with operation and file statuses

**Example:**

```go
// Get main task status
status, err := client.GetStatus(d3.StatusOptions{
    MainTaskID: "task-123",
})

// Get specific file status by file key
status, err := client.GetStatus(d3.StatusOptions{
    MainTaskID: "task-123",
    FileKey:    "file-key-456",
})

fmt.Printf("Operation status: %s\n", status.OperationStatus)
// Possible values: "queued", "running", "completed", "failed"
```

#### `PollStatus(options PollStatusOptions) (*StatusResponse, error)`

Poll operation status until completion or failure.

**Parameters:**

- `StatusOptions` (required) - Status options with `MainTaskID` and optional `FileKey`
- `Interval` (optional) - Polling interval (default: `2 * time.Second`)
- `Timeout` (optional) - Maximum polling duration (default: `5 * time.Minute`)
- `OnUpdate` (optional) - Callback for each status update

**Returns:** `*StatusResponse` with final status

**Example:**

```go
status, err := client.PollStatus(d3.PollStatusOptions{
    StatusOptions: d3.StatusOptions{
        MainTaskID: "task-123",
    },
    Interval: 2 * time.Second,
    Timeout:  5 * time.Minute,
    OnUpdate: func(status d3.StatusResponse) {
        fmt.Printf("Status: %s\n", status.OperationStatus)
    },
})

if status.OperationStatus == "completed" {
    fmt.Println("All files processed successfully!")
    for _, file := range status.FilesData {
        fmt.Printf("Download: %s\n", file.DownloadLink)
    }
}
```

---

## Complete Workflow Example

Here's a complete example showing the typical workflow:

```go
package main

import (
    "fmt"
    "os"
    "time"
    "github.com/dragdropdo/dragdropdo-sdk-go"
)

func main() {
    // Initialize client
    client, err := d3.NewDragdropdo(d3.Config{
        APIKey:  os.Getenv("D3_API_KEY"),
        BaseURL: "https://api.d3.com",
    })
    if err != nil {
        panic(err)
    }

    // Step 1: Upload file
    fmt.Println("Uploading file...")
    uploadResult, err := client.UploadFile(d3.UploadFileOptions{
        File:     "./document.pdf",
        FileName: "document.pdf",
        OnProgress: func(progress d3.UploadProgress) {
            fmt.Printf("Upload progress: %d%%\n", progress.Percentage)
        },
    })
    if err != nil {
        panic(err)
    }
    fmt.Printf("Upload complete. File key: %s\n", uploadResult.FileKey)

    // Step 2: Check if operation is supported
    fmt.Println("Checking supported operations...")
    supported, err := client.CheckSupportedOperation(d3.SupportedOperationOptions{
        Ext:    "pdf",
        Action: "convert",
        Parameters: map[string]interface{}{
            "convert_to": "png",
        },
    })
    if err != nil {
        panic(err)
    }

    if !supported.Supported {
        panic("Convert to PNG is not supported for PDF")
    }

    // Step 3: Create operation
    fmt.Println("Creating convert operation...")
    operation, err := client.Convert(
        []string{uploadResult.FileKey},
        "png",
        map[string]string{
            "userId": "user-123",
            "source": "api",
        },
    )
    if err != nil {
        panic(err)
    }
    fmt.Printf("Operation created. Task ID: %s\n", operation.MainTaskID)

    // Step 4: Poll for completion
    fmt.Println("Waiting for operation to complete...")
    status, err := client.PollStatus(d3.PollStatusOptions{
        StatusOptions: d3.StatusOptions{
            MainTaskID: operation.MainTaskID,
        },
        Interval: 2 * time.Second,
        OnUpdate: func(status d3.StatusResponse) {
            fmt.Printf("Status: %s\n", status.OperationStatus)
        },
    })
    if err != nil {
        panic(err)
    }

    // Step 5: Handle result
    if status.OperationStatus == "completed" {
        fmt.Println("Operation completed successfully!")
        for i, file := range status.FilesData {
            fmt.Printf("File %d:\n", i+1)
            fmt.Printf("  Status: %s\n", file.Status)
            fmt.Printf("  Download: %s\n", file.DownloadLink)
        }
    } else {
        fmt.Println("Operation failed")
        for _, file := range status.FilesData {
            if file.ErrorMessage != "" {
                fmt.Printf("Error: %s\n", file.ErrorMessage)
            }
        }
    }
}
```

---

## Error Handling

The client provides several error types for better error handling:

```go
import "github.com/dragdropdo/dragdropdo-sdk-go"

result, err := client.UploadFile(...)
if err != nil {
    if d3.IsD3APIError(err) {
        // API returned an error
        apiErr := err.(*d3.D3APIError)
        fmt.Printf("API Error (%d): %s\n", *apiErr.StatusCode, apiErr.Message)
        if apiErr.Code != nil {
            fmt.Printf("Error code: %d\n", *apiErr.Code)
        }
    } else if d3.IsD3ValidationError(err) {
        // Validation error (missing required fields, etc.)
        fmt.Printf("Validation Error: %s\n", err.Error())
    } else if d3.IsD3UploadError(err) {
        // Upload-specific error
        fmt.Printf("Upload Error: %s\n", err.Error())
    } else if d3.IsD3TimeoutError(err) {
        // Timeout error (from polling)
        fmt.Printf("Timeout: %s\n", err.Error())
    } else {
        // Other errors
        fmt.Printf("Error: %s\n", err.Error())
    }
}
```

---

## Requirements

- Go 1.19 or higher

---

## License

ISC

---

## Support

For API documentation and support, visit [D3 Developer Portal](https://developer.d3.com).
