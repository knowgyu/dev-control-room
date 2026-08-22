// Package masking keeps secret-bearing text on the safe side of persistence
// and presentation boundaries. It is deliberately independent of collectors,
// HTTP, and the action broker so every caller can use the same implementation.
package masking

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const Replacement = "[REDACTED]"

var (
	authorizationPattern = regexp.MustCompile(`(?i)(\b(?:authorization|proxy-authorization)\s*:\s*(?:bearer|basic|token)\s+)[^\s,;]+`)
	apiKeyHeaderPattern  = regexp.MustCompile(`(?i)(\b(?:x-api-key|x-auth-token|api-key)\s*:\s*)[^\s,;]+`)
	cookieHeaderPattern  = regexp.MustCompile(`(?i)(\bcookie\s*:\s*)[^\r\n]+`)
	credentialURLPattern = regexp.MustCompile(`(?i)(\b[a-z][a-z0-9+.-]*://)[^\s/@:]+:[^\s/@]+@`)
	querySecretPattern   = regexp.MustCompile(`(?i)([?&](?:token|api[_-]?key|password|passwd|secret|credential|signature)=[^&#\s]+)`)
	variablePattern      = regexp.MustCompile(`(?i)(\$env:)?([a-z_][a-z0-9_]*)\s*([=:])\s*([^\s,;]+)`)
	githubTokenPattern   = regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9_]{20,}\b`)
	awsAccessKeyPattern  = regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)
	jwtPattern           = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b`)
)

type Masker struct {
	secrets         []string
	secretPatterns  []string
	sensitiveNames  map[string]struct{}
	maxSecretLength int
}

func New(secrets, sensitiveVariableNames []string) *Masker {
	unique := make(map[string]struct{}, len(secrets))
	clean := make([]string, 0, len(secrets))
	maxLength := 0
	for _, secret := range secrets {
		if strings.TrimSpace(secret) == "" || len(secret) < 4 {
			continue
		}
		if _, exists := unique[secret]; exists {
			continue
		}
		unique[secret] = struct{}{}
		clean = append(clean, secret)
		if len(secret) > maxLength {
			maxLength = len(secret)
		}
	}
	// Longest-first prevents a short known value from consuming a prefix of a
	// more specific value before it can be replaced.
	sort.Slice(clean, func(i, j int) bool { return len(clean[i]) > len(clean[j]) })

	names := make(map[string]struct{}, len(sensitiveVariableNames))
	for _, name := range sensitiveVariableNames {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" {
			names[name] = struct{}{}
		}
	}

	patterns := make([]string, 0, len(clean)*2)
	for _, secret := range clean {
		patterns = append(patterns, secret)
		if encoded := url.QueryEscape(secret); encoded != secret {
			patterns = append(patterns, encoded)
		}
		if encoded := url.PathEscape(secret); encoded != secret {
			patterns = append(patterns, encoded)
		}
	}
	return &Masker{secrets: clean, secretPatterns: patterns, sensitiveNames: names, maxSecretLength: maxLength}
}

func (m *Masker) Mask(text string) string {
	if m == nil || text == "" {
		return text
	}
	for _, secret := range m.secretPatterns {
		text = strings.ReplaceAll(text, secret, Replacement)
	}
	text = authorizationPattern.ReplaceAllString(text, `${1}`+Replacement)
	text = apiKeyHeaderPattern.ReplaceAllString(text, `${1}`+Replacement)
	text = cookieHeaderPattern.ReplaceAllString(text, `${1}`+Replacement)
	text = credentialURLPattern.ReplaceAllString(text, `${1}`+Replacement+`@`)
	text = querySecretPattern.ReplaceAllStringFunc(text, func(value string) string {
		index := strings.IndexByte(value, '=')
		if index < 0 {
			return value
		}
		return value[:index+1] + Replacement
	})
	text = m.maskSensitiveVariables(text)
	text = githubTokenPattern.ReplaceAllString(text, Replacement)
	text = awsAccessKeyPattern.ReplaceAllString(text, Replacement)
	text = jwtPattern.ReplaceAllString(text, Replacement)
	return text
}

func (m *Masker) maskSensitiveVariables(text string) string {
	return variablePattern.ReplaceAllStringFunc(text, func(value string) string {
		matches := variablePattern.FindStringSubmatch(value)
		if len(matches) != 5 {
			return value
		}
		name := strings.ToLower(matches[2])
		if _, ok := m.sensitiveNames[name]; !ok {
			return value
		}
		return matches[1] + matches[2] + matches[3] + Replacement
	})
}

func (m *Masker) MaskBytes(value []byte) []byte {
	return []byte(m.Mask(string(value)))
}

func (m *Masker) MaskValue(value any) any {
	switch typed := value.(type) {
	case string:
		return m.Mask(typed)
	case []byte:
		return string(m.MaskBytes(typed))
	case map[string]any:
		masked := make(map[string]any, len(typed))
		for key, item := range typed {
			masked[key] = m.MaskValue(item)
		}
		return masked
	case []any:
		masked := make([]any, len(typed))
		for index, item := range typed {
			masked[index] = m.MaskValue(item)
		}
		return masked
	default:
		return value
	}
}

// StreamMasker retains a bounded suffix between writes. This prevents a
// secret split across stdout/stderr chunks from being emitted unmasked.
type StreamMasker struct {
	masker  *Masker
	pending string
}

func (m *Masker) Stream() *StreamMasker {
	return &StreamMasker{masker: m}
}

func (s *StreamMasker) Write(chunk string) string {
	if s == nil || s.masker == nil {
		return chunk
	}
	s.pending += chunk
	masked := s.masker.Mask(s.pending)
	hold := s.masker.maxSecretLength - 1
	if hold < 64 {
		hold = 64
	}
	if len(masked) <= hold {
		s.pending = masked
		return ""
	}
	cut := len(masked) - hold
	for cut > 0 && cut < len(masked) && !utf8.RuneStart(masked[cut]) {
		cut--
	}
	output := masked[:cut]
	s.pending = masked[cut:]
	return output
}

func (s *StreamMasker) Flush() string {
	if s == nil || s.masker == nil {
		return ""
	}
	output := s.masker.Mask(s.pending)
	s.pending = ""
	return output
}
