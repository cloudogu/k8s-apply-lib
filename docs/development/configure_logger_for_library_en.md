# Configure Logger for Library

The library generates console output which can be captured by configuring a valid logger.

A valid logger for the library needs to fulfill the [logger interface](../../apply/logger.go) provided by the library.

We need to implement the logger in our application. For that, we create a new struct `libraryLogger` containing a
reference to the logger used inside our application (logr, logrus, log). All the interface calls are delegated to our
internal logger:

```go
package example

type libraryLogger struct {
	logger *yourInternalLogger
}

func (l libraryLogger) Debug(args ...interface{})   { ... }
func (l libraryLogger) Info(args ...interface{})    { ... }
func (l libraryLogger) Warning(args ...interface{}) { ... }
func (l libraryLogger) Error(args ...interface{})   { ... }
func (l libraryLogger) Print(args ...interface{})   { ... }

func (l libraryLogger) Debugf(format string, args ...interface{})   { ... }
func (l libraryLogger) Infof(format string, args ...interface{})    { ... }
func (l libraryLogger) Warningf(format string, args ...interface{}) { ... }
func (l libraryLogger) Errorf(format string, args ...interface{})   { ... }
func (l libraryLogger) Printf(format string, args ...interface{})   { ... }
```

Then we can register our logger to the library:

```go
package example

func configureLibraryLogger(applicationLogger *yourInternalLogger) {
	// assign the application logger as the internal logger for our logger wrapper.
	k8sApplyLibLogger := libraryLogger{logger: applicationLogger}
	
	// The method core.GetLogger provides the logger instance to the whole library
	apply.GetLogger = func() apply.Logger {
		return &k8sApplyLibLogger
	}
}
```