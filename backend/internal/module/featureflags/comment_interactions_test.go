package featureflags

import "testing"

func TestCommentInteractionRoutesKeepForumCommentAndLikeGates(t *testing.T) {
	cases := []struct {
		method, path string
		want         []string
	}{
		{"GET", "/community/posts/8/comments/threads", []string{KeyForum, KeyComment}},
		{"POST", "/community/posts/8/comments/1/replies", []string{KeyForum, KeyComment}},
		{"PUT", "/community/posts/8/comments/1/like", []string{KeyForum, KeyComment, KeyLikes}},
		{"DELETE", "/community/posts/8/comments/1/like", []string{KeyForum, KeyComment, KeyLikes}},
	}
	for _, test := range cases {
		got := RequestKeys(test.method, test.path)
		for _, key := range test.want {
			found := false
			for _, item := range got {
				if item == key {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("%s %s missing gate %s: %#v", test.method, test.path, key, got)
			}
		}
	}
}
