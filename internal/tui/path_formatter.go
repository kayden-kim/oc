package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

func isPathLike(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "/") || strings.Contains(trimmed, "\\") {
		return true
	}
	if strings.HasPrefix(trimmed, "~") {
		return true
	}
	return len(trimmed) >= 3 && trimmed[1] == ':' && ((trimmed[2] == '\\') || (trimmed[2] == '/'))
}

func truncatePathAware(value string, width int) string {
	if isPathLike(value) {
		return shortenPathMiddle(value, width)
	}
	return truncateDisplayWidth(value, width)
}

func shortenPathMiddle(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}

	sep := "/"
	if strings.Contains(value, "\\") {
		sep = "\\"
	}

	root := ""
	remainder := value
	if len(remainder) >= 3 && remainder[1] == ':' && (remainder[2] == '\\' || remainder[2] == '/') {
		root = remainder[:3]
		remainder = remainder[3:]
	} else if strings.HasPrefix(remainder, "~/") || strings.HasPrefix(remainder, "~\\") {
		root = remainder[:2]
		remainder = remainder[2:]
	} else if strings.HasPrefix(remainder, sep) {
		root = sep
		remainder = strings.TrimPrefix(remainder, sep)
	}

	parts := strings.FieldsFunc(remainder, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	if len(parts) == 0 {
		return truncateDisplayWidth(value, width)
	}
	if len(parts) == 1 {
		return truncateDisplayWidth(value, width)
	}

	left := parts[0]
	right := parts[len(parts)-1]
	middle := sep + ".." + sep

	build := func(leftPart string, rightPart string) string {
		leftPrefix := root
		if leftPart != "" {
			leftPrefix += leftPart
		}
		if leftPrefix == "" {
			return ".." + sep + rightPart
		}
		return leftPrefix + middle + rightPart
	}

	candidate := build(left, right)
	if lipgloss.Width(candidate) > width {
		availableRightWidth := width - lipgloss.Width(root) - lipgloss.Width(left) - lipgloss.Width(middle)
		if availableRightWidth > 0 {
			right = keepRightSegmentWidth(right, availableRightWidth)
			candidate = build(left, right)
		}
	}
	for lipgloss.Width(candidate) > width && (lipgloss.Width(left) > 1 || lipgloss.Width(right) > 1) {
		if lipgloss.Width(right) > 1 {
			right = trimLeadingRune(right)
		} else if lipgloss.Width(left) > 1 {
			left = trimTrailingRune(left)
		}
		candidate = build(left, right)
	}

	if lipgloss.Width(candidate) <= width {
		return candidate
	}

	tail := ".." + sep + parts[len(parts)-1]
	if root != "" {
		tail = root + tail
	}
	if lipgloss.Width(tail) <= width {
		return tail
	}

	return keepRightDisplayWidth(tail, width)
}

func trimLeadingRune(value string) string {
	runes := []rune(value)
	if len(runes) <= 1 {
		return value
	}
	return string(runes[1:])
}

func trimTrailingRune(value string) string {
	runes := []rune(value)
	if len(runes) <= 1 {
		return value
	}
	return string(runes[:len(runes)-1])
}

func keepRightDisplayWidth(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	if width <= 2 {
		return truncateDisplayWidth(value, width)
	}
	kept := keepRightSegmentWidth(value, width-2)
	if len(kept) == 0 {
		return ".."
	}
	return ".." + kept
}

func keepRightSegmentWidth(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	runes := []rune(value)
	var kept []rune
	currentWidth := 0
	for i := len(runes) - 1; i >= 0; i-- {
		r := runes[i]
		runeWidth := lipgloss.Width(string(r))
		if currentWidth+runeWidth > width {
			break
		}
		kept = append([]rune{r}, kept...)
		currentWidth += runeWidth
	}
	return string(kept)
}
