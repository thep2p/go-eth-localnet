package internal

type PortAssigner interface {
	// NewPort returns a new randomly assigned port that is not currently in use.
	// Error is irrecoverable if no port can be assigned.
	NewPort() int
}
