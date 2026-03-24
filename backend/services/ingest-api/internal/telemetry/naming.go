package telemetry

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	normalizeSeparatorsRe = regexp.MustCompile(`[_\s]+`)
	collapseDashRe        = regexp.MustCompile(`-+`)
	siteInlineRe          = regexp.MustCompile(`^([a-z]{2})([0-9]{1,2})$`)
	digitsRe              = regexp.MustCompile(`[0-9]+`)
	siteCanonicalRe       = regexp.MustCompile(`^site-([a-z]{2})-([0-9]{2})$`)
)

// NormalizeOperationalIDs converts legacy identifiers into a canonical form.
func NormalizeOperationalIDs(siteID, rackID, minerID string) (string, string, string, error) {
	normalizedSite, err := NormalizeSiteID(siteID)
	if err != nil {
		return "", "", "", fmt.Errorf("normalize site_id: %w", err)
	}

	normalizedRack, err := NormalizeRackID(normalizedSite, rackID)
	if err != nil {
		return "", "", "", fmt.Errorf("normalize rack_id: %w", err)
	}

	normalizedMiner, err := NormalizeMinerID(minerID)
	if err != nil {
		return "", "", "", fmt.Errorf("normalize miner_id: %w", err)
	}

	return normalizedSite, normalizedRack, normalizedMiner, nil
}

func NormalizeSiteID(raw string) (string, error) {
	sanitized := sanitizeIdentifier(raw)
	if sanitized == "" {
		return "", fmt.Errorf("site id is empty")
	}

	sanitized = strings.TrimPrefix(sanitized, "site-")
	parts := strings.Split(sanitized, "-")

	if len(parts) == 1 {
		inline := siteInlineRe.FindStringSubmatch(parts[0])
		if len(inline) == 3 {
			siteNum, err := parseBoundedNumber(inline[2], 1, 99)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("site-%s-%02d", inline[1], siteNum), nil
		}
	}

	if len(parts) < 2 {
		return "", fmt.Errorf("expected country and site number")
	}

	country := parts[0]
	if len(country) != 2 || !isLowerAlpha(country) {
		return "", fmt.Errorf("country must be two letters")
	}

	siteNumberToken := parts[len(parts)-1]
	siteNum, err := parseBoundedNumber(siteNumberToken, 1, 99)
	if err != nil {
		return "", fmt.Errorf("invalid site number: %w", err)
	}

	return fmt.Sprintf("site-%s-%02d", country, siteNum), nil
}

func NormalizeRackID(siteID, raw string) (string, error) {
	site := siteCanonicalRe.FindStringSubmatch(siteID)
	if len(site) != 3 {
		return "", fmt.Errorf("site id %q is not canonical", siteID)
	}

	sanitized := sanitizeIdentifier(raw)
	if sanitized == "" {
		return "", fmt.Errorf("rack id is empty")
	}

	if strings.HasPrefix(sanitized, "rack-") {
		tokens := strings.Split(sanitized, "-")
		if len(tokens) == 4 && tokens[1] == site[1] && tokens[2] == site[2] {
			rackNum, err := parseBoundedNumber(tokens[3], 1, 99)
			if err == nil {
				return fmt.Sprintf("rack-%s-%s-%02d", site[1], site[2], rackNum), nil
			}
		}
		sanitized = strings.TrimPrefix(sanitized, "rack-")
	}

	rackNumToken := lastDigitsToken(strings.Split(sanitized, "-"))
	if rackNumToken == "" {
		return "", fmt.Errorf("rack number not found")
	}

	rackNum, err := parseBoundedNumber(rackNumToken, 1, 99)
	if err != nil {
		return "", fmt.Errorf("invalid rack number: %w", err)
	}

	return fmt.Sprintf("rack-%s-%s-%02d", site[1], site[2], rackNum), nil
}

func NormalizeMinerID(raw string) (string, error) {
	sanitized := sanitizeIdentifier(raw)
	if sanitized == "" {
		return "", fmt.Errorf("miner id is empty")
	}

	sanitized = strings.TrimPrefix(sanitized, "asic-")
	digits := strings.Join(digitsRe.FindAllString(sanitized, -1), "")
	if digits == "" {
		return "", fmt.Errorf("miner numeric segment not found")
	}

	minerNum, err := parseBoundedNumber(digits, 1, 999999)
	if err != nil {
		return "", fmt.Errorf("invalid miner number: %w", err)
	}

	return fmt.Sprintf("asic-%06d", minerNum), nil
}

func sanitizeIdentifier(raw string) string {
	s := strings.TrimSpace(strings.ToLower(raw))
	s = normalizeSeparatorsRe.ReplaceAllString(s, "-")
	s = collapseDashRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

func parseBoundedNumber(raw string, min, max int) (int, error) {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("must be numeric")
	}
	if value < min || value > max {
		return 0, fmt.Errorf("must be between %d and %d", min, max)
	}
	return value, nil
}

func lastDigitsToken(tokens []string) string {
	for i := len(tokens) - 1; i >= 0; i-- {
		if token := digitsRe.FindString(tokens[i]); token != "" {
			return token
		}
	}
	return ""
}

func isLowerAlpha(s string) bool {
	for _, r := range s {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}
