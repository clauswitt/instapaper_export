package util

import (
	"fmt"
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

// ParseRelativeDate parses relative date expressions like "1d", "1w", "today", "yesterday"
func ParseRelativeDate(dateStr string) (time.Time, error) {
	if dateStr == "" {
		return time.Time{}, fmt.Errorf("empty date string")
	}

	now := time.Now().UTC()
	dateStr = strings.ToLower(strings.TrimSpace(dateStr))

	// Handle specific keywords
	switch dateStr {
	case "today":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), nil
	case "yesterday":
		yesterday := now.AddDate(0, 0, -1)
		return time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, time.UTC), nil
	}

	// Handle relative time expressions (1d, 2w, 3m, etc.)
	re := regexp.MustCompile(`^(\d+)([dwmyh])$`)
	matches := re.FindStringSubmatch(dateStr)
	if len(matches) == 3 {
		amount, err := strconv.Atoi(matches[1])
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid number: %s", matches[1])
		}

		unit := matches[2]
		var targetTime time.Time

		switch unit {
		case "h":
			targetTime = now.Add(-time.Duration(amount) * time.Hour)
		case "d":
			targetTime = now.AddDate(0, 0, -amount)
		case "w":
			targetTime = now.AddDate(0, 0, -amount*7)
		case "m":
			targetTime = now.AddDate(0, -amount, 0)
		case "y":
			targetTime = now.AddDate(-amount, 0, 0)
		default:
			return time.Time{}, fmt.Errorf("invalid time unit: %s", unit)
		}

		// For day, week, month, year - set to beginning of that day
		if unit != "h" {
			targetTime = time.Date(targetTime.Year(), targetTime.Month(), targetTime.Day(), 0, 0, 0, 0, time.UTC)
		}

		return targetTime, nil
	}

	// Try to parse as ISO date (YYYY-MM-DD)
	if t, err := time.Parse("2006-01-02", dateStr); err == nil {
		return t.UTC(), nil
	}

	// Try to parse as ISO datetime
	if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
		return t.UTC(), nil
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

// FormatDateRange formats a date range for SQL queries
func FormatDateRange(since, until string) (sinceTime, untilTime *time.Time, err error) {
	if since != "" {
		t, err := ParseRelativeDate(since)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid since date: %w", err)
		}
		sinceTime = &t
	}

	if until != "" {
		t, err := ParseRelativeDate(until)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid until date: %w", err)
		}
		// For until dates, set to end of day
		endOfDay := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, time.UTC)
		untilTime = &endOfDay
	}

	return sinceTime, untilTime, nil
}