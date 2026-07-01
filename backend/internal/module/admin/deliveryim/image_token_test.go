package deliveryim

import "testing"

func TestImageTokenIsConversationBoundAndTamperEvident(t *testing.T) {
	codec, err := NewImageTokenCodec("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	conversationID := "01J00000000000000000000001"
	token := codec.Encode(conversationID, 17)
	slot, err := codec.Decode(token, conversationID)
	if err != nil || slot != 17 {
		t.Fatalf("slot=%d err=%v", slot, err)
	}
	if _, err := codec.Decode(token, "01J00000000000000000000002"); err == nil {
		t.Fatal("cross-conversation token must be rejected")
	}
	index := len(token) / 2
	replacement := byte('A')
	if token[index] == replacement {
		replacement = 'B'
	}
	tampered := token[:index] + string(replacement) + token[index+1:]
	if _, err := codec.Decode(tampered, conversationID); err == nil {
		t.Fatal("tampered token must be rejected")
	}
}
