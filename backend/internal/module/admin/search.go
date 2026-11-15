package admin

import (
	"context"
)

// SearchHit 是全局搜索的统一结果条目（管理端顶栏搜索）。
type SearchHit struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Section  string `json:"section"`
}

// GlobalSearch 跨订单、用户、帖子、商品与骑手做前缀/模糊搜索，每类最多 5 条。
func (s *Service) GlobalSearch(ctx context.Context, keyword string) ([]SearchHit, error) {
	return s.repo.GlobalSearch(ctx, keyword)
}
