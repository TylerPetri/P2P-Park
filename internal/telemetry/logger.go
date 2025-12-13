package telemetry

type Logger interface {
	Printf(format string, args ...any)
}
