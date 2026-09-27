package duva_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/duva-mail/duva-go"
)

func TestNewAttachmentFromBytes(t *testing.T) {
	a, err := duva.NewAttachmentFromBytes("invoice.pdf", []byte("hello"), duva.WithContentType("application/pdf"))
	if err != nil {
		t.Fatalf("NewAttachmentFromBytes: %v", err)
	}
	if a.Filename != "invoice.pdf" || a.Content == "" {
		t.Fatalf("unexpected attachment: %#v", a)
	}
	if a.ContentType == nil || *a.ContentType != "application/pdf" {
		t.Fatalf("expected content type to be set")
	}
}

func TestNewAttachmentRejectsAPathInTheFilename(t *testing.T) {
	if _, err := duva.NewAttachmentFromBytes("../etc/passwd", []byte("x")); err == nil {
		t.Fatal("expected a path in the filename to be rejected")
	}
}

func TestNewAttachmentRejectsAnExecutableExtension(t *testing.T) {
	if _, err := duva.NewAttachmentFromBytes("virus.exe", []byte("x")); err == nil {
		t.Fatal("expected an executable extension to be rejected")
	}
}

func TestSendRejectsTooManyAttachments(t *testing.T) {
	var attachments []duva.Attachment
	for i := 0; i < duva.MaxAttachments+1; i++ {
		a, err := duva.NewAttachmentFromBytes("f.txt", []byte("x"))
		if err != nil {
			t.Fatalf("NewAttachmentFromBytes: %v", err)
		}
		attachments = append(attachments, a)
	}
	client, transport := fakeClient(t, func(req *http.Request) (*http.Response, error) {
		t.Fatal("should not have sent a request")
		return nil, nil
	})
	_, err := client.Messages.Send(context.Background(), duva.SendMessageParams{
		From: "a@example.com", To: []string{"b@example.org"}, Subject: "s", Text: "t", Attachments: attachments,
	})
	if err == nil {
		t.Fatal("expected the local attachment-count limit to be enforced")
	}
	if len(transport.Requests) != 0 {
		t.Fatal("expected no request to have been sent")
	}
}

func TestSendRejectsAttachmentsOverTheTotalSizeLimit(t *testing.T) {
	big := strings.Repeat("x", duva.MaxAttachmentTotalBytes+1)
	a, err := duva.NewAttachmentFromBytes("f.txt", []byte(big))
	if err != nil {
		t.Fatalf("NewAttachmentFromBytes: %v", err)
	}
	client, transport := fakeClient(t, func(req *http.Request) (*http.Response, error) {
		t.Fatal("should not have sent a request")
		return nil, nil
	})
	_, err = client.Messages.Send(context.Background(), duva.SendMessageParams{
		From: "a@example.com", To: []string{"b@example.org"}, Subject: "s", Text: "t",
		Attachments: []duva.Attachment{a},
	})
	if err == nil {
		t.Fatal("expected the local total-size limit to be enforced")
	}
	if len(transport.Requests) != 0 {
		t.Fatal("expected no request to have been sent")
	}
}
