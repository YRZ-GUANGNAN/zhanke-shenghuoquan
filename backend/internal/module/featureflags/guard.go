package featureflags

import (
	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

// RequestKeys only gates optional surfaces and NEW business. Account safety,
// moderation, wallet access and fulfillment of existing orders stay available.
func RequestKeys(method, path string) []string {
	path = strings.TrimPrefix(path, "/api/v1")
	switch {
	case strings.HasPrefix(path, "/community/"):
		keys := []string{KeyForum}
		if strings.Contains(path, "/search") {
			keys = append(keys, KeySearch)
		}
		if strings.Contains(path, "/notifications") {
			keys = append(keys, KeyNotifications)
		}
		if strings.Contains(path, "/following-feed") || strings.HasSuffix(path, "/follow") {
			keys = append(keys, KeyFollowing)
		}
		if strings.HasSuffix(path, "/like") {
			keys = append(keys, KeyLikes)
		}
		if strings.Contains(path, "/favorite") {
			keys = append(keys, KeyFavorites)
		}
		if strings.Contains(path, "/comments") {
			keys = append(keys, KeyComment)
		}
		if method == http.MethodPost && path == "/community/posts" {
			keys = append(keys, KeyPost)
		}
		return keys
	case strings.HasPrefix(path, "/trade/"):
		keys := []string{KeySecondhand}
		if strings.Contains(path, "/search") {
			keys = append(keys, KeySearch)
		}
		if strings.Contains(path, "/favorite") {
			keys = append(keys, KeyFavorites)
		}
		if strings.Contains(path, "/comments") {
			keys = append(keys, KeySecondhandComments)
		}
		if strings.HasSuffix(path, "/contact") {
			keys = append(keys, KeySecondhandChat)
		}
		if (method == http.MethodPost && path == "/trade/products") || method == http.MethodPut && strings.HasPrefix(path, "/trade/products/") && !strings.HasSuffix(path, "/favorite") && !strings.HasSuffix(path, "/contact") {
			keys = append(keys, KeySecondhandPublish)
		}
		return keys
	case strings.HasPrefix(path, "/im/"):
		// Product relations are checked after participant authorization by IM.
		return []string{KeyMessages}
	case strings.HasPrefix(path, "/navigation/external-takeout"):
		keys := []string{KeyTakeout}
		if strings.Contains(path, "/search") {
			keys = append(keys, KeySearch)
		}
		return keys
	case strings.HasPrefix(path, "/navigation"):
		return []string{KeyCampusTools}
	case strings.HasPrefix(path, "/auth/identity-applications") && (method == http.MethodPost || method == http.MethodPut):
		if strings.HasSuffix(path, "/rider") {
			return []string{KeyIdentityApplication, KeyRider}
		}
		return []string{KeyIdentityApplication, KeyMerchant}
	case path == "/delivery/merchant/orders" && method == http.MethodPost:
		return []string{KeyDelivery, KeyMerchant}
	case path == "/delivery/rider/orders/claimable" || strings.HasPrefix(path, "/delivery/rider/orders/") && strings.HasSuffix(path, "/claim") && method == http.MethodPost:
		return []string{KeyDelivery, KeyRider}
	case strings.HasPrefix(path, "/takeout/buyer/merchants") || path == "/takeout/buyer/orders" && method == http.MethodPost:
		return []string{KeyTakeout}
	case strings.HasPrefix(path, "/takeout/merchant/") && (strings.Contains(path, "/categories") || strings.Contains(path, "/dishes") || strings.HasSuffix(path, "/onboarding")):
		return []string{KeyTakeout, KeyMerchant}
	case path == "/takeout/rider/claimable-orders" || path == "/takeout/rider/register" || strings.HasPrefix(path, "/takeout/rider/orders/") && strings.HasSuffix(path, "/claim"):
		return []string{KeyTakeout, KeyRider}
	}
	return nil
}

func GuardRequests() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Protected anonymous requests reach their existing authentication gate.
		protected := c.Request.Method != http.MethodGet || strings.HasPrefix(c.Request.URL.Path, "/api/v1/im/") || strings.Contains(c.Request.URL.Path, "/me/") || strings.Contains(c.Request.URL.Path, "/delivery/") || strings.Contains(c.Request.URL.Path, "/takeout/")
		if protected && c.GetHeader("Authorization") == "" {
			c.Next()
			return
		}
		for _, key := range RequestKeys(c.Request.Method, c.Request.URL.Path) {
			if !Can(key) {
				response.Fail(c, http.StatusForbidden, "该功能暂未开放")
				c.Abort()
				return
			}
		}
		c.Next()
	}
}
