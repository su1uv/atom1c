package feed

import (
	"strings"
	"time"
)

var rssZoneOffsets = map[string]string{
	"UT":   "+0000",
	"UTC":  "+0000",
	"GMT":  "+0000",
	"EST":  "-0500",
	"EDT":  "-0400",
	"CST":  "-0600",
	"CDT":  "-0500",
	"MST":  "-0700",
	"MDT":  "-0600",
	"PST":  "-0800",
	"PDT":  "-0700",
	"CET":  "+0100",
	"CEST": "+0200",
	"EET":  "+0200",
	"EEST": "+0300",
}

var rssDateLayouts = []string{
	time.RFC1123Z,
	time.RFC1123,
	time.RFC822Z,
	time.RFC822,
	"Mon, 02 Jan 2006 15:04 -0700",
	"Mon, 02 Jan 2006 15:04:05 -07:00",
	"Mon, 02 Jan 2006 15:04 -07:00",
	"Mon, 02 Jan 2006 15:04 MST",
	"02 Jan 2006 15:04:05 -0700",
	"02 Jan 2006 15:04:05 -07:00",
	"02 Jan 2006 15:04:05 MST",
	"02 Jan 2006 15:04 -0700",
	"02 Jan 2006 15:04 -07:00",
	"02 Jan 2006 15:04 MST",
}

func atomDate(raw string) SourceDate {
	return sourceDate(raw, []string{time.RFC3339Nano})
}

func rssDate(raw string) SourceDate {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return SourceDate{Raw: raw}
	}
	normalized, ok := normalizeRSSZone(trimmed)
	if !ok {
		return SourceDate{Raw: raw}
	}
	return sourceDateWithValues(raw, []string{normalized}, rssDateLayouts)
}

func sourceDate(raw string, layouts []string) SourceDate {
	return sourceDateWithValues(raw, []string{strings.TrimSpace(raw)}, layouts)
}

func sourceDateWithValues(raw string, values, layouts []string) SourceDate {
	result := SourceDate{Raw: raw}
	for _, layout := range layouts {
		for _, value := range values {
			parsed, err := time.Parse(layout, value)
			if err == nil {
				utc := parsed.UTC()
				result.Normalized = &utc
				return result
			}
		}
	}
	return result
}

func normalizeRSSZone(raw string) (string, bool) {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return raw, true
	}
	zone := fields[len(fields)-1]
	if offset, ok := rssZoneOffsets[zone]; ok {
		fields[len(fields)-1] = offset
		return strings.Join(fields, " "), true
	}
	if isAlphabetic(zone) {
		return raw, false
	}
	return raw, true
}

func isAlphabetic(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') {
			return false
		}
	}
	return true
}
