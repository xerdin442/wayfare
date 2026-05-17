package storage

import (
	"context"
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

func ParseImageMimetype(file multipart.File) error {
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

func ProcessFileUpload(ctx context.Context, file multipart.File, cfg *FileUploadConfig, subfolder string) (*uploader.UploadResult, error) {
	cld, _ := cloudinary.NewFromParams(cfg.CloudName, cfg.ApiKey, cfg.CloudSecret)
	return cld.Upload.Upload(ctx, file, uploader.UploadParams{
		Folder: cfg.Folder + subfolder,
	})
}
