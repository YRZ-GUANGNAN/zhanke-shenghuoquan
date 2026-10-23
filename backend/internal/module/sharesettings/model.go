package sharesettings

import (
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var ErrConflict = errors.New("分享配置已更新，请重新读取后保存")
var ErrInvalid = errors.New("分享配置格式无效")

type Cards struct {
	HomeTitle          string `json:"homeTitle"`
	SecondhandTitle    string `json:"secondhandTitle"`
	ServicesTitle      string `json:"servicesTitle"`
	DefaultImageURL    string `json:"defaultImageUrl"`
	DetailTitleSuffix  string `json:"detailTitleSuffix"`
	PreferContentImage bool   `json:"preferContentImage"`
}

type Settings struct {
	Cards
	RowVersion uint64    `json:"rowVersion"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type UpdateInput struct {
	Cards
	ExpectedRowVersion uint64 `json:"expectedRowVersion"`
}

func Defaults() Cards {
	return Cards{HomeTitle: "湛科生活圈", SecondhandTitle: "校园二手 · 湛科生活圈", ServicesTitle: "校园服务 · 湛科生活圈", DetailTitleSuffix: "湛科生活圈", PreferContentImage: true}
}

func (v *Cards) Validate() error {
	for _, field := range []*string{&v.HomeTitle, &v.SecondhandTitle, &v.ServicesTitle, &v.DetailTitleSuffix} {
		*field = strings.TrimSpace(*field)
		if !utf8.ValidString(*field) || utf8.RuneCountInString(*field) > 60 || strings.IndexFunc(*field, unicode.IsControl) >= 0 {
			return errors.New("标题限 60 个字符，不能包含换行或控制字符")
		}
	}
	if v.HomeTitle == "" || v.SecondhandTitle == "" || v.ServicesTitle == "" {
		return errors.New("首页、二手市场和校园服务的分享标题不能为空")
	}
	v.DefaultImageURL = strings.TrimSpace(v.DefaultImageURL)
	if v.DefaultImageURL == "" {
		return nil
	}
	u, err := url.Parse(v.DefaultImageURL)
	if err != nil || len(v.DefaultImageURL) > 2048 || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.Port() != "" || u.RawQuery != "" {
		return errors.New("封面请使用不含临时签名、账号或端口的 HTTPS 图片地址")
	}
	// Covers must be stable public media on our site or the project's COS bucket.
	host := strings.ToLower(u.Hostname())
	if host != "zhanduiwang.top" && !strings.HasSuffix(host, ".myqcloud.com") {
		return errors.New("封面仅支持 zhanduiwang.top 或腾讯云 COS 的已公开图片")
	}
	path := strings.ToLower(u.Path)
	if !strings.HasSuffix(path, ".jpg") && !strings.HasSuffix(path, ".jpeg") && !strings.HasSuffix(path, ".png") {
		return errors.New("微信分享封面必须是 JPG 或 PNG 图片")
	}
	return nil
}
