package ginrollbar

import (
	"fmt"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/rollbar/rollbar-go"
)

// allow monkey-patching
var (
	RollbarCritical = rollbar.Critical
	RollbarError    = rollbar.Error
)

// Middleware for rollbar panic and error monitoring
// onlyPanics: if true, only panics will be logged, otherwise errors will be logged
// printStack: if true, the stack trace will be printed
// requestIdCtxKey: the key of the request id in the context
func LogRequests(onlyPanics, printStack bool, requestIdCtxKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			// Log errors before handling any panic
			if !onlyPanics && len(c.Errors) > 0 {
				extraData := make(map[string]interface{})
				extraData["endpoint"] = c.Request.RequestURI
				if requestIdCtxKey != "" {
					extraData["request_id"] = c.Writer.Header().Get(requestIdCtxKey)
				}
				for _, item := range c.Errors {
					extraData["meta"] = fmt.Sprint(item.Meta)
					RollbarError(item.Err, c.Request, extraData)
				}
			}

			// If there's a panic, recover the panic, log it, and re-panic.
			if r := recover(); r != nil {
				if printStack {
					debug.PrintStack()
				}

				extraPanicData := make(map[string]interface{})
				extraPanicData["endpoint"] = c.Request.RequestURI
				if requestIdCtxKey != "" {
					extraPanicData["request_id"] = c.Writer.Header().Get(requestIdCtxKey)
				}

				// From the rollbar-go docs:
				// Critical reports an item with level `critical`. This function recognizes arguments with the following types:
				//    *http.Request
				//    error
				//    string
				//    map[string]interface{}
				//    int
				// The string and error types are mutually exclusive.
				// If an error is present then a stack trace is captured. If an int is also present then we skip
				// that number of stack frames. If the map is present it is used as extra custom data in the
				// item. If a string is present without an error, then we log a message without a stack
				// trace. If a request is present we extract as much relevant information from it as we can.
				RollbarCritical(
					errors.New(fmt.Sprint(r)),
					c.Request,
					3,
					extraPanicData,
				)
				panic(r)
			}
		}()

		c.Next()
	}
}
