package duva

const hexDigits = "0123456789ABCDEF"

// pathEscape percent-encodes every byte that isn't RFC 3986 "unreserved" (ALPHA / DIGIT / "-" /
// "." / "_" / "~"). Stricter than net/url.PathEscape, which leaves sub-delimiters and "@" alone
// (both valid in a path segment per RFC 3986): this matches the other official libraries'
// encoder (Python's urllib.parse.quote(safe=""), for instance), so a domain name or an email
// address in the path is escaped identically across languages.
func pathEscape(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isUnreserved(c) {
			out = append(out, c)
			continue
		}
		out = append(out, '%', hexDigits[c>>4], hexDigits[c&0x0f])
	}
	return string(out)
}

func isUnreserved(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
		c == '-' || c == '.' || c == '_' || c == '~'
}
