// Package contactsettings owns the single-row platform contact configuration:
// the WeChat id and intro copy shown in the maker-space application dialog.
package contactsettings

import (
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	ErrInvalid  = errors.New("联系设置格式无效")
	ErrConflict = errors.New("联系设置已更新，请重新读取后保存")
)

type Settings struct {
	MakerWechat string    `json:"makerWechat"`
	MakerIntro  string    `json:"makerIntro"`
	RowVersion  uint64    `json:"rowVersion"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type UpdateInput struct {
	MakerWechat        string `json:"makerWechat"`
	MakerIntro         string `json:"makerIntro"`
	ExpectedRowVersion uint64 `json:"expectedRowVersion"`
}

const (
	DefaultMakerWechat = "zhanke-zhongchuang"
	DefaultMakerIntro  = "众创空间入驻与合作，请添加微信，添加时请备注「众创申请」。"
)

func newlineAllowed(r rune) bool { return r == '\n' }

func (v *UpdateInput) Validate() error {
	v.MakerWechat = strings.TrimSpace(v.MakerWechat)
	if utf8.RuneCountInString(v.MakerWechat) == 0 {
		return errors.New("微信号不能为空")
	}
	if !utf8.ValidString(v.MakerWechat) || utf8.RuneCountInString(v.MakerWechat) > 40 ||
		strings.IndexFunc(v.MakerWechat, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) >= 0 {
		return errors.New("微信号限 40 个字符，不能包含空格或控制字符")
	}
	v.MakerIntro = strings.TrimSpace(v.MakerIntro)
	if !utf8.ValidString(v.MakerIntro) || utf8.RuneCountInString(v.MakerIntro) > 500 {
		return errors.New("弹窗文案限 500 个字符")
	}
	if strings.IndexFunc(v.MakerIntro, func(r rune) bool { return unicode.IsControl(r) && !newlineAllowed(r) }) >= 0 {
		return errors.New("弹窗文案不能包含控制字符")
	}
	return nil
}

// Row is the single persisted configuration row (id = 1).
type Row struct {
	ID          uint8     `gorm:"primaryKey;column:id"`
	MakerWechat string    `gorm:"column:maker_wechat"`
	MakerIntro  string    `gorm:"column:maker_intro"`
	RowVersion  uint64    `gorm:"column:row_version"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (Row) TableName() string { return "contact_settings" }

func (r Row) Settings() Settings {
	return Settings{MakerWechat: r.MakerWechat, MakerIntro: r.MakerIntro, RowVersion: r.RowVersion, UpdatedAt: r.UpdatedAt}
}
