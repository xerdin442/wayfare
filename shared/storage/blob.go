package storage

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"slices"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/xerdin442/wayfare/shared/util"
)

type FileUploadConfig struct {
	Folder      string
	CloudName   string
	ApiKey      string
	CloudSecret string
}

func parseImageMimetype(file multipart.File) error {
	buffer := make([]byte, 512)
	file.Read(buffer)

	// Detect MIME type of file
	contentType := http.DetectContentType(buffer)

	// Reset file pointer
	file.Seek(0, io.SeekStart)

	// Verify if the MIME type is supported
	supportedTypes := []string{"image/jpeg", "image/png", "image/heic", "image/webp", "image/jpg"}
	if !slices.Contains(supportedTypes, contentType) {
		return util.ErrUnsupportedFileType
	}

	return nil
}

func ProcessFileUpload(ctx context.Context, cfg *FileUploadConfig, part *multipart.FileHeader, path string) (string, error) {
	file, err := part.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	if err := parseImageMimetype(file); err != nil {
		return "", err
	}

	cld, err := cloudinary.NewFromParams(cfg.CloudName, cfg.ApiKey, cfg.CloudSecret)
	if err != nil {
		return "", fmt.Errorf("failed to init cloudinary instance: %v", err)
	}

	result, err := cld.Upload.Upload(ctx, file, uploader.UploadParams{
		Folder: cfg.Folder + path,
	})
	if err != nil {
		return "", err
	}

	return result.SecureURL, nil
}
