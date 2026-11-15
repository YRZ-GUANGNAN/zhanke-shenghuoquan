package notification

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMarshalWecomTemplateCardStructure(t *testing.T) {
	body, err := marshalWecomTemplateCard(WecomTemplateCard{
		SourceDesc: "湛科生活圈 · 京东订单", Title: "新订单通知", Desc: "京东订单 · 配送单等待接单",
		EmphasisTitle: "2元", EmphasisDesc: "赏金",
		Items:     []WecomCardItem{{Key: "单号", Value: "01M7F1YAG2S54TQK57G3MXX8SF"}, {Key: "状态", Value: "配送单等待接单"}},
		ActionURL: "https://zhanduiwang.top/",
	}, time.Date(2026, 11, 15, 7, 32, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		MsgType      string `json:"msgtype"`
		TemplateCard struct {
			CardType    string `json:"card_type"`
			Source      struct{ Desc string `json:"desc"` } `json:"source"`
			MainTitle   struct{ Title, Desc string } `json:"main_title"`
			Emphasis    struct{ Title, Desc string } `json:"emphasis_content"`
			Horizontal  []map[string]string          `json:"horizontal_content_list"`
			CardAction  struct {
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
		payload.TemplateCard.MainTitle.Title != "新订单通知" ||
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
