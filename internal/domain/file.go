package domain

import "time"

type File struct {
	ID         int64     `json:"id"`
	Filename   string    `json:"filename"`
	FilePath   string    `json:"file_path"`
	FileSize   int64     `json:"file_size"`
	MimeType   string    `json:"mime_type"`
	UploaderID int64     `json:"uploader_id"`
	CreatedAt  time.Time `json:"created_at"`
}
