package main

import (
	"bytes"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type routerLogProbe struct {
	bytes.Buffer
	writes int
}

func (p *routerLogProbe) Write(data []byte) (int, error) {
	p.writes++
	return p.Buffer.Write(data)
}

func TestRouterLoggerSkipsOnlyExactWechatSensitivePaths(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		target     string
		marker     string
		wantLogged bool
	}{
		{
			name:       "miniapp callback",
			target:     "/api/v1/miniapp/media/wechat/callback?logprobe=miniapp-secret",
			wantLogged: false,
		},
		{
			name:       "legacy callback",
			target:     "/api/v1/media/wechat/callback?logprobe=legacy-secret",
			wantLogged: false,
		},
		{
			name:       "review proxy get",
			method:     http.MethodGet,
			target:     "/api/v1/miniapp/media/wechat/review-content?token=proxy-secret",
			wantLogged: false,
		},
		{
			name:       "review proxy head",
			method:     http.MethodHead,
			target:     "/api/v1/miniapp/media/wechat/review-content?token=proxy-secret",
			wantLogged: false,
		},
		{
			name:       "ordinary route",
			target:     "/healthz?logprobe=ordinary",
			marker:     "logprobe=ordinary",
			wantLogged: true,
		},
		{
			name:       "similar miniapp callback path",
			target:     "/api/v1/miniapp/media/wechat/callback/near?logprobe=similar",
			marker:     "logprobe=similar",
			wantLogged: true,
		},
		{
			name:       "similar legacy callback path",
			target:     "/api/v1/media/wechat/callbackish?logprobe=legacy-similar",
			marker:     "logprobe=legacy-similar",
			wantLogged: true,
		},
		{
			name:       "similar review proxy path",
			method:     http.MethodGet,
			target:     "/api/v1/miniapp/media/wechat/review-content/near?token=ordinary-marker",
			marker:     "token=ordinary-marker",
			wantLogged: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			probe := &routerLogProbe{}
			router := newRouter(probe, &routerLogProbe{})
			router.GET("/healthz", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			method := test.method
			if method == "" {
				method = http.MethodGet
			}
			request := httptest.NewRequest(method, test.target, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if test.wantLogged {
				if probe.writes == 0 {
					t.Fatal("request was not logged")
				}
				if !strings.Contains(probe.String(), test.marker) {
					t.Fatalf("log = %q, want marker %q", probe.String(), test.marker)
				}
				return
			}
			if probe.writes != 0 || probe.Len() != 0 {
				t.Fatalf("sensitive request leaked into Gin log: %q", probe.String())
			}
		})
	}
}

func TestRouterRecoverySanitizesPanicLogs(t *testing.T) {
	tests := []struct {
		name             string
		target           string
		wantAccessLogged bool
	}{
		{
			name:             "miniapp callback panic",
			target:           "/api/v1/miniapp/media/wechat/callback?signature=miniapp-query-secret",
			wantAccessLogged: false,
		},
		{
			name:             "legacy callback panic",
			target:           "/api/v1/media/wechat/callback?signature=legacy-query-secret",
			wantAccessLogged: false,
		},
		{
			name:             "review proxy panic",
			target:           "/api/v1/miniapp/media/wechat/review-content?token=proxy-secret",
			wantAccessLogged: false,
		},
		{
			name:             "ordinary route panic",
			target:           "/panic?logprobe=ordinary",
			wantAccessLogged: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			accessProbe := &routerLogProbe{}
			recoveryProbe := &routerLogProbe{}
			router := newRouter(accessProbe, recoveryProbe)
			path := strings.SplitN(test.target, "?", 2)[0]
			router.Any(path, func(*gin.Context) {
				panic("signature=panic-value-secret")
			})

			request := httptest.NewRequest(http.MethodPost, test.target, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if response.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
			}
			recoveryLog := recoveryProbe.String()
			if !strings.Contains(recoveryLog, "path="+path) {
				t.Fatalf("recovery log = %q, want sanitized path", recoveryLog)
			}
			if !strings.Contains(recoveryLog, "panic_recovered") {
				t.Fatalf("recovery log = %q, want fixed panic marker", recoveryLog)
			}
			if !strings.Contains(recoveryLog, "stack:") || !strings.Contains(recoveryLog, "goroutine ") {
				t.Fatalf("recovery log = %q, want stack", recoveryLog)
			}
			if strings.Contains(recoveryLog, "signature=") || strings.Contains(recoveryLog, "token=") || strings.Contains(recoveryLog, "proxy-secret") || strings.Contains(recoveryLog, "logprobe=") || strings.Contains(recoveryLog, "panic-value-secret") {
				t.Fatalf("recovery log leaked RawQuery or recovered value: %q", recoveryLog)
			}
			if strings.Contains(accessProbe.String(), "panic-value-secret") {
				t.Fatalf("access log leaked recovered value: %q", accessProbe.String())
			}

			if test.wantAccessLogged {
				if !strings.Contains(accessProbe.String(), "logprobe=ordinary") {
					t.Fatalf("ordinary access log = %q, want existing query logging behavior", accessProbe.String())
				}
			} else if accessProbe.Len() != 0 || accessProbe.writes != 0 {
				t.Fatalf("sensitive panic leaked into access log: %q", accessProbe.String())
			}
		})
	}
}

func TestRouterRecoveryDiscardsGinBrokenPipeRequestDump(t *testing.T) {
	accessProbe := &routerLogProbe{}
	recoveryProbe := &routerLogProbe{}
	router := newRouter(accessProbe, recoveryProbe)
	path := "/api/v1/miniapp/media/wechat/callback"
	router.POST(path, func(*gin.Context) {
		panic(&net.OpError{
			Op:  "write",
			Net: "tcp",
			Err: &os.SyscallError{Syscall: "write", Err: errors.New("broken pipe")},
		})
	})

	request := httptest.NewRequest(http.MethodPost, path+"?signature=broken-pipe-secret", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if accessProbe.Len() != 0 || accessProbe.writes != 0 {
		t.Fatalf("broken-pipe callback leaked into access log: %q", accessProbe.String())
	}
	if recoveryProbe.Len() != 0 || recoveryProbe.writes != 0 {
		t.Fatalf("Gin broken-pipe request dump was not discarded: %q", recoveryProbe.String())
	}
}
