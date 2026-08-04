// handler_instances_test.go — Verifies upload HTTP dependencies are instance-owned.
package httpapi

import (
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

func TestHandlersKeepStageDependenciesIsolated(t *testing.T) {
	t.Parallel()

	first := newHandlersWithStages(nil, false, testJSONResponder, stageFunctions{
		fileRead: func(upload.FileReadRequest, *uploadsec.Security, bool) upload.FileReadResponse {
			return upload.FileReadResponse{Success: true, FileName: "first"}
		},
	})
	second := newHandlersWithStages(nil, false, testJSONResponder, stageFunctions{
		fileRead: func(upload.FileReadRequest, *uploadsec.Security, bool) upload.FileReadResponse {
			return upload.FileReadResponse{Success: true, FileName: "second"}
		},
	})

	var wait sync.WaitGroup
	for _, testCase := range []struct {
		handler *Handlers
		want    string
	}{{first, "first"}, {second, "second"}} {
		wait.Add(1)
		go func(handler *Handlers, want string) {
			defer wait.Done()
			response := httptest.NewRecorder()
			request := httptest.NewRequest("POST", "/api/file/read", strings.NewReader(`{"file_path":"/tmp/a"}`))
			handler.HandleFileRead(response, request)
			if !strings.Contains(response.Body.String(), `"file_name":"`+want+`"`) {
				t.Errorf("response = %s, want isolated %s stage", response.Body.String(), want)
			}
		}(testCase.handler, testCase.want)
	}
	wait.Wait()
}
