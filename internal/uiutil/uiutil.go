package uiutil

const (
	AnsiReset = "\033[0m"
	AnsiDim   = "\033[2m"
)

var nameColors = []string{
	"\033[31m", // red
	"\033[32m", // green
	"\033[33m", // yellow
	"\033[34m", // blue
	"\033[35m", // magenta
	"\033[36m", // cyan
}

func ShortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func PickColor(s string) string {
	if s == "" {
		return AnsiReset
	}
	var h uint32
	for i := 0; i < len(s); i++ {
		h = h*16777619 ^ uint32(s[i])
	}
	return nameColors[h%uint32(len(nameColors))]
}

func FormatName(name, fallback string) string {
	display := name
	if display == "" {
		display = ShortID(fallback)
	}
	color := PickColor(display)
	return color + display + AnsiReset
}
