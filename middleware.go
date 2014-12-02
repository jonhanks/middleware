// Some basic net/http compliant middleware
package middleware

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

var registry map[string]func(http.Handler) http.Handler = make(map[string]func(http.Handler) http.Handler)
var registryLock sync.RWMutex

type statusResponseWriter struct {
	wrapped http.ResponseWriter
	status  int
}

func (l *statusResponseWriter) Header() http.Header {
	return l.wrapped.Header()
}

func (l *statusResponseWriter) Write(data []byte) (int, error) {
	l.setStatus(http.StatusOK)
	return l.wrapped.Write(data)
}

func (l *statusResponseWriter) WriteHeader(statusValue int) {
	l.setStatus(statusValue)
	l.wrapped.WriteHeader(statusValue)
}

func (l *statusResponseWriter) setStatus(statusValue int) {
	if l.status == 0 {
		l.status = statusValue
	}
}

func (l *statusResponseWriter) GetStatus() int {
	if l.status == 0 {
		return http.StatusOK
	}
	return l.status
}

// Create a logging middleware.  It reports the http status code, url, and time taken to execute the request
//
// out - The io.Writer to log to
// chain - The handler that is next in the middleware/handler chain
//
// A two instances are added to the registry
//  "middleware.LoggingStdOut" a logger that logs to os.Stdout
//  "middleware.LoggingStdErr" a logger that logs to os.Stderr
func NewLoggingMiddleware(out io.Writer, chain http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statusWriter := &statusResponseWriter{wrapped: w}
		start := time.Now()
		chain.ServeHTTP(statusWriter, r)
		end := time.Now()
		code := statusWriter.GetStatus()

		out.Write([]byte(fmt.Sprintf("%d %s %v\n", code, r.URL.Path, end.Sub(start))))
		//out.Write([]byte(strconv.FormatInt(int64(code), 10) + " " + r.URL.Path + " " + end.Sub(start) + "\n"))
	}
}

// Create a handler to handle panics
// Added to the registry as "middleware.Panic"
func NewPanicMiddleware(chain http.Handler) http.Handler {
	var f http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if r := recover(); r != nil {
				w.WriteHeader(500)
			}
		}()
		chain.ServeHTTP(w, r)
	}
	return f
}

func init() {
	Register("middleware.Panic", NewPanicMiddleware)
	Register("middleware.LoggingStdOut", func(chain http.Handler) http.Handler {
		return NewLoggingMiddleware(os.Stdout, chain)
	})
	Register("middleware.LoggingStdErr", func(chain http.Handler) http.Handler {
		return NewLoggingMiddleware(os.Stderr, chain)
	})
}

// Add a middleware function to a global registry.
// Duplicate keys are not allowed (and panic)
// Nill entries are not added
func Register(key string, f func(http.Handler) http.Handler) {
	if f == nil {
		return
	}
	registryLock.Lock()
	defer registryLock.Unlock()

	if _, ok := registry[key]; ok {
		panic("Middleware registry key reused")
	}

	registry[key] = f
}

// Retreive a middleware function from the global registry
// returns handler, bool.  True if there is a matching handler, esle false
func Get(key string) (func(http.Handler) http.Handler, bool) {
	registryLock.RLock()
	defer registryLock.RUnlock()

	f, ok := registry[key]
	return f, ok
}

func MustGet(key string) func(http.Handler) http.Handler {
	registryLock.RLock()
	defer registryLock.RUnlock()

	f := registry[key]
	if f == nil {
		panic("Invalid middleware requested")
	}
	return f
}
