package ginrollbar

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestLogPanicsToRollbar(t *testing.T) {
	testError := &gin.Error{
		Err:  errors.New("test error"),
		Type: gin.ErrorTypePublic,
	}
	testError.SetMeta("some data") //nolint:errcheck

	type args struct {
		onlyPanics         bool
		handler            gin.HandlerFunc
		expectedErrorCalls int
		expectedPanicCalls int
		expectedHttpStatus int
	}

	tests := []struct {
		name string
		args args
	}{
		// Tests with onlyPanics set to true
		{
			name: "setting onlyPanics should not log errors when present and should log panic when present",
			args: args{
				onlyPanics: true,
				handler: func(c *gin.Context) {
					_ = c.Error(testError)
					_ = c.Error(testError)
					panic("occurs panic")
				},
				expectedErrorCalls: 0,
				expectedPanicCalls: 1,
				expectedHttpStatus: http.StatusInternalServerError,
			},
		},
		{
			name: "setting onlyPanics should log only panic when present",
			args: args{
				onlyPanics: true,
				handler: func(c *gin.Context) {
					panic("occurs panic")
				},
				expectedErrorCalls: 0,
				expectedPanicCalls: 1,
				expectedHttpStatus: http.StatusInternalServerError,
			},
		},
		{
			name: "setting onlyPanics should not log anything when only errors are present",
			args: args{
				onlyPanics: true,
				handler: func(c *gin.Context) {
					_ = c.Error(testError)
					_ = c.Error(testError)
					c.AbortWithStatus(http.StatusBadRequest)
				},
				expectedErrorCalls: 0,
				expectedPanicCalls: 0,
				expectedHttpStatus: http.StatusBadRequest,
			},
		},
		{
			name: "setting onlyPanics should not log anything when neither errors nor panics are present",
			args: args{
				onlyPanics: true,
				handler: func(c *gin.Context) {
					c.Status(http.StatusOK)
				},
				expectedErrorCalls: 0,
				expectedPanicCalls: 0,
				expectedHttpStatus: http.StatusOK,
			},
		},
		// Tests with onlyPanics set to false
		{
			name: "not setting onlyPanics should log errors when present and should log panic when present",
			args: args{
				onlyPanics: false,
				handler: func(c *gin.Context) {
					_ = c.Error(testError)
					_ = c.Error(testError)
					panic("occurs panic")
				},
				expectedErrorCalls: 2,
				expectedPanicCalls: 1,
				expectedHttpStatus: http.StatusInternalServerError,
			},
		},
		{
			name: "not setting onlyPanics should log only panics are present",
			args: args{
				onlyPanics: false,
				handler: func(c *gin.Context) {
					panic("occurs panic")
				},
				expectedErrorCalls: 0,
				expectedPanicCalls: 1,
				expectedHttpStatus: http.StatusInternalServerError,
			},
		},
		{
			name: "not setting onlyPanics should log errors when only errors are present",
			args: args{
				onlyPanics: false,
				handler: func(c *gin.Context) {
					_ = c.Error(testError)
					_ = c.Error(testError)
					c.AbortWithStatus(http.StatusBadRequest)
				},
				expectedErrorCalls: 2,
				expectedPanicCalls: 0,
				expectedHttpStatus: http.StatusBadRequest,
			},
		},
		{
			name: "not setting onlyPanics should not log anything when neither errors nor panics are present",
			args: args{
				onlyPanics: false,
				handler: func(c *gin.Context) {
					c.Status(http.StatusOK)
				},
				expectedErrorCalls: 0,
				expectedPanicCalls: 0,
				expectedHttpStatus: http.StatusOK,
			},
		},
	}

	for _, tt := range tests {
		panicCalls := 0
		RollbarCritical = func(interfaces ...interface{}) {
			panicCalls++
			if err, ok := interfaces[0].(error); ok {
				assert.Equal(t, "occurs panic", err.Error())
			} else {
				t.Error("interfaces[0] should be error")
			}
			if request, ok := interfaces[1].(*http.Request); ok {
				assert.Equal(t, "/", request.RequestURI)
			} else {
				t.Error("interfaces[1] should be *http.Request")
			}
			if level, ok := interfaces[2].(int); ok {
				assert.Equal(t, 3, level)
			} else {
				t.Error("interfaces[2] should be int")
			}
			if metaData, ok := interfaces[3].(map[string]interface{}); ok {
				fmt.Printf("%+v", metaData)
				endpoint, _ := metaData["endpoint"].(string)
				assert.Equal(t, "/", endpoint)
			} else {
				t.Error("interfaces[3] should be map[string]interface{}")
			}
		}

		errorCalls := 0
		RollbarError = func(interfaces ...interface{}) {
			errorCalls++
			if err, ok := interfaces[0].(error); ok {
				assert.Equal(t, testError.Err.Error(), err.Error())
			} else {
				t.Error("interfaces[0] should be error")
			}
			if request, ok := interfaces[1].(*http.Request); ok {
				assert.Equal(t, "/", request.RequestURI)
			} else {
				t.Error("interfaces[1] should be *http.Request")
			}
			if metaData, ok := interfaces[2].(map[string]interface{}); ok {
				endpoint, _ := metaData["endpoint"].(string)
				assert.Equal(t, "/", endpoint)
				meta, _ := metaData["meta"].(string)
				assert.Equal(t, "some data", meta)
			} else {
				t.Error("interfaces[2] should be map[string]interface{}")
			}
		}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(func(c *gin.Context) {
			defer func() {
				if err := recover(); err != nil {
					c.AbortWithStatus(http.StatusInternalServerError)
				}
			}()
			c.Next()
		})

		router.Use(LogRequests(tt.args.onlyPanics, false, ""))
		router.GET("/", tt.args.handler)

		t.Run(tt.name, func(t *testing.T) {
			w := performRequest("GET", "/", router)

			assert.Equal(t, tt.args.expectedHttpStatus, w.Code, "http status code")
			assert.Equal(t, tt.args.expectedErrorCalls, errorCalls, "Calls to RollbarError")
			assert.Equal(t, tt.args.expectedPanicCalls, panicCalls, "Calls to RollbarCritical")
		})
	}
}

func performRequest(method, target string, router *gin.Engine) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w
}
