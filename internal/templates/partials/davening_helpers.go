package partials

import "strings"

func daveningIcon(label string) string {
	lower := strings.ToLower(label)
	switch {
	case strings.Contains(lower, "shach"):
		return "🌅"
	case strings.Contains(lower, "mincha"):
		return "☀️"
	case strings.Contains(lower, "maariv") || strings.Contains(lower, "marriv"):
		return "🌙"
	default:
		return "🕯️"
	}
}

func isMincha(label string) bool {
	return strings.Contains(strings.ToLower(label), "mincha")
}

func daveningRowClass(label string) string {
	lower := strings.ToLower(label)
	switch {
	case strings.Contains(lower, "shach"):
		return "davening-times__row--morning"
	case strings.Contains(lower, "mincha"):
		return "davening-times__row--afternoon"
	case strings.Contains(lower, "maariv") || strings.Contains(lower, "marriv"):
		return "davening-times__row--night"
	default:
		return "davening-times__row--davening"
	}
}
