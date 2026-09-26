package usecase

import (
	"path/filepath"
	"strings"
)

// AvatarURLPrefix — публичный путь, по которому раздаются файлы из <uploadDir>/avatars.
// В avatar_url храним именно его + имя файла, чтобы фронт мог сразу подставить в <img src>.
const AvatarURLPrefix = "/uploads/avatars/"

// avatarURL собирает публичную ссылку на файл аватарки по его имени.
func avatarURL(filename string) string {
	return AvatarURLPrefix + filename
}

// safeFileName убирает из имени файла всё, что может сломать URL (пробелы, кириллицу, спецсимволы).
func safeFileName(name string) string {
	name = filepath.Base(name)
	ext := strings.ToLower(filepath.Ext(name))
	base := strings.TrimSuffix(name, filepath.Ext(name))

	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}

	clean := strings.Trim(b.String(), "_")
	if clean == "" {
		clean = "avatar"
	}
	if len(clean) > 50 {
		clean = clean[:50]
	}

	// Расширение оставляем, только если оно нормальное (.jpg, .png, .webp и т.п.).
	if len(ext) < 2 || len(ext) > 6 {
		return clean
	}
	for _, r := range ext[1:] {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			return clean
		}
	}
	return clean + ext
}
