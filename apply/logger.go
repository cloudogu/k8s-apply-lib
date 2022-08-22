package apply

import "github.com/sirupsen/logrus"

// Logger provides a simple definition of Logging methods that may be used within this project.
// Logger implementations can be set by getting the current logger referent with GetLogger and to replace it with one's
// own implementation.
type Logger interface {
	// Debug logs messages that generally should not be visible within a production environment but useful when trying
	// to pinpoint error situations during the development.
	Debug(args ...interface{})
	// Info logs messages that may be of general interest but do not state any danger.
	Info(args ...interface{})
	// Warning logs error messages that may require the attention of the user.
	Warning(args ...interface{})
	// Error logs error messages that may jeopardize the success during the run-time.
	Error(args ...interface{})
	// Print logs are used to inform the user about the current state of the application and its tasks.
	Print(args ...interface{})

	// Debugf logs messages that generally should not be visible within a production environment but useful when trying
	// to pinpoint error situations during the development.
	Debugf(format string, args ...interface{})
	// Infof logs messages that may be of general interest but do not state any danger.
	Infof(format string, args ...interface{})
	// Warningf logs error messages that may require the attention of the user.
	Warningf(format string, args ...interface{})
	// Errorf logs error messages that may jeopardize the success during the run-time.
	Errorf(format string, args ...interface{})
	// Printf logs are used to inform the user about the current state of the application and its tasks.
	Printf(format string, args ...interface{})
}

// GetLogger is an alias function to provide a different logger for the core.
var GetLogger = func() Logger { return logrus.StandardLogger() }
