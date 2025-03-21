package ginrollbar

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/rollbar/rollbar-go"
)

// Recovery middleware for rollbar error monitoring
func Recovery(onlyCrashes, printStack bool, requestIdCtxKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rval := recover(); rval != nil {
				if printStack {
					debug.PrintStack()
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
				rollbar.Critical(
					errors.New(fmt.Sprint(rval)),
					c.Request,
					3,
					map[string]interface{}{"endpoint": c.Request.RequestURI},
				)

				c.AbortWithStatus(http.StatusInternalServerError)
			}

			if !onlyCrashes {
				extraData := make(map[string]interface{})
				extraData["endpoint"] = c.Request.RequestURI
				if requestIdCtxKey != "" {
					extraData["request_id"] = c.Writer.Header().Get(requestIdCtxKey)
				}
				for _, item := range c.Errors {
					extraData["meta"] = fmt.Sprint(item.Meta)
					rollbar.Error(item.Err, c.Request, extraData)
				}
			}
		}()

		c.Next()
	}
}
