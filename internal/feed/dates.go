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

var rssDateLayouts = func() []string {
	var layouts []string
	// Day layout "2" accepts both padded and single-digit days. Named zones
	// are normalized to known numeric offsets before trying these layouts.
	for _, weekday := range []string{"Mon, ", ""} {
		for _, year := range []string{"2006", "06"} {
			for _, clock := range []string{"15:04:05", "15:04"} {
				for _, zone := range []string{"-0700", "-07:00"} {
					layouts = append(layouts, weekday+"2 Jan "+year+" "+clock+" "+zone)
				}
			}
		}
	}
	return layouts
}()

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
