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

func isMaarivOnly(label string) bool {
	lower := strings.ToLower(label)
	return (strings.Contains(lower, "maariv") || strings.Contains(lower, "marriv")) && !strings.Contains(lower, "mincha")
}

func daveningRowClass(label string) string {
	lower := strings.ToLower(label)
	classes := []string{}
	switch {
	case strings.Contains(lower, "shach"):
		classes = append(classes, "davening-times__row--morning")
	case strings.Contains(lower, "mincha"):
		classes = append(classes, "davening-times__row--afternoon")
	case strings.Contains(lower, "maariv") || strings.Contains(lower, "marriv"):
		classes = append(classes, "davening-times__row--night")
	default:
		classes = append(classes, "davening-times__row--davening")
	}
	if len([]rune(label)) >= 22 {
		classes = append(classes, "davening-times__row--verbose-label")
	}
	return strings.Join(classes, " ")
}
