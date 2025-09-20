package util

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gosimple/slug"
)

func CanonicalizeURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	if u.Scheme == "http" {
		u.Scheme = "https"
	}

	u.Fragment = ""

	u.Path = strings.TrimSuffix(u.Path, "/")

	return u.String(), nil
}

func UnixToISO8601(unixTime int64) string {
	return time.Unix(unixTime, 0).UTC().Format(time.RFC3339)
}

func ParseISO8601(timeStr string) (time.Time, error) {
	return time.Parse(time.RFC3339, timeStr)
}

func ParseTags(tagsStr string) []string {
	tagsStr = strings.TrimSpace(tagsStr)

	if tagsStr == "" || tagsStr == "[]" {
		return nil
	}

	if strings.HasPrefix(tagsStr, "[") && strings.HasSuffix(tagsStr, "]") {
		inner := strings.Trim(tagsStr, "[]")
		if inner == "" {
			return nil
		}

		re := regexp.MustCompile(`"([^"]*)"`)
		matches := re.FindAllStringSubmatch(inner, -1)

		var tags []string
		for _, match := range matches {
			if len(match) > 1 && match[1] != "" {
				tags = append(tags, strings.TrimSpace(match[1]))
			}
		}
		return tags
	}

	parts := strings.Split(tagsStr, ",")
	var tags []string
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

func SlugifyTitle(title string, maxLength int) string {
	s := slug.Make(title)
	if len(s) > maxLength {
		s = s[:maxLength]
	}
	return s
}

func SafeFilename(title string, id int64, maxLength int) string {
	base := SlugifyTitle(title, maxLength-20) // Reserve space for ID suffix
	if base == "" {
		base = "article"
	}

	if len(base) < maxLength-20 {
		return base + "-" + strconv.FormatInt(id, 10)
	}

	return base[:maxLength-20] + "-" + strconv.FormatInt(id, 10)
}

func DedupeStrings(slice []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, item := range slice {
		item = strings.TrimSpace(item)
		if item != "" && !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}