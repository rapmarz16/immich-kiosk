package partials

import "strings"

func daveningIcon(label string) string {
	switch strings.ToLower(label) {
	case "shachris", "shacharit", "shacharis":
		return "🌅"
	case "mincha":
		return "☀️"
	case "maariv", "ma'ariv":
		return "🌙"
	default:
		return "🕯️"
	}
}

func isMincha(label string) bool {
	return strings.EqualFold(label, "mincha")
}

func daveningRowClass(label string) string {
	switch strings.ToLower(label) {
	case "shachris", "shacharit", "shacharis":
		return "davening-times__row--morning"
	case "mincha":
		return "davening-times__row--afternoon"
	case "maariv", "ma'ariv":
		return "davening-times__row--night"
	default:
		return "davening-times__row--davening"
	}
}
