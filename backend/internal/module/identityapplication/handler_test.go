package identityapplication

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/response"
)

const handlerTestUserID = uint64(9007199254740993)

type userApplicationHTTPServiceStub struct {
	listFn   func(context.Context, uint64) ([]ApplicationView, error)
	getFn    func(context.Context, uint64, Role) (ApplicationView, error)
	submitFn func(context.Context, uint64, Role, SubmitInput) (ApplicationView, error)
}

func (s *userApplicationHTTPServiceStub) ListMine(ctx context.Context, userID uint64) ([]ApplicationView, error) {
	if s.listFn == nil {
		return nil, nil
	}
	return s.listFn(ctx, userID)
}

func (s *userApplicationHTTPServiceStub) GetMine(ctx context.Context, userID uint64, role Role) (ApplicationView, error) {
	if s.getFn == nil {
		return ApplicationView{}, ErrApplicationNotFound
	}
	return s.getFn(ctx, userID, role)
}

func (s *userApplicationHTTPServiceStub) Submit(ctx context.Context, userID uint64, role Role, input SubmitInput) (ApplicationView, error) {
	if s.submitFn == nil {
		return ApplicationView{}, errors.New("unexpected submit")
	}
	return s.submitFn(ctx, userID, role, input)
}

func TestUserIdentityApplicationHTTPRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := user.NewService(nil, "identity-handler-test-secret-that-is-longer-than-thirty-two-characters", "", "", false)
	router := gin.New()
	routes := router.Group("/api/v1/auth", auth.AuthRequired(), user.UnmutedRequired())
	NewHandler(&userApplicationHTTPServiceStub{}).RegisterUser(routes)

	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/auth/identity-applications"},
		{http.MethodGet, "/api/v1/auth/identity-applications/rider"},
		{http.MethodPut, "/api/v1/auth/identity-applications/rider"},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			request := httptest.NewRequest(route.method, route.path, nil)
			result := httptest.NewRecorder()
			router.ServeHTTP(result, request)
			if result.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d body=%s", result.Code, result.Body.String())
			}
		})
	}
}

func TestUserIdentityApplicationHTTPSuccessAndUserIDString(t *testing.T) {
	var submitted SubmitInput
	service := &userApplicationHTTPServiceStub{
		listFn: func(_ context.Context, userID uint64) ([]ApplicationView, error) {
			if userID != handlerTestUserID {
				t.Fatalf("list userID=%d", userID)
			}
			return []ApplicationView{{ApplicationPublicID: proofAppID1, UserID: "9007199254740993", Role: RoleRider, Status: StatusPending}}, nil
		},
		getFn: func(_ context.Context, userID uint64, role Role) (ApplicationView, error) {
			if userID != handlerTestUserID || role != RoleRider {
				t.Fatalf("get userID=%d role=%s", userID, role)
			}
			return ApplicationView{ApplicationPublicID: proofAppID1, UserID: "9007199254740993", Role: role, Status: StatusPending}, nil
		},
		submitFn: func(_ context.Context, userID uint64, role Role, input SubmitInput) (ApplicationView, error) {
			if userID != handlerTestUserID || role != RoleRider {
				t.Fatalf("submit userID=%d role=%s", userID, role)
			}
			submitted = input
			return ApplicationView{ApplicationPublicID: proofAppID1, UserID: "9007199254740993", Role: role, Status: StatusPending, AssetPublicIDs: input.AssetPublicIDs}, nil
		},
	}
	router := newAuthorizedUserApplicationRouter(service)

	put := performIdentityApplicationRequest(router, http.MethodPut, "/api/v1/auth/identity-applications/rider", `{"realName":"张同学","phone":"13800138000","merchantName":"","merchantAddress":"","assetIds":["`+proofAssetID1+`"],"expectedRowVersion":0}`)
	if put.Code != http.StatusOK || submitted.RealName != "张同学" || len(submitted.AssetPublicIDs) != 1 || submitted.AssetPublicIDs[0] != proofAssetID1 {
		t.Fatalf("put status=%d submitted=%+v body=%s", put.Code, submitted, put.Body.String())
	}
	assertIdentityApplicationResponseUserID(t, put, "9007199254740993")

	list := performIdentityApplicationRequest(router, http.MethodGet, "/api/v1/auth/identity-applications", "")
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"userId":"9007199254740993"`) {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	get := performIdentityApplicationRequest(router, http.MethodGet, "/api/v1/auth/identity-applications/rider", "")
	if get.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", get.Code, get.Body.String())
	}
	assertIdentityApplicationResponseUserID(t, get, "9007199254740993")
}

func TestUserIdentityApplicationHTTPFirstSubmitThenPendingConflict(t *testing.T) {
	service := NewService(newMemoryRepository(), &submissionMediaStub{}, fixedClock)
	router := newAuthorizedUserApplicationRouter(service)
	body := `{"realName":"张同学","phone":"13800138000","merchantName":"","merchantAddress":"","assetIds":["` + proofAssetID1 + `"],"expectedRowVersion":0}`

	first := performIdentityApplicationRequest(router, http.MethodPut, "/api/v1/auth/identity-applications/rider", body)
	if first.Code != http.StatusOK || !strings.Contains(first.Body.String(), `"status":"pending"`) {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}
	second := performIdentityApplicationRequest(router, http.MethodPut, "/api/v1/auth/identity-applications/rider", body)
	if second.Code != http.StatusConflict {
		t.Fatalf("second status=%d body=%s", second.Code, second.Body.String())
	}
}

func TestUserIdentityApplicationHTTPRejectsInvalidBodies(t *testing.T) {
	service := &userApplicationHTTPServiceStub{submitFn: func(context.Context, uint64, Role, SubmitInput) (ApplicationView, error) {
		t.Fatal("invalid body reached service")
		return ApplicationView{}, nil
	}}
	router := newAuthorizedUserApplicationRouter(service)
	valid := `{"realName":"张同学","phone":"13800138000","merchantName":"","merchantAddress":"","assetIds":["` + proofAssetID1 + `"],"expectedRowVersion":0}`
	for _, test := range []struct {
		name string
		body string
	}{
		{name: "unknown field", body: strings.TrimSuffix(valid, "}") + `,"privateSignedURL":"https://secret.example/?signature=leak"}`},
		{name: "case variant field", body: strings.Replace(valid, `"realName"`, `"RealName"`, 1)},
		{name: "null", body: `null`},
		{name: "array", body: `[]`},
		{name: "multiple", body: valid + valid},
		{name: "oversized", body: `{"realName":"` + strings.Repeat("x", 33<<10) + `"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := performIdentityApplicationRequest(router, http.MethodPut, "/api/v1/auth/identity-applications/rider", test.body)
			if result.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", result.Code, result.Body.String())
			}
			if strings.Contains(result.Body.String(), "signature=leak") {
				t.Fatalf("private media data leaked: %s", result.Body.String())
			}
		})
	}
}

func TestUserIdentityApplicationHTTPErrorMappingAndSanitization(t *testing.T) {
	for _, test := range []struct {
		name       string
		path       string
		serviceErr error
		wantStatus int
	}{
		{name: "invalid role", path: "/api/v1/auth/identity-applications/student", serviceErr: ErrInvalidRole, wantStatus: http.StatusBadRequest},
		{name: "pending", path: "/api/v1/auth/identity-applications/rider", serviceErr: ErrApplicationPending, wantStatus: http.StatusConflict},
		{name: "approved", path: "/api/v1/auth/identity-applications/rider", serviceErr: ErrApplicationApproved, wantStatus: http.StatusConflict},
		{name: "version", path: "/api/v1/auth/identity-applications/rider", serviceErr: ErrVersionConflict, wantStatus: http.StatusConflict},
		{name: "other user media", path: "/api/v1/auth/identity-applications/rider", serviceErr: ErrProofMediaConflict, wantStatus: http.StatusConflict},
		{name: "unknown", path: "/api/v1/auth/identity-applications/rider", serviceErr: errors.New("sql failed https://private.example/proof?signature=do-not-leak"), wantStatus: http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &userApplicationHTTPServiceStub{submitFn: func(context.Context, uint64, Role, SubmitInput) (ApplicationView, error) {
				return ApplicationView{}, test.serviceErr
			}}
			router := newAuthorizedUserApplicationRouter(service)
			result := performIdentityApplicationRequest(router, http.MethodPut, test.path, `{"realName":"张同学","phone":"13800138000","merchantName":"","merchantAddress":"","assetIds":["`+proofAssetID1+`"],"expectedRowVersion":0}`)
			if result.Code != test.wantStatus {
				t.Fatalf("status=%d body=%s", result.Code, result.Body.String())
			}
			if strings.Contains(result.Body.String(), "signature=") || strings.Contains(result.Body.String(), "sql failed") {
				t.Fatalf("private error leaked: %s", result.Body.String())
			}
		})
	}
}

func TestUserIdentityApplicationHTTPGetNotFoundAndEmptyList(t *testing.T) {
	service := &userApplicationHTTPServiceStub{}
	router := newAuthorizedUserApplicationRouter(service)

	list := performIdentityApplicationRequest(router, http.MethodGet, "/api/v1/auth/identity-applications", "")
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"data":[]`) {
		t.Fatalf("empty list status=%d body=%s", list.Code, list.Body.String())
	}
	get := performIdentityApplicationRequest(router, http.MethodGet, "/api/v1/auth/identity-applications/merchant", "")
	if get.Code != http.StatusNotFound {
		t.Fatalf("not found status=%d body=%s", get.Code, get.Body.String())
	}
}

func newAuthorizedUserApplicationRouter(service userApplicationService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	routes := router.Group("/api/v1/auth", func(c *gin.Context) { c.Next() })
	NewHandler(service, func(*gin.Context) uint64 { return handlerTestUserID }).RegisterUser(routes)
	return router
}

func performIdentityApplicationRequest(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	result := httptest.NewRecorder()
	router.ServeHTTP(result, request)
	return result
}

func assertIdentityApplicationResponseUserID(t *testing.T, result *httptest.ResponseRecorder, want string) {
	t.Helper()
	var envelope struct {
		response.Body
		Data struct {
			UserID string `json:"userId"`
		} `json:"data"`
	}
	if err := json.Unmarshal(result.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v body=%s", err, result.Body.String())
	}
	if envelope.Data.UserID != want {
		t.Fatalf("userId=%q want=%q body=%s", envelope.Data.UserID, want, result.Body.String())
	}
}
