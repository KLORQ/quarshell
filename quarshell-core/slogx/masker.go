package slogx

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"net/mail"
	"regexp"
	"strings"
	"unicode"
)

// MaskType defines the category of data being masked.
type MaskType int

const (
	// MaskDefault replaces the value with [MASKED].
	MaskDefault MaskType = iota
	// MaskEmail redacts email addresses while preserving parts for debugging.
	MaskEmail
	// MaskPhone redacts phone numbers while preserving prefix and suffix.
	MaskPhone
	// MaskCard redacts payment card numbers (PAN), keeping BIN + last4 when possible.
	MaskCard
	// MaskSecret completely hides the value as [SECRET].
	MaskSecret
	// MaskToken redacts bearer/API tokens; fingerprinting is explicit opt-in.
	MaskToken
	// MaskJWT redacts JWT strings, keeping header.alg hint when parseable.
	MaskJWT
	// MaskIBAN redacts IBAN, keeping country code + last4.
	MaskIBAN
	// MaskHash replaces the value with a stable sha256[:12] fingerprint (for correlation).
	MaskHash
	// MaskPartial keeps first/last rune clusters; middle is redacted.
	MaskPartial
)

// Masker applies redaction rules to attribute values.
type Masker interface {
	Mask(value any, mType MaskType) any
}

// DefaultMasker provides built-in redaction for common sensitive data types.
type DefaultMasker struct{}

// CorporateMasker extends DefaultMasker with auto-detection for unstructured strings
// (JWT, AWS keys, PEM blocks, bearer headers) even when keys are not pre-registered.
type CorporateMasker struct {
	DefaultMasker
	// Fingerprint when true, MaskSecret/MaskToken emit sha256 fingerprints instead of fixed labels.
	Fingerprint bool
}

// MaskMap associates attribute keys with masking strategies.
type MaskMap map[string]MaskType

// MaskRules is a fluent builder for masking configuration.
type MaskRules struct {
	rules MaskMap
}

// NewMaskRules creates a new builder.
func NewMaskRules() *MaskRules {
	return &MaskRules{rules: make(MaskMap)}
}

// Add registers a key with a masking strategy (case-insensitive match at apply time).
func (r *MaskRules) Add(key string, mType MaskType) *MaskRules {
	r.rules[normalizeKey(key)] = mType
	return r
}

// Merge copies rules from another builder.
func (r *MaskRules) Merge(other *MaskRules) *MaskRules {
	if other == nil {
		return r
	}
	maps.Copy(r.rules, other.rules)
	return r
}

// Keys exposes the underlying map (normalized keys).
func (r *MaskRules) Keys() MaskMap {
	return r.rules
}

// CorporateMaskRules returns a corporate-default key → strategy map covering
// common secret field names used in APIs, configs, and cloud credentials.
func CorporateMaskRules() *MaskRules {
	r := NewMaskRules()
	secretKeys := []string{
		"password", "passwd", "pwd", "secret", "client_secret", "private_key",
		"privatekey", "passphrase", "pin", "cvv", "cvc", "ssn", "tax_id",
	}
	tokenKeys := []string{
		"token", "access_token", "refresh_token", "id_token", "session",
		"session_id", "api_key", "apikey", "api-key", "auth", "authorization",
		"x-api-key", "x-auth-token", "cookie", "set-cookie", "proxy-authorization",
	}
	for _, k := range secretKeys {
		r.Add(k, MaskSecret)
	}
	for _, k := range tokenKeys {
		r.Add(k, MaskToken)
	}
	r.Add("email", MaskEmail).Add("mail", MaskEmail).Add("user_email", MaskEmail)
	r.Add("phone", MaskPhone).Add("mobile", MaskPhone).Add("msisdn", MaskPhone)
	r.Add("card", MaskCard).Add("pan", MaskCard).Add("card_number", MaskCard).Add("credit_card", MaskCard)
	r.Add("iban", MaskIBAN).Add("account_number", MaskPartial)
	r.Add("jwt", MaskJWT).Add("id_token_jwt", MaskJWT)
	return r
}

func normalizeKey(k string) string {
	k = strings.TrimSpace(k)
	k = strings.ToLower(k)
	k = strings.ReplaceAll(k, "-", "_")
	return k
}

// lookupMask finds a strategy for key, trying normalized and suffix matches
// (e.g. "user.password", "headers.authorization").
func lookupMask(keys MaskMap, key string) (MaskType, bool) {
	if len(keys) == 0 {
		return 0, false
	}
	nk := normalizeKey(key)
	if t, ok := keys[nk]; ok {
		return t, true
	}
	lower := strings.ToLower(strings.TrimSpace(key))
	if t, ok := keys[lower]; ok {
		return t, true
	}
	// dotted / nested attr keys
	if i := strings.LastIndexByte(nk, '.'); i >= 0 {
		if t, ok := keys[nk[i+1:]]; ok {
			return t, true
		}
	}
	if i := strings.LastIndexByte(lower, '.'); i >= 0 {
		if t, ok := keys[lower[i+1:]]; ok {
			return t, true
		}
	}
	// suffix match: "my_api_key" matches registered "api_key"
	for rk, t := range keys {
		nrk := normalizeKey(rk)
		if nk == nrk || lower == strings.ToLower(rk) {
			return t, true
		}
		if strings.HasSuffix(nk, "_"+nrk) || strings.HasSuffix(nk, "."+nrk) {
			return t, true
		}
	}
	return 0, false
}

// Mask processes the input value based on the specified MaskType.
func (m *DefaultMasker) Mask(value any, mType MaskType) any {
	valStr := fmt.Sprintf("%v", value)
	switch mType {
	case MaskEmail:
		return maskEmail(valStr)
	case MaskPhone:
		return maskPhone(valStr)
	case MaskCard:
		return maskCard(valStr)
	case MaskSecret:
		return "[SECRET]"
	case MaskToken:
		return maskToken(valStr, false)
	case MaskJWT:
		return maskJWT(valStr)
	case MaskIBAN:
		return maskIBAN(valStr)
	case MaskHash:
		return maskHash(valStr)
	case MaskPartial:
		return maskPartial(valStr)
	default:
		return "[MASKED]"
	}
}

// Mask applies corporate masking, with auto-detect for high-entropy secrets.
func (m *CorporateMasker) Mask(value any, mType MaskType) any {
	valStr := fmt.Sprintf("%v", value)
	if mType == MaskDefault || mType == MaskSecret || mType == MaskToken {
		if detected, ok := detectSecret(valStr); ok {
			mType = detected
		}
	}
	out := m.DefaultMasker.Mask(value, mType)
	if m.Fingerprint && (mType == MaskSecret || mType == MaskToken) {
		return maskToken(valStr, true)
	}
	return out
}

var (
	reAuditQuery  = regexp.MustCompile(`\?[^\s]+`)
	reAuditSecret = regexp.MustCompile(`(?i)\b(token|api[_-]?key|secret|password|authorization|cookie)=([^\s&]+)`)
	reAuditBearer = regexp.MustCompile(`(?i)\bbearer\s+[^\s]+`)
	reAWSKey      = regexp.MustCompile(`\b(AKIA|ASIA)[0-9A-Z]{16}\b`)
	rePEM         = regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)
	reBearer      = regexp.MustCompile(`(?i)^bearer\s+\S+`)
	reJWT         = regexp.MustCompile(`^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$`)
	reIBAN        = regexp.MustCompile(`(?i)\b[A-Z]{2}[0-9]{2}[A-Z0-9]{10,30}\b`)
	reCardLike    = regexp.MustCompile(`\b(?:\d[ -]*?){13,19}\b`)
)

// RedactAuditText removes query values and common inline credentials from
// unstructured audit details while preserving the rest of the explanation.
func RedactAuditText(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r", " "), "\n", " ")
	s = reAuditQuery.ReplaceAllString(s, "?[REDACTED]")
	s = reAuditSecret.ReplaceAllString(s, "$1=[SECRET]")
	s = reAuditBearer.ReplaceAllString(s, "Bearer [TOKEN]")
	if kind, ok := detectSecret(s); ok {
		return fmt.Sprint((&CorporateMasker{}).Mask(s, kind))
	}
	return s
}

func detectSecret(s string) (MaskType, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if rePEM.MatchString(s) {
		return MaskSecret, true
	}
	if reBearer.MatchString(s) {
		return MaskToken, true
	}
	if reJWT.MatchString(s) && looksLikeJWT(s) {
		return MaskJWT, true
	}
	if reAWSKey.MatchString(s) {
		return MaskToken, true
	}
	if reIBAN.MatchString(s) && looksLikeIBAN(s) {
		return MaskIBAN, true
	}
	if reCardLike.MatchString(s) && luhnOK(digitsOnly(s)) {
		return MaskCard, true
	}
	return 0, false
}

// looksLikeJWT reduces false positives for dotted identifiers (e.g. op=
// "cli.policy.check" / "mysql.AnalyticsStorage.TeamSummaries"). Real JWTs
// encode a JSON header, so the first segment is base64url starting with "eyJ".
func looksLikeJWT(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return false
	}
	// Minimum lengths: tiny dotted labels are never JWTs.
	if len(parts[0]) < 8 || len(parts[1]) < 8 || len(parts[2]) < 8 {
		return false
	}
	return strings.HasPrefix(parts[0], "eyJ")
}

func maskEmail(s string) string {
	if addr, err := mail.ParseAddress(s); err == nil {
		s = addr.Address
	}
	parts := strings.Split(s, "@")
	if len(parts) != 2 {
		return "***@***"
	}
	user := parts[0]
	domain := parts[1]
	if len(user) <= 2 {
		return user[:1] + "***@" + domain
	}
	return user[:2] + "***@" + domain
}

func maskPhone(s string) string {
	d := digitsOnly(s)
	if len(d) < 8 {
		return "***"
	}
	return d[:3] + strings.Repeat("*", len(d)-6) + d[len(d)-3:]
}

func maskCard(s string) string {
	d := digitsOnly(s)
	if len(d) < 12 {
		return "****"
	}
	return d[:4] + " **** **** " + d[len(d)-4:]
}

func maskToken(s string, fingerprint bool) string {
	s = strings.TrimSpace(s)
	if fingerprint {
		sum := sha256.Sum256([]byte(s))
		return "[TOKEN:" + hex.EncodeToString(sum[:6]) + "]"
	}
	return "[TOKEN]"
}

func maskJWT(s string) string {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return "[JWT]"
	}
	return "[JWT:" + truncate(parts[0], 8) + ".***." + truncate(parts[2], 6) + "]"
}

func maskIBAN(s string) string {
	s = strings.ToUpper(strings.ReplaceAll(s, " ", ""))
	if len(s) < 8 {
		return "[IBAN]"
	}
	return s[:4] + strings.Repeat("*", len(s)-8) + s[len(s)-4:]
}

func maskHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return "sha256:" + hex.EncodeToString(sum[:8])
}

func maskPartial(s string) string {
	r := []rune(s)
	if len(r) <= 4 {
		return strings.Repeat("*", len(r))
	}
	keep := 2
	if len(r) > 12 {
		keep = 3
	}
	return string(r[:keep]) + strings.Repeat("*", len(r)-2*keep) + string(r[len(r)-keep:])
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func looksLikeIBAN(s string) bool {
	s = strings.ToUpper(strings.ReplaceAll(s, " ", ""))
	return len(s) >= 15 && len(s) <= 34 && unicode.IsLetter(rune(s[0])) && unicode.IsLetter(rune(s[1]))
}

func luhnOK(d string) bool {
	if len(d) < 13 || len(d) > 19 {
		return false
	}
	sum := 0
	alt := false
	for i := len(d) - 1; i >= 0; i-- {
		n := int(d[i] - '0')
		if n < 0 || n > 9 {
			return false
		}
		if alt {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		alt = !alt
	}
	return sum%10 == 0
}
