// Small formatting helpers matched to the API's own parsing rules (docs/api.md in the duva
// repository).
package duva

import (
	"fmt"
	"net/http"
	"strings"
)

// FormatAddress returns "Name <address>" (Name quoted if it contains a `"` or `,`), or just
// address when name is empty.
func FormatAddress(address, name string) string {
	if name == "" {
		return address
	}
	escaped := strings.ReplaceAll(name, `"`, `\"`)
	quoted := name
	if strings.ContainsAny(name, `",`) {
		quoted = `"` + escaped + `"`
	}
	return quoted + " <" + address + ">"
}

// UnsubscribeOptions configures UnsubscribeHeaders.
type UnsubscribeOptions struct {
	HTTPSURL string
	Mailto   string
	// OneClick adds List-Unsubscribe-Post (RFC 8058). Defaults to true when HTTPSURL is set,
	// false otherwise.
	OneClick *bool
}

// UnsubscribeHeaders builds List-Unsubscribe (and List-Unsubscribe-Post for one-click) exactly as
// the API validates them: at most 3 links, https:// or mailto: only.
func UnsubscribeHeaders(opts UnsubscribeOptions) (http.Header, error) {
	if opts.HTTPSURL == "" && opts.Mailto == "" {
		return nil, fmt.Errorf("duva: UnsubscribeHeaders needs HTTPSURL and/or Mailto")
	}
	if opts.HTTPSURL != "" && !strings.HasPrefix(opts.HTTPSURL, "https://") {
		return nil, fmt.Errorf("duva: UnsubscribeHeaders.HTTPSURL must be an https:// link")
	}
	oneClick := opts.HTTPSURL != ""
	if opts.OneClick != nil {
		oneClick = *opts.OneClick
	}

	var links []string
	if opts.HTTPSURL != "" {
		links = append(links, "<"+opts.HTTPSURL+">")
	}
	if opts.Mailto != "" {
		links = append(links, "<mailto:"+opts.Mailto+">")
	}
	headers := http.Header{}
	headers.Set("List-Unsubscribe", strings.Join(links, ", "))
	if oneClick {
		if opts.HTTPSURL == "" {
			return nil, fmt.Errorf("duva: one-click unsubscribe (List-Unsubscribe-Post) needs HTTPSURL")
		}
		headers.Set("List-Unsubscribe-Post", "List-Unsubscribe=One-Click")
	}
	return headers, nil
}
