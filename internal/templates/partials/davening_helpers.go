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

func daveningDisplayLabel(label string) string {
	words := strings.Fields(label)
	if len(words) == 0 {
		return label
	}

	displayWords := make([]string, 0, len(words))
	for _, word := range words {
		trimmed := strings.Trim(word, "()[]{}.,;:-")
		if strings.EqualFold(trimmed, "downstairs") {
			continue
		}
		displayWords = append(displayWords, word)
	}

	if len(displayWords) == 0 {
		return label
	}
	return strings.Join(displayWords, " ")
}
