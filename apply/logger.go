package apply

// Logger provides a simple definition of Logging methods that may be used within this project.
// Logger implementations can be set by getting the current logger referent with GetLogger and to replace it with one's
// own implementation.
type Logger interface {
	// Debug logs messages that generally should not be visible within a production environment but useful when trying
	// to pinpoint error situations during the development.
	Debug(args ...interface{})
	// Info logs messages that may be of general interest but do not state any danger.
	Info(args ...interface{})
	// Error logs error messages that may jeopardize the success during the run-time.
	Error(args ...interface{})
}
