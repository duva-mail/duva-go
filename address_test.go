package duva_test

import (
	"testing"

	"github.com/duva-mail/duva-go"
)

func TestFormatAddress(t *testing.T) {
	cases := []struct{ address, name, want string }{
		{"a@example.com", "", "a@example.com"},
		{"a@example.com", "Example", "Example <a@example.com>"},
		{"a@example.com", `Say "hi"`, `"Say \"hi\"" <a@example.com>`},
		{"a@example.com", "Doe, Jane", `"Doe, Jane" <a@example.com>`},
	}
	for _, c := range cases {
		if got := duva.FormatAddress(c.address, c.name); got != c.want {
			t.Errorf("FormatAddress(%q, %q) = %q, want %q", c.address, c.name, got, c.want)
		}
	}
}

func TestUnsubscribeHeadersNeedsAtLeastOneLink(t *testing.T) {
	if _, err := duva.UnsubscribeHeaders(duva.UnsubscribeOptions{}); err == nil {
		t.Fatal("expected an error when neither HTTPSURL nor Mailto is given")
	}
}

func TestUnsubscribeHeadersRejectsAnInsecureURL(t *testing.T) {
	if _, err := duva.UnsubscribeHeaders(duva.UnsubscribeOptions{HTTPSURL: "http://example.org/u"}); err == nil {
		t.Fatal("expected a non-https URL to be rejected")
	}
}

func TestUnsubscribeHeadersOneClickDefaultsOnWithAnHTTPSURL(t *testing.T) {
	headers, err := duva.UnsubscribeHeaders(duva.UnsubscribeOptions{HTTPSURL: "https://example.org/u"})
	if err != nil {
		t.Fatalf("UnsubscribeHeaders: %v", err)
	}
	if headers.Get("List-Unsubscribe") != "<https://example.org/u>" {
		t.Errorf("unexpected List-Unsubscribe: %s", headers.Get("List-Unsubscribe"))
	}
	if headers.Get("List-Unsubscribe-Post") != "List-Unsubscribe=One-Click" {
		t.Errorf("expected one-click header by default")
	}
}

func TestUnsubscribeHeadersMailtoOnly(t *testing.T) {
	headers, err := duva.UnsubscribeHeaders(duva.UnsubscribeOptions{Mailto: "unsub@example.org"})
	if err != nil {
		t.Fatalf("UnsubscribeHeaders: %v", err)
	}
	if headers.Get("List-Unsubscribe") != "<mailto:unsub@example.org>" {
		t.Errorf("unexpected List-Unsubscribe: %s", headers.Get("List-Unsubscribe"))
	}
	if headers.Get("List-Unsubscribe-Post") != "" {
		t.Errorf("expected no one-click header without an HTTPS URL")
	}
}

func TestUnsubscribeHeadersOneClickWithoutHTTPSURLIsRejected(t *testing.T) {
	oneClick := true
	if _, err := duva.UnsubscribeHeaders(duva.UnsubscribeOptions{Mailto: "u@example.org", OneClick: &oneClick}); err == nil {
		t.Fatal("expected one-click without an HTTPS URL to be rejected")
	}
}
