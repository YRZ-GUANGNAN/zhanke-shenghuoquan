package user

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/pkg/response"
)

const contextUser = "domainA.user"

func (s *Service) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			response.Fail(c, http.StatusUnauthorized, "authentication required")
			c.Abort()
			return
		}
		value, err := s.Authenticate(c, strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
		if err != nil {
			status := http.StatusUnauthorized
			message := "authentication required"
			if errors.Is(err, ErrBanned) {
				status = http.StatusForbidden
				message = "user is banned"
			}
			response.Fail(c, status, message)
			c.Abort()
			return
		}
		c.Set(contextUser, value)
		c.Next()
	}
}

func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		value := CurrentUser(c)
		if value.Status != StatusActive || (value.Role != RoleAdmin && value.Role != RoleSuperAdmin) {
			response.Fail(c, http.StatusForbidden, "admin required")
			c.Abort()
			return
		}
		c.Next()
	}
}

func SuperAdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if CurrentUser(c).Role != RoleSuperAdmin {
			response.Fail(c, http.StatusForbidden, "superadmin required")
			c.Abort()
			return
		}
		c.Next()
	}
}

func UnmutedRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if CurrentUser(c).Status == StatusMuted {
			response.Fail(c, http.StatusForbidden, "user is muted")
			c.Abort()
			return
		}
		c.Next()
	}
}

func CurrentUser(c *gin.Context) User {
	value, _ := c.Get(contextUser)
	userValue, _ := value.(User)
	return userValue
}

func CurrentUserID(c *gin.Context) uint { return CurrentUser(c).ID }
