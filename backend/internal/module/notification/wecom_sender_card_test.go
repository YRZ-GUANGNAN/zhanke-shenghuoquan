package notification

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMarshalWecomTemplateCardStructure(t *testing.T) {
	body, err := marshalWecomTemplateCard(WecomTemplateCard{
		SourceDesc: "湛科生活圈 · 京东订单", Title: "骑手已接单", Desc: "京东订单 · 单号 01M7F1…X8SF",
		EmphasisTitle: "2元", EmphasisDesc: "赏金",
		Items:     []WecomCardItem{{Key: "单号", Value: "01M7F1YAG2S54TQK57G3MXX8SF"}, {Key: "时间", Value: "11-15 15:32"}},
		ActionURL: "https://zhanduiwang.top/",
	}, time.Date(2026, 11, 15, 7, 32, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		MsgType      string `json:"msgtype"`
		TemplateCard struct {
			CardType string `json:"card_type"`
			Source   struct {
				Desc string `json:"desc"`
			} `json:"source"`
			MainTitle  struct{ Title, Desc string } `json:"main_title"`
			Emphasis   struct{ Title, Desc string } `json:"emphasis_content"`
			Horizontal []map[string]string          `json:"horizontal_content_list"`
			CardAction struct {
				Type int    `json:"type"`
				URL  string `json:"url"`
			} `json:"card_action"`
		} `json:"template_card"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.MsgType != "template_card" || payload.TemplateCard.CardType != "text_notice" {
		t.Fatalf("wrong type: %s %s", payload.MsgType, payload.TemplateCard.CardType)
	}
	if payload.TemplateCard.Source.Desc != "湛科生活圈 · 京东订单" ||
		payload.TemplateCard.MainTitle.Title != "骑手已接单" ||
		payload.TemplateCard.Emphasis.Title != "2元" || payload.TemplateCard.Emphasis.Desc != "赏金" {
		t.Fatalf("wrong card head: %+v", payload.TemplateCard)
	}
	if len(payload.TemplateCard.Horizontal) != 2 || payload.TemplateCard.Horizontal[0]["keyname"] != "单号" {
		t.Fatalf("wrong items: %+v", payload.TemplateCard.Horizontal)
	}
	if payload.TemplateCard.CardAction.Type != 1 || payload.TemplateCard.CardAction.URL != "https://zhanduiwang.top/" {
		t.Fatalf("wrong action: %+v", payload.TemplateCard.CardAction)
	}
	if strings.Contains(string(body), "private-pickup") || strings.Contains(string(body), "markdown") {
		t.Fatal("card payload must not fall back to markdown or carry address data")
	}
}

func TestMarshalWecomTemplateCardMiniAppAction(t *testing.T) {
	body, err := marshalWecomTemplateCard(WecomTemplateCard{
		SourceDesc: "湛科生活圈 · 京东订单", Title: "骑手已接单", Desc: "京东订单 · 单号 01M7F1…X8SF",
		Items:          []WecomCardItem{{Key: "单号", Value: "01M7F1YAG2S54TQK57G3MXX8SF"}},
		ActionURL:      "https://zhanduiwang.top/",
		ActionAppID:    "wx0123456789abcdef",
		ActionPagePath: "pages/delivery-rider/index",
	}, time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		TemplateCard struct {
			CardAction struct {
				Type     int    `json:"type"`
				URL      string `json:"url"`
				AppID    string `json:"appid"`
				PagePath string `json:"pagepath"`
			} `json:"card_action"`
		} `json:"template_card"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	action := payload.TemplateCard.CardAction
	if action.Type != 2 || action.AppID != "wx0123456789abcdef" || action.PagePath != "pages/delivery-rider/index" ||
		action.URL != "https://zhanduiwang.top/" {
		t.Fatalf("wrong miniapp action: %+v", action)
	}
}

func TestMarshalWecomTemplateCardFallsBackOnInvalidApp(t *testing.T) {
	for _, card := range []WecomTemplateCard{
		{Title: "t", ActionAppID: "not-an-appid", ActionPagePath: "pages/delivery-rider/index"},
		{Title: "t", ActionAppID: "wx0123456789abcdef", ActionPagePath: "https://evil.example/steal"},
		{Title: "t", ActionAppID: "wx0123456789abcdef", ActionPagePath: "/pages/delivery-rider/index"},
	} {
		body, err := marshalWecomTemplateCard(card, time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatal(err)
		}
		var payload struct {
			TemplateCard struct {
				CardAction struct {
					Type int `json:"type"`
				} `json:"card_action"`
			} `json:"template_card"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatal(err)
		}
		if payload.TemplateCard.CardAction.Type != 1 {
			t.Fatalf("expected URL fallback, got %+v", payload.TemplateCard.CardAction)
		}
	}
}
