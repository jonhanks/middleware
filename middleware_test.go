package middleware

import (
	"bytes"
	. "github.com/smartystreets/goconvey/convey"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoggingMiddleware(t *testing.T) {
	called := false
	var okHandler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}

	Convey("The logging middleware should log the request path", t, func() {
		buf := bytes.NewBuffer(make([]byte, 0, 100))
		m := NewLoggingMiddleware(buf, okHandler)

		record := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/about/", nil)
		if err != nil {
			t.Fatalf("Unable to create test request")
		}
		m(record, req)
		So(record.Code, ShouldEqual, http.StatusOK)
		So(strings.Contains(buf.String(), "/about/"), ShouldBeTrue)
		Convey("The middleware should also have called the next function in the chain", func() {
			So(called, ShouldBeTrue)
		})
		Convey("The middleware should have caught the response code as well", func() {
			So(strings.Contains(buf.String(), "200"), ShouldBeTrue)
		})
		Convey("The last character in the log entry should be a newline", func() {
			tmp := buf.Bytes()
			So(tmp[len(tmp)-1], ShouldEqual, '\n')
		})
	})
}

func TestLoggingResponseWriter(t *testing.T) {
	Convey("The statusResponseWriter caches the http status value on a request", t, func() {
		record := httptest.NewRecorder()
		statusWriter := &statusResponseWriter{wrapped: record}

		Convey("The default status returned is a 200 (http.StatusOK)", func() {
			So(statusWriter.GetStatus(), ShouldEqual, http.StatusOK)
		})

		Convey("If you call WriteHeader with a value you should be able to read it back", func() {
			statusWriter.WriteHeader(http.StatusForbidden)
			So(statusWriter.GetStatus(), ShouldEqual, http.StatusForbidden)

			Convey("Calling WriteHeader a second time does not make sense, so you should only read back the original value", func() {
				statusWriter.WriteHeader(http.StatusConflict)
				So(statusWriter.GetStatus(), ShouldEqual, http.StatusForbidden)
			})
		})

		record = httptest.NewRecorder()
		record.Body = bytes.NewBuffer(make([]byte, 0, 100))
		statusWriter = &statusResponseWriter{wrapped: record}
		Convey("Calling Write on a StatusWriter should set the status value to StatusOK (the default anyways)", func() {
			statusWriter.Write([]byte("Hello World!"))

			So(statusWriter.GetStatus(), ShouldEqual, http.StatusOK)
			Convey("The value written should have been written through to the wrapped ResponseWriter", func() {
				So(record.Body.String(), ShouldEqual, "Hello World!")
			})
		})
		Convey("You should be able to call Headers() on the statusResponseWriter", func() {
			header := statusWriter.Header()
			So(header, ShouldNotBeNil)
		})
	})
}

func TestPanicMiddleware(t *testing.T) {
	called := false
	var doPanic http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		called = true
		panic("bye bye")
	}
	Convey("The panic middleware should stop panics in the handler from escaping", t, func() {
		Convey("The test handler should panic", func() {
			So(func() { doPanic(nil, nil) }, ShouldPanic)
		})
		Convey("NewPanicMiddleware should return a http.Handler", func() {
			m := NewPanicMiddleware(doPanic)
			So(m, ShouldNotBeNil)

			Convey("Calling the returned handler should not panic, and should call the wrapped handler", func() {
				called = false

				record := httptest.NewRecorder()
				So(func() { m.ServeHTTP(record, nil) }, ShouldNotPanic)
				So(called, ShouldBeTrue)
				Convey("The status code should be set to 500", func() {
					So(record.Code, ShouldEqual, 500)
				})
			})
		})
	})
}

func TestInitalRegistryContents(t *testing.T) {
	var noopHandler http.HandlerFunc = func(http.ResponseWriter, *http.Request) {

	}
	Convey("The registry should have some initial contents", t, func() {
		tests := []string{"middleware.Panic", "middleware.LoggingStdOut", "middleware.LoggingStdErr"}

		for _, test := range tests {
			f, ok := Get(test)
			So(f, ShouldNotBeNil)
			So(ok, ShouldBeTrue)
			Convey(""+test+" should chain into a http.Handler", func() {
				fullChain := f(noopHandler)
				So(fullChain, ShouldNotBeNil)
			})
		}
	})
}

func TestRegisterGetMustGet(t *testing.T) {
	f := func(next http.Handler) http.Handler {
		return next
	}

	o := func(next http.Handler) http.Handler {
		return next
	}

	registryLock.Lock()
	oldReg := registry
	registry = make(map[string]func(http.Handler) http.Handler)
	registryLock.Unlock()

	defer func() {
		registryLock.Lock()
		registry = oldReg
		registryLock.Unlock()
	}()

	Convey("The register function allows you to register middleware components", t, func() {
		Convey("Middleware is registered by name", func() {
			registryLock.Lock()
			registry = make(map[string]func(http.Handler) http.Handler)
			registryLock.Unlock()

			Register("nillHandler", f)
			Convey("After a function is registered we should be able to retreive it", func() {
				f1, ok := Get("nillHandler")
				So(f1, ShouldNotBeNil)
				So(ok, ShouldBeTrue)
				So(f1, ShouldEqual, f)
			})
			Convey("Passing nil as a handler is a nop", func() {
				Register("noop", nil)
				f2, ok := Get("noop")
				So(f2, ShouldBeNil)
				So(ok, ShouldBeFalse)
			})
			Convey("Registering a name twice panics", func() {
				So(func() { Register("nillHandler", o) }, ShouldPanic)
			})
			Convey("Registering a name twice panics, even if it is the same value", func() {
				So(func() { Register("nillHandler", f) }, ShouldPanic)
			})
			Convey("It is safe to call Get on a invalid string", func() {
				f3, ok := Get("")
				So(ok, ShouldBeFalse)
				So(f3, ShouldBeNil)
			})
			Convey("MustGet also retreives middleware from the registry, but it panics if a match is not found", func() {
				Convey("Getting a valid middleware should work", func() {
					var f4 func(http.Handler) http.Handler
					So(func() { f4 = MustGet("nillHandler") }, ShouldNotPanic)
					So(f4, ShouldNotBeNil)
				})
				Convey("Calling MustGet a non-existant key should panic", func() {
					var f5 func(http.Handler) http.Handler
					So(func() { f5 = MustGet("non-existant") }, ShouldPanic)
					So(f5, ShouldBeNil)
				})
			})
		})
	})
}
