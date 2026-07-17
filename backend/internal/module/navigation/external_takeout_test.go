package navigation

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func validExternalTakeoutMerchantInput() ExternalTakeoutMerchantInput {
	return ExternalTakeoutMerchantInput{
		Title:    "校内餐厅",
		Subtitle: "一食堂二楼",
		IconText: "餐厅",
		AppID:    "wx1234567890ABCDEF",
		Path:     "pages/store/index?merchant=42",
		Color:    "#1A2b3C",
		Sort:     0,
	}
}

func TestNormalizeExternalTakeoutMerchantInputTrimsAndNormalizes(t *testing.T) {
	input := validExternalTakeoutMerchantInput()
	input.Title = " \t校内餐厅\n"
	input.Subtitle = "  一食堂二楼  "
	input.IconText = "\n餐厅\t"
	input.AppID = " wx1234567890ABCDEF "
	input.Path = " /pages/store/index?merchant=42 "
	input.Color = " #1A2b3C "
	input.Sort = 10000

	got, err := normalizeExternalTakeoutMerchantInput(input)
	if err != nil {
		t.Fatalf("normalize valid input: %v", err)
	}

	want := validExternalTakeoutMerchantInput()
	want.Sort = 10000
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized input mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestNormalizeExternalTakeoutMerchantInputTextRuneBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*ExternalTakeoutMerchantInput)
		wantErr bool
	}{
		{name: "title 64 runes", mutate: func(v *ExternalTakeoutMerchantInput) { v.Title = strings.Repeat("商", 64) }},
		{name: "title 65 runes", mutate: func(v *ExternalTakeoutMerchantInput) { v.Title = strings.Repeat("商", 65) }, wantErr: true},
		{name: "subtitle 120 runes", mutate: func(v *ExternalTakeoutMerchantInput) { v.Subtitle = strings.Repeat("介", 120) }},
		{name: "subtitle 121 runes", mutate: func(v *ExternalTakeoutMerchantInput) { v.Subtitle = strings.Repeat("介", 121) }, wantErr: true},
		{name: "icon text 12 runes", mutate: func(v *ExternalTakeoutMerchantInput) { v.IconText = strings.Repeat("店", 12) }},
		{name: "icon text 13 runes", mutate: func(v *ExternalTakeoutMerchantInput) { v.IconText = strings.Repeat("店", 13) }, wantErr: true},
		{name: "path 240 runes", mutate: func(v *ExternalTakeoutMerchantInput) { v.Path = strings.Repeat("页", 240) }},
		{name: "path 241 runes", mutate: func(v *ExternalTakeoutMerchantInput) { v.Path = strings.Repeat("页", 241) }, wantErr: true},
		{name: "decoded path 240 runes", mutate: func(v *ExternalTakeoutMerchantInput) { v.Path = strings.Repeat("%E9%A1%B5", 240) }},
		{name: "decoded path 241 runes", mutate: func(v *ExternalTakeoutMerchantInput) { v.Path = strings.Repeat("%E9%A1%B5", 241) }, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validExternalTakeoutMerchantInput()
			tt.mutate(&input)

			_, err := normalizeExternalTakeoutMerchantInput(input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeExternalTakeoutMerchantInputRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ExternalTakeoutMerchantInput)
	}{
		{name: "blank title", mutate: func(v *ExternalTakeoutMerchantInput) { v.Title = " \t\n " }},
		{name: "blank icon text", mutate: func(v *ExternalTakeoutMerchantInput) { v.IconText = "  " }},
		{name: "unicode line separator in title", mutate: func(v *ExternalTakeoutMerchantInput) { v.Title = "商家\u2028名称" }},
		{name: "unicode paragraph separator in subtitle", mutate: func(v *ExternalTakeoutMerchantInput) { v.Subtitle = "简介\u2029门店" }},
		{name: "appid too short", mutate: func(v *ExternalTakeoutMerchantInput) { v.AppID = "wx1234567890ABCDE" }},
		{name: "appid too long", mutate: func(v *ExternalTakeoutMerchantInput) { v.AppID = "wx1234567890ABCDEFG" }},
		{name: "appid wrong prefix", mutate: func(v *ExternalTakeoutMerchantInput) { v.AppID = "WX1234567890ABCDEF" }},
		{name: "appid punctuation", mutate: func(v *ExternalTakeoutMerchantInput) { v.AppID = "wx1234567890ABCDE_" }},
		{name: "color short", mutate: func(v *ExternalTakeoutMerchantInput) { v.Color = "#12345" }},
		{name: "color missing hash", mutate: func(v *ExternalTakeoutMerchantInput) { v.Color = "123456" }},
		{name: "color non hex", mutate: func(v *ExternalTakeoutMerchantInput) { v.Color = "#12345G" }},
		{name: "sort below range", mutate: func(v *ExternalTakeoutMerchantInput) { v.Sort = -1 }},
		{name: "sort above range", mutate: func(v *ExternalTakeoutMerchantInput) { v.Sort = 10001 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validExternalTakeoutMerchantInput()
			tt.mutate(&input)

			if _, err := normalizeExternalTakeoutMerchantInput(input); err == nil {
				t.Fatal("expected invalid input to be rejected")
			}
		})
	}
}

func TestNormalizeExternalTakeoutMerchantInputRejectsInvalidUTF8InEveryStringField(t *testing.T) {
	invalid := string([]byte{0xff})
	tests := []struct {
		name   string
		mutate func(*ExternalTakeoutMerchantInput)
	}{
		{name: "title", mutate: func(v *ExternalTakeoutMerchantInput) { v.Title = invalid }},
		{name: "subtitle", mutate: func(v *ExternalTakeoutMerchantInput) { v.Subtitle = invalid }},
		{name: "icon text", mutate: func(v *ExternalTakeoutMerchantInput) { v.IconText = invalid }},
		{name: "appid", mutate: func(v *ExternalTakeoutMerchantInput) { v.AppID = "wx1234567890ABCDE" + invalid }},
		{name: "path", mutate: func(v *ExternalTakeoutMerchantInput) { v.Path = "pages/" + invalid }},
		{name: "color", mutate: func(v *ExternalTakeoutMerchantInput) { v.Color = "#12345" + invalid }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validExternalTakeoutMerchantInput()
			tt.mutate(&input)
			if _, err := normalizeExternalTakeoutMerchantInput(input); err == nil {
				t.Fatal("expected invalid UTF-8 to be rejected")
			}
		})
	}
}

func TestNormalizeExternalTakeoutMiniProgramPathAcceptsSafePaths(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "plain path", raw: "pages/store/index", want: "pages/store/index"},
		{name: "one leading slash", raw: "/pages/store/index", want: "pages/store/index"},
		{name: "safe query", raw: "/pages/store/index?merchant=42&scene=campus", want: "pages/store/index?merchant=42&scene=campus"},
		{name: "encoded slash", raw: "pages%2Fstore/index", want: "pages/store/index"},
		{name: "encoded leading slash", raw: "%2Fpages/store/index", want: "pages/store/index"},
		{name: "encoded unicode", raw: "pages/%E5%95%86%E5%AE%B6/index", want: "pages/商家/index"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeMiniProgramPath(tt.raw)
			if err != nil {
				t.Fatalf("normalize valid path: %v", err)
			}
			if got != tt.want {
				t.Fatalf("path = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeExternalTakeoutMiniProgramPathCanonicalizesEquivalentForms(t *testing.T) {
	tests := []struct {
		name     string
		variants []string
		want     string
	}{
		{name: "encoded ascii path", variants: []string{"%70ages/store", "pages/store"}, want: "pages/store"},
		{name: "encoded unicode path", variants: []string{"pages/%E5%95%86%E5%AE%B6", "pages/商家"}, want: "pages/商家"},
		{name: "encoded query key", variants: []string{"pages/store?%61=1", "pages/store?a=1"}, want: "pages/store?a=1"},
		{name: "query space", variants: []string{"pages/store?a=hello%20world", "pages/store?a=hello+world"}, want: "pages/store?a=hello+world"},
		{name: "query key order", variants: []string{"pages/store?b=2&a=1", "pages/store?a=1&b=2"}, want: "pages/store?a=1&b=2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, raw := range tt.variants {
				got, err := normalizeMiniProgramPath(raw)
				if err != nil {
					t.Fatalf("normalize %q: %v", raw, err)
				}
				if got != tt.want {
					t.Fatalf("normalize %q = %q, want %q", raw, got, tt.want)
				}
				again, err := normalizeMiniProgramPath(got)
				if err != nil || again != got {
					t.Fatalf("canonical path is not idempotent: first=%q second=%q err=%v", got, again, err)
				}
			}
		})
	}
}

func TestNormalizeExternalTakeoutMiniProgramPathRejectsUnsafeRawAndDecodedContent(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "empty", raw: ""},
		{name: "only slash", raw: "/"},
		{name: "only whitespace", raw: " \t\n "},
		{name: "http scheme", raw: "http://evil.example/path"},
		{name: "https scheme", raw: "https://evil.example/path"},
		{name: "other scheme", raw: "custom:pages/store/index"},
		{name: "host", raw: "//evil.example/path"},
		{name: "mixed encoded host", raw: "/%2Fevil.example/path"},
		{name: "double slash", raw: "pages//store/index"},
		{name: "backslash", raw: `pages\store\index`},
		{name: "dot dot segment", raw: "pages/../index"},
		{name: "dot dot substring", raw: "pages/store..backup/index"},
		{name: "fragment", raw: "pages/store/index#section"},
		{name: "leading control before trim", raw: "\t/pages/store/index"},
		{name: "trailing control before trim", raw: "/pages/store/index\n"},
		{name: "raw control", raw: "pages/store\n/index"},
		{name: "raw del control", raw: "pages/store\x7f/index"},
		{name: "unicode control", raw: "pages/store\u0085/index"},
		{name: "unicode format control", raw: "pages/store\u202E/index"},
		{name: "unicode line separator", raw: "pages/store\u2028/index"},
		{name: "unicode paragraph separator", raw: "pages/store\u2029/index"},
		{name: "invalid utf8", raw: "pages/" + string([]byte{0xff}) + "/index"},
		{name: "invalid path percent", raw: "pages/%zz/index"},
		{name: "invalid query percent", raw: "pages/store/index?merchant=%zz"},
		{name: "decoded invalid utf8 path", raw: "pages/%FF/index"},
		{name: "decoded invalid utf8 query key", raw: "pages/store/index?%FF=value"},
		{name: "decoded invalid utf8 query value", raw: "pages/store/index?safe=%FF"},
		{name: "partial query parse error", raw: "pages/store/index?safe=1;other=2"},
		{name: "decoded lower dot dot", raw: "pages/%2e%2e/index"},
		{name: "decoded upper dot dot", raw: "pages/%2E%2E/index"},
		{name: "decoded lower backslash", raw: "pages/%5c/index"},
		{name: "decoded upper backslash", raw: "pages/%5C/index"},
		{name: "decoded nul", raw: "pages/%00/index"},
		{name: "decoded del", raw: "pages/%7F/index"},
		{name: "decoded c1", raw: "pages/%C2%85/index"},
		{name: "decoded lower double slash", raw: "pages/%2f%2fhost/index"},
		{name: "decoded upper double slash", raw: "pages/%2F%2Fhost/index"},
		{name: "decoded fragment", raw: "pages/store%23section/index"},
		{name: "decoded question delimiter", raw: "pages%3Fquery"},
		{name: "decoded scheme", raw: "custom%3Apages/store"},
		{name: "decoded bare percent", raw: "pages/%25/index"},
		{name: "decoded format control", raw: "pages/%E2%80%AE/index"},
		{name: "decoded line separator", raw: "pages/%E2%80%A8/index"},
		{name: "decoded paragraph separator", raw: "pages/%E2%80%A9/index"},
		{name: "decoded query dot dot", raw: "pages/store/index?next=%2e%2e"},
		{name: "decoded query backslash", raw: "pages/store/index?next=%5Cadmin"},
		{name: "decoded query nul", raw: "pages/store/index?next=%00"},
		{name: "decoded query del", raw: "pages/store/index?next=%7F"},
		{name: "decoded query c1", raw: "pages/store/index?next=%C2%85"},
		{name: "decoded query format control", raw: "pages/store/index?next=%E2%80%AE"},
		{name: "decoded query line separator", raw: "pages/store/index?next=%E2%80%A8"},
		{name: "decoded query paragraph separator", raw: "pages/store/index?next=%E2%80%A9"},
		{name: "decoded query double slash", raw: "pages/store/index?next=%2F%2Fevil"},
		{name: "decoded query fragment", raw: "pages/store/index?next=%23section"},
		{name: "double encoded lower dot dot", raw: "pages/%252e%252e/index"},
		{name: "double encoded upper dot dot", raw: "pages/%252E%252E/index"},
		{name: "double encoded backslash", raw: "pages/%255c/index"},
		{name: "double encoded nul", raw: "pages/%2500/index"},
		{name: "double encoded double slash", raw: "pages/%252f%252fhost/index"},
		{name: "double encoded fragment", raw: "pages/store%2523section/index"},
		{name: "double encoded query control", raw: "pages/store/index?next=%2500"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := normalizeMiniProgramPath(tt.raw); err == nil {
				t.Fatal("expected unsafe path to be rejected")
			}
		})
	}
}

func TestNormalizeExternalTakeoutMiniProgramPathChecksCanonicalLength(t *testing.T) {
	valid := "p?q=" + strings.Repeat("商", 26)
	got, err := normalizeMiniProgramPath(valid)
	if err != nil {
		t.Fatalf("normalize 238-rune canonical path: %v", err)
	}
	if count := len([]rune(got)); count != 238 {
		t.Fatalf("canonical length = %d, want 238 (%q)", count, got)
	}

	invalid := "p?q=" + strings.Repeat("商", 27)
	if _, err := normalizeMiniProgramPath(invalid); err == nil {
		t.Fatal("expected 247-rune canonical path to be rejected")
	}
}

func TestNormalizeExternalTakeoutMiniProgramPathRejectsSensitiveQueryKeys(t *testing.T) {
	paths := []string{
		"pages/store/index?token=value",
		"pages/store/index?SECRET=value",
		"pages/store/index?OpenId=value",
		"pages/store/index?SeSsIoN=value",
		"pages/store/index?AUTHORIZATION=value",
		"pages/store/index?%74oken=value",
		"pages/store/index?%54%4f%4B%45%4e=value",
		"pages/store/index?%2574oken=value",
		"pages/store/index?safe=1&TOKEN=first&token=second",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			if _, err := normalizeMiniProgramPath(path); err == nil {
				t.Fatal("expected sensitive query key to be rejected")
			}
		})
	}
}

func TestNormalizeExternalTakeoutErrorsDoNotLeakPathOrQueryValues(t *testing.T) {
	input := validExternalTakeoutMerchantInput()
	input.Path = "pages/store/index?ToKeN=do-not-leak-42&safe=also-private"

	_, err := normalizeExternalTakeoutMerchantInput(input)
	if err == nil {
		t.Fatal("expected sensitive query key to be rejected")
	}

	message := err.Error()
	for _, secret := range []string{input.Path, "do-not-leak-42", "also-private"} {
		if strings.Contains(message, secret) {
			t.Fatalf("error leaked sensitive input %q: %q", secret, message)
		}
	}
}

func TestExternalTakeoutPublicDTOExcludesManagementFields(t *testing.T) {
	for _, value := range []any{PublicItem{}, PublicCategory{}} {
		typeOf := reflect.TypeOf(value)
		for _, field := range []string{"Enabled", "CreatedAt", "UpdatedAt", "CategoryID"} {
			if _, ok := typeOf.FieldByName(field); ok {
				t.Fatalf("%s unexpectedly exposes management field %s", typeOf.Name(), field)
			}
		}
	}

	encoded, err := json.Marshal(PublicCategory{Items: []PublicItem{{}}})
	if err != nil {
		t.Fatalf("marshal public DTO: %v", err)
	}
	for _, key := range []string{`"enabled"`, `"createdAt"`, `"updatedAt"`, `"categoryId"`} {
		if strings.Contains(string(encoded), key) {
			t.Fatalf("public DTO JSON unexpectedly contains %s: %s", key, encoded)
		}
	}
}

func TestExternalTakeoutDTOFieldBoundaries(t *testing.T) {
	assertFields := func(t *testing.T, value any, want []string) {
		t.Helper()
		typeOf := reflect.TypeOf(value)
		got := make([]string, 0, typeOf.NumField())
		for i := 0; i < typeOf.NumField(); i++ {
			got = append(got, typeOf.Field(i).Name)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s fields = %v, want %v", typeOf.Name(), got, want)
		}
	}

	assertFields(t, ExternalTakeoutMerchantInput{}, []string{
		"Title", "Subtitle", "IconText", "AppID", "Path", "Color", "Sort",
	})
	assertFields(t, ExternalTakeoutMerchant{}, []string{
		"ID", "Title", "Subtitle", "IconText", "AppID", "Path", "Color", "Sort", "Enabled", "OperatingStatus",
	})
	assertFields(t, ExternalTakeoutDirectory{}, []string{"CategoryEnabled", "Items"})
	assertFields(t, PublicItem{}, []string{
		"ID", "Title", "Subtitle", "IconText", "AppID", "Path", "Color", "Sort", "OperatingStatus",
	})
	assertFields(t, PublicCategory{}, []string{"ID", "Name", "SystemKey", "Sort", "Items"})

	for name, value := range map[string]any{
		"ExternalTakeoutDirectory": ExternalTakeoutDirectory{Items: []ExternalTakeoutMerchant{}},
		"PublicCategory":           PublicCategory{Items: []PublicItem{}},
	} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		if !strings.Contains(string(encoded), `"items":[]`) {
			t.Fatalf("%s does not serialize an initialized empty Items slice as []: %s", name, encoded)
		}
	}

	assertJSONTag := func(t *testing.T, value any, fieldName, want string) {
		t.Helper()
		field, ok := reflect.TypeOf(value).FieldByName(fieldName)
		if !ok {
			t.Fatalf("%T.%s is missing", value, fieldName)
		}
		if got := field.Tag.Get("json"); got != want {
			t.Fatalf("%T.%s json tag = %q, want %q", value, fieldName, got, want)
		}
	}
	assertJSONTag(t, ExternalTakeoutMerchant{}, "AppID", "appId")
	assertJSONTag(t, ExternalTakeoutMerchant{}, "OperatingStatus", "operatingStatus")
	assertJSONTag(t, ExternalTakeoutDirectory{}, "CategoryEnabled", "categoryEnabled")
	assertJSONTag(t, PublicItem{}, "AppID", "appId")
	assertJSONTag(t, PublicItem{}, "OperatingStatus", "operatingStatus")
	assertJSONTag(t, PublicCategory{}, "SystemKey", "systemKey,omitempty")
}

func TestExternalTakeoutModelUsesBinaryCollation(t *testing.T) {
	categoryField, ok := reflect.TypeOf(Category{}).FieldByName("SystemKey")
	if !ok {
		t.Fatal("Category.SystemKey is missing")
	}
	itemField, ok := reflect.TypeOf(Item{}).FieldByName("OperatingStatus")
	if !ok {
		t.Fatal("Item.OperatingStatus is missing")
	}

	for name, tag := range map[string]string{
		"Category.SystemKey":   categoryField.Tag.Get("gorm"),
		"Item.OperatingStatus": itemField.Tag.Get("gorm"),
	} {
		if !strings.Contains(tag, "utf8mb4_0900_bin") {
			t.Fatalf("%s gorm tag does not pin binary collation: %q", name, tag)
		}
	}
	if tag := categoryField.Tag.Get("gorm"); !strings.Contains(tag, "uniqueIndex:uq_navigation_categories_system_key") {
		t.Fatalf("Category.SystemKey gorm tag does not pin the unique index: %q", tag)
	}
	if tag := categoryField.Tag.Get("gorm"); !strings.Contains(tag, "size:64") {
		t.Fatalf("Category.SystemKey gorm tag does not pin the field size: %q", tag)
	}
	if tag := itemField.Tag.Get("gorm"); !strings.Contains(tag, "size:16") || !strings.Contains(tag, "not null") || !strings.Contains(tag, "default:open") {
		t.Fatalf("Item.OperatingStatus gorm tag does not pin null/default semantics: %q", tag)
	}
}

func TestOperatingStatus(t *testing.T) {
	tests := []struct {
		value OperatingStatus
		want  bool
	}{
		{value: OperatingStatusOpen, want: true},
		{value: OperatingStatusResting, want: true},
		{value: "", want: false},
		{value: "Open", want: false},
		{value: "open ", want: false},
		{value: "closed", want: false},
	}

	for _, tt := range tests {
		t.Run(string(tt.value), func(t *testing.T) {
			if got := validOperatingStatus(tt.value); got != tt.want {
				t.Fatalf("validOperatingStatus(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
