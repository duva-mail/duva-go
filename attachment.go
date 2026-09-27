// Attachment helpers. The server remains the authority on the limits below (they can change):
// these are a courtesy, so a mistake fails locally instead of after an upload.
package duva

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Mirrors services/messages.py in the duva repository at the time of writing: 10 attachments,
// 5 MB decoded in total, these extensions refused. Re-check against docs/api.md if this drifts.
const (
	MaxAttachments          = 10
	MaxAttachmentTotalBytes = 5 * 1024 * 1024
)

var forbiddenAttachmentExtensions = map[string]bool{
	".exe": true, ".bat": true, ".cmd": true, ".com": true, ".js": true, ".vbs": true,
	".vbe": true, ".scr": true, ".msi": true, ".msp": true, ".ps1": true, ".jar": true,
}

// AttachmentOption sets an optional field on an Attachment built by NewAttachmentFromBytes or
// NewAttachmentFromFile.
type AttachmentOption func(*Attachment)

// WithContentType sets the attachment's MIME type (guessed by the server when omitted).
func WithContentType(contentType string) AttachmentOption {
	return func(a *Attachment) { a.ContentType = &contentType }
}

// WithContentID turns the attachment into an inline image the HTML body references with cid:.
func WithContentID(contentID string) AttachmentOption {
	return func(a *Attachment) { a.ContentId = &contentID }
}

// NewAttachmentFromBytes builds an Attachment from bytes already in memory.
func NewAttachmentFromBytes(filename string, content []byte, opts ...AttachmentOption) (Attachment, error) {
	if err := assertAllowedFilename(filename); err != nil {
		return Attachment{}, err
	}
	a := Attachment{Filename: filename, Content: base64.StdEncoding.EncodeToString(content)}
	for _, opt := range opts {
		opt(&a)
	}
	return a, nil
}

// NewAttachmentFromFile reads a file from disk.
func NewAttachmentFromFile(path string, opts ...AttachmentOption) (Attachment, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- the caller chooses this path
	if err != nil {
		return Attachment{}, err
	}
	return NewAttachmentFromBytes(filepath.Base(path), data, opts...)
}

func assertAllowedFilename(filename string) error {
	if strings.ContainsAny(filename, `/\`) {
		return fmt.Errorf("duva: attachment filename must not contain a path: %s", filename)
	}
	if forbiddenAttachmentExtensions[strings.ToLower(filepath.Ext(filename))] {
		return fmt.Errorf("duva: executable attachments are refused: %s", filename)
	}
	return nil
}

// assertAttachmentLimits are local, courtesy-only checks: at most MaxAttachments attachments, at
// most MaxAttachmentTotalBytes decoded in total. The server re-checks regardless.
func assertAttachmentLimits(attachments []Attachment) error {
	if len(attachments) > MaxAttachments {
		return fmt.Errorf("duva: at most %d attachments per message", MaxAttachments)
	}
	total := 0
	for _, a := range attachments {
		total += decodedLength(a.Content)
	}
	if total > MaxAttachmentTotalBytes {
		return fmt.Errorf(
			"duva: attachments are %d bytes decoded, over the %d limit", total, MaxAttachmentTotalBytes,
		)
	}
	return nil
}

func decodedLength(b64 string) int {
	start := len(b64) - 2
	if start < 0 {
		start = 0
	}
	padding := strings.Count(b64[start:], "=")
	return (len(b64)*3)/4 - padding
}
