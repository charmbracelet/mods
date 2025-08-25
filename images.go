package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/mods/internal/proto"
)

// supportedImageFormats maps file extensions to MIME types
var supportedImageFormats = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
}

// maxImageSize sets the maximum allowed image file size (5MB)
const maxImageSize = 5 * 1024 * 1024

// processImageFiles reads and validates image files from paths
func processImageFiles(imagePaths []string) ([]proto.ImageContent, error) {
	if len(imagePaths) == 0 {
		return nil, nil
	}

	if len(imagePaths) > 10 {
		return nil, fmt.Errorf("too many images: maximum 10 images allowed, got %d", len(imagePaths))
	}

	var images []proto.ImageContent
	for _, path := range imagePaths {
		image, err := readImageFile(path)
		if err != nil {
			return nil, fmt.Errorf("error processing image %s: %w", path, err)
		}
		images = append(images, *image)
	}

	return images, nil
}

// readImageFile reads and validates a single image file
func readImageFile(path string) (*proto.ImageContent, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("image file does not exist: %s", path)
	}

	// Get file info for size check
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("could not get file info: %w", err)
	}

	if fileInfo.Size() > maxImageSize {
		return nil, fmt.Errorf("image file too large: %s (%.2f MB > 5 MB)",
			path, float64(fileInfo.Size())/(1024*1024))
	}

	// Detect MIME type from extension
	ext := strings.ToLower(filepath.Ext(path))
	mimeType, supported := supportedImageFormats[ext]
	if !supported {
		return nil, fmt.Errorf("unsupported image format: %s (supported: %s)",
			ext, getSupportedFormats())
	}

	// Read file data
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("could not read file: %w", err)
	}

	return &proto.ImageContent{
		Data:     data,
		MimeType: mimeType,
		Filename: filepath.Base(path),
	}, nil
}

// getSupportedFormats returns a comma-separated list of supported formats
func getSupportedFormats() string {
	var formats []string
	for ext := range supportedImageFormats {
		formats = append(formats, ext)
	}
	return strings.Join(formats, ", ")
}

// createDataURI creates a data URI for base64 encoded image data
func createDataURI(image proto.ImageContent) string {
	base64Data := base64.StdEncoding.EncodeToString(image.Data)
	return fmt.Sprintf("data:%s;base64,%s", image.MimeType, base64Data)
}

// hasImages checks if any message contains images
func hasImages(messages []proto.Message) bool {
	for _, msg := range messages {
		if len(msg.Images) > 0 {
			return true
		}
	}
	return false
}
