package admin

import (
	"context"
	"strconv"
	"strings"
)

// GlobalSearch 在仓储层实现：跨订单、用户、帖子、商品与骑手模糊搜索，每类最多 5 条。
func (r *GormRepository) GlobalSearch(ctx context.Context, keyword string) ([]SearchHit, error) {
	keyword = strings.TrimSpace(keyword)
	if len([]rune(keyword)) < 2 {
		return []SearchHit{}, nil
	}
	if len([]rune(keyword)) > 50 {
		keyword = string([]rune(keyword)[:50])
	}
	pattern := "%" + escapeSearchLike(keyword) + "%"
	hits := make([]SearchHit, 0, 25)

	// 订单
	var orders []struct {
		PublicID       string
		RecipientName  string
		RecipientPhone string
		Status         string
		CreatedAt      string
	}
	if err := r.db.WithContext(ctx).Raw(
		`SELECT delivery_order_public_id AS public_id, recipient_name, recipient_phone, status, DATE_FORMAT(created_at, '%m-%d %H:%i') AS created_at
		 FROM delivery_orders
		 WHERE delivery_order_public_id LIKE ? OR recipient_name LIKE ? OR recipient_phone LIKE ? OR pickup_address LIKE ? OR delivery_address LIKE ?
		 ORDER BY created_at DESC LIMIT 5`,
		pattern, pattern, pattern, pattern, pattern).Scan(&orders).Error; err != nil {
		return nil, err
	}
	for _, row := range orders {
		hits = append(hits, SearchHit{
			Type: "order", ID: row.PublicID, Title: row.PublicID,
			Subtitle: row.RecipientName + " · " + row.Status + " · " + row.CreatedAt, Section: "delivery",
		})
	}

	// 用户（手机号仅用于匹配，不回显）
	var users []struct {
		ID       uint64
		Nickname string
		Role     string
		Status   string
	}
	if err := r.db.WithContext(ctx).Raw(
		`SELECT id, nickname, role, status FROM users
		 WHERE deleted_at IS NULL AND (nickname LIKE ? OR phone LIKE ? OR CAST(id AS CHAR) = ?)
		 ORDER BY id DESC LIMIT 5`,
		pattern, pattern, keyword).Scan(&users).Error; err != nil {
		return nil, err
	}
	for _, row := range users {
		nickname := strings.TrimSpace(row.Nickname)
		if nickname == "" {
			nickname = "用户"
		}
		hits = append(hits, SearchHit{
			Type: "user", ID: strconv.FormatUint(row.ID, 10), Title: nickname,
			Subtitle: row.Role + " · " + row.Status + " · #" + strconv.FormatUint(row.ID, 10), Section: "users",
		})
	}

	// 帖子
	var posts []struct {
		ID    uint64
		Title string
	}
	if err := r.db.WithContext(ctx).Raw(
		`SELECT id, title FROM posts WHERE deleted_at IS NULL AND title LIKE ? ORDER BY created_at DESC LIMIT 5`,
		pattern).Scan(&posts).Error; err != nil {
		return nil, err
	}
	for _, row := range posts {
		hits = append(hits, SearchHit{
			Type: "post", ID: strconv.FormatUint(row.ID, 10), Title: row.Title,
			Subtitle: "帖子 #" + strconv.FormatUint(row.ID, 10), Section: "community",
		})
	}

	// 商品
	var products []struct {
		ID    uint64
		Title string
	}
	if err := r.db.WithContext(ctx).Raw(
		`SELECT id, title FROM products WHERE deleted_at IS NULL AND title LIKE ? ORDER BY created_at DESC LIMIT 5`,
		pattern).Scan(&products).Error; err != nil {
		return nil, err
	}
	for _, row := range products {
		hits = append(hits, SearchHit{
			Type: "product", ID: strconv.FormatUint(row.ID, 10), Title: row.Title,
			Subtitle: "商品 #" + strconv.FormatUint(row.ID, 10), Section: "trade",
		})
	}

	// 骑手
	var riders []struct {
		RiderUserID uint64
		RealName    string
		Status      string
	}
	if err := r.db.WithContext(ctx).Raw(
		`SELECT rider_user_id, real_name, status FROM takeout_riders
		 WHERE real_name LIKE ? OR phone LIKE ? ORDER BY rider_user_id DESC LIMIT 5`,
		pattern, pattern).Scan(&riders).Error; err != nil {
		return nil, err
	}
	for _, row := range riders {
		name := strings.TrimSpace(row.RealName)
		if name == "" {
			name = "骑手"
		}
		hits = append(hits, SearchHit{
			Type: "rider", ID: strconv.FormatUint(row.RiderUserID, 10), Title: name,
			Subtitle: row.Status + " · 骑手 #" + strconv.FormatUint(row.RiderUserID, 10), Section: "delivery",
		})
	}
	return hits, nil
}

func escapeSearchLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}
