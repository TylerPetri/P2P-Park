package main

const (
	ansiReset = "\033[0m"
	ansiDim   = "\033[2m"
)

// Some basic, deterministic colors for names.
var nameColors = []string{
	"\033[31m", // red
	"\033[32m", // green
	"\033[33m", // yellow
	"\033[34m", // blue
	"\033[35m", // magenta
	"\033[36m", // cyan
}

// pickColor returns a color based on a stable hash of the string.
func pickColor(s string) string {
	if s == "" {
		return ansiReset
	}
	var h uint32
	for i := 0; i < len(s); i++ {
		h = h*16777619 ^ uint32(s[i]) // FNV-ish
	}
	return nameColors[h%uint32(len(nameColors))]
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func formatName(name, fallback string) string {
	display := name
	if display == "" {
		display = shortID(fallback)
	}
	color := pickColor(display)
	return color + display + ansiReset
}
