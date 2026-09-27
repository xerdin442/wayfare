package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"slices"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/xerdin442/wayfare/shared/util"
)

// MaxImageSize is the largest image accepted for upload (5MB)
const MaxImageSize = 5 << 20

type FileUploadConfig struct {
	Folder      string
	CloudName   string
	ApiKey      string
	CloudSecret string
}

func isHEIC(header []byte) bool {
	if len(header) < 12 || !bytes.Equal(header[4:8], []byte("ftyp")) {
		return false
	}

	brands := []string{"heic", "heix", "hevc", "hevx", "heim", "heis", "mif1", "msf1"}
	return slices.Contains(brands, string(header[8:12]))
}

func parseImageMimetype(file multipart.File) error {
	buffer := make([]byte, 512)
	n, err := io.ReadFull(file, buffer)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return fmt.Errorf("failed to read file header: %w", err)
	}
	header := buffer[:n]

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to rewind file: %w", err)
	}

	if isHEIC(header) {
		return nil
	}

	supportedTypes := []string{"image/jpeg", "image/png", "image/webp"}
	if !slices.Contains(supportedTypes, http.DetectContentType(header)) {
		return util.ErrUnsupportedFileType
	}

	return nil
}

func ValidateImage(part *multipart.FileHeader) error {
	if part.Size > MaxImageSize {
		return util.ErrFileTooLarge
	}

	file, err := part.Open()
	if err != nil {
		return err
	}
	defer file.Close()

	return parseImageMimetype(file)
}

func uploadImage(ctx context.Context, cfg *FileUploadConfig, part *multipart.FileHeader, path string, deliveryType api.DeliveryType) (*uploader.UploadResult, error) {
	if err := ValidateImage(part); err != nil {
		return nil, err
	}

	file, err := part.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return upload(ctx, cfg, file, path, deliveryType)
}

func upload(ctx context.Context, cfg *FileUploadConfig, file any, path string, deliveryType api.DeliveryType) (*uploader.UploadResult, error) {
	cld, err := cloudinary.NewFromParams(cfg.CloudName, cfg.ApiKey, cfg.CloudSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to init cloudinary instance: %v", err)
	}

	result, err := cld.Upload.Upload(ctx, file, uploader.UploadParams{
		Folder: cfg.Folder + path,
		Type:   deliveryType,
	})
	if err != nil {
		return nil, err
	}

	if result.Error.Message != "" {
		return nil, fmt.Errorf("cloudinary upload failed: %s", result.Error.Message)
	}

	return result, nil
}

func ProcessFileUpload(ctx context.Context, cfg *FileUploadConfig, part *multipart.FileHeader, path string) (string, error) {
	result, err := uploadImage(ctx, cfg, part, path, api.Upload)
	if err != nil {
		return "", err
	}

	return result.SecureURL, nil
}

func UploadFromURL(ctx context.Context, cfg *FileUploadConfig, sourceURL string, path string) (string, error) {
	result, err := upload(ctx, cfg, sourceURL, path, api.Upload)
	if err != nil {
		return "", err
	}

	return result.SecureURL, nil
}

func ProcessPrivateFileUpload(ctx context.Context, cfg *FileUploadConfig, part *multipart.FileHeader, path string) (string, error) {
	result, err := uploadImage(ctx, cfg, part, path, api.Authenticated)
	if err != nil {
		return "", err
	}

	return result.PublicID, nil
}
