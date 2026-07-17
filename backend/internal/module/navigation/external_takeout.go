package navigation

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	errInvalidExternalTakeoutTitle    = errors.New("invalid external takeout merchant title")
	errInvalidExternalTakeoutSubtitle = errors.New("invalid external takeout merchant subtitle")
	errInvalidExternalTakeoutIconText = errors.New("invalid external takeout merchant icon text")
	errInvalidExternalTakeoutAppID    = errors.New("invalid external takeout merchant appid")
	errInvalidExternalTakeoutPath     = errors.New("invalid external takeout merchant path")
	errInvalidExternalTakeoutColor    = errors.New("invalid external takeout merchant color")
	errInvalidExternalTakeoutSort     = errors.New("invalid external takeout merchant sort")
)

var (
	externalTakeoutAppIDPattern       = regexp.MustCompile(`^wx[A-Za-z0-9]{16}$`)
	externalTakeoutColorPattern       = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	miniProgramSchemePattern          = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)
	nestedPercentEncodingPattern      = regexp.MustCompile(`(?i)%[0-9a-f]{2}`)
	externalTakeoutSensitiveQueryKeys = []string{"token", "secret", "openid", "session", "authorization"}
)

type ExternalTakeoutMerchantInput struct {
	Title, Subtitle, IconText, AppID, Path, Color string
	Sort                                          int
}

type ExternalTakeoutMerchant struct {
	ID              uint            `json:"id"`
	Title           string          `json:"title"`
	Subtitle        string          `json:"subtitle"`
	IconText        string          `json:"iconText"`
	AppID           string          `json:"appId"`
	Path            string          `json:"path"`
	Color           string          `json:"color"`
	Sort            int             `json:"sort"`
	Enabled         bool            `json:"enabled"`
	OperatingStatus OperatingStatus `json:"operatingStatus"`
}

type ExternalTakeoutDirectory struct {
	CategoryEnabled bool                      `json:"categoryEnabled"`
	Items           []ExternalTakeoutMerchant `json:"items"`
}

type PublicItem struct {
	ID              uint            `json:"id"`
	Title           string          `json:"title"`
	Subtitle        string          `json:"subtitle"`
	IconText        string          `json:"iconText"`
	AppID           string          `json:"appId"`
	Path            string          `json:"path"`
	Color           string          `json:"color"`
	Sort            int             `json:"sort"`
	OperatingStatus OperatingStatus `json:"operatingStatus"`
}

type PublicCategory struct {
	ID        uint         `json:"id"`
	Name      string       `json:"name"`
	SystemKey *string      `json:"systemKey,omitempty"`
	Sort      int          `json:"sort"`
	Items     []PublicItem `json:"items"`
}

func normalizeExternalTakeoutMerchantInput(input ExternalTakeoutMerchantInput) (ExternalTakeoutMerchantInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Subtitle = strings.TrimSpace(input.Subtitle)
	input.IconText = strings.TrimSpace(input.IconText)
	input.AppID = strings.TrimSpace(input.AppID)
	input.Color = strings.TrimSpace(input.Color)

	if !validExternalTakeoutText(input.Title, 64, true) {
		return ExternalTakeoutMerchantInput{}, errInvalidExternalTakeoutTitle
	}
	if !validExternalTakeoutText(input.Subtitle, 120, false) {
		return ExternalTakeoutMerchantInput{}, errInvalidExternalTakeoutSubtitle
	}
	if !validExternalTakeoutText(input.IconText, 12, true) {
		return ExternalTakeoutMerchantInput{}, errInvalidExternalTakeoutIconText
	}
	if !externalTakeoutAppIDPattern.MatchString(input.AppID) {
		return ExternalTakeoutMerchantInput{}, errInvalidExternalTakeoutAppID
	}

	normalizedPath, err := normalizeMiniProgramPath(input.Path)
	if err != nil {
		return ExternalTakeoutMerchantInput{}, errInvalidExternalTakeoutPath
	}
	input.Path = normalizedPath

	if !externalTakeoutColorPattern.MatchString(input.Color) {
		return ExternalTakeoutMerchantInput{}, errInvalidExternalTakeoutColor
	}
	if input.Sort < 0 || input.Sort > 10000 {
		return ExternalTakeoutMerchantInput{}, errInvalidExternalTakeoutSort
	}

	return input, nil
}

func normalizeMiniProgramPath(raw string) (string, error) {
	// Validate before trimming so leading or trailing control characters cannot be
	// silently removed and turned into a different accepted target.
	if !validExternalTakeoutDecodedString(raw) {
		return "", errInvalidExternalTakeoutPath
	}

	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errInvalidExternalTakeoutPath
	}

	rawPath, rawQuery, hasQuery := strings.Cut(trimmed, "?")
	decodedPath, err := url.PathUnescape(rawPath)
	if err != nil || !validExternalTakeoutDecodedString(decodedPath) {
		return "", errInvalidExternalTakeoutPath
	}
	if strings.HasPrefix(decodedPath, "//") {
		return "", errInvalidExternalTakeoutPath
	}
	if strings.HasPrefix(decodedPath, "/") {
		decodedPath = strings.TrimPrefix(decodedPath, "/")
	}
	if decodedPath == "" || decodedPath != strings.TrimSpace(decodedPath) || unsafeMiniProgramPathComponent(decodedPath) {
		return "", errInvalidExternalTakeoutPath
	}

	normalized := decodedPath
	if hasQuery {
		query, parseErr := url.ParseQuery(rawQuery)
		if parseErr != nil {
			return "", errInvalidExternalTakeoutPath
		}
		for key, values := range query {
			if unsafeMiniProgramQueryComponent(key) || sensitiveExternalTakeoutQueryKey(key) {
				return "", errInvalidExternalTakeoutPath
			}
			for _, value := range values {
				if unsafeMiniProgramQueryComponent(value) {
					return "", errInvalidExternalTakeoutPath
				}
			}
		}
		if encodedQuery := query.Encode(); encodedQuery != "" {
			normalized += "?" + encodedQuery
		}
	}

	if utf8.RuneCountInString(normalized) > 240 {
		return "", errInvalidExternalTakeoutPath
	}
	return normalized, nil
}

func validOperatingStatus(value OperatingStatus) bool {
	return value == OperatingStatusOpen || value == OperatingStatusResting
}

func validExternalTakeoutText(value string, maxRunes int, required bool) bool {
	if required && value == "" {
		return false
	}
	return validExternalTakeoutDecodedString(value) && utf8.RuneCountInString(value) <= maxRunes
}

func validExternalTakeoutDecodedString(value string) bool {
	if !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r) {
			return false
		}
	}
	return true
}

func unsafeMiniProgramPathComponent(value string) bool {
	return miniProgramSchemePattern.MatchString(value) ||
		strings.Contains(value, "//") ||
		strings.Contains(value, `\`) ||
		strings.Contains(value, "..") ||
		strings.Contains(value, "?") ||
		strings.Contains(value, "#") ||
		strings.Contains(value, "%") ||
		nestedPercentEncodingPattern.MatchString(value)
}

func unsafeMiniProgramQueryComponent(value string) bool {
	return !validExternalTakeoutDecodedString(value) ||
		strings.Contains(value, "//") ||
		strings.Contains(value, `\`) ||
		strings.Contains(value, "..") ||
		strings.Contains(value, "#") ||
		nestedPercentEncodingPattern.MatchString(value)
}

func sensitiveExternalTakeoutQueryKey(value string) bool {
	for _, sensitive := range externalTakeoutSensitiveQueryKeys {
		if strings.EqualFold(value, sensitive) {
			return true
		}
	}
	return false
}
