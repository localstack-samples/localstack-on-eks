package strings

import (
	"reflect"
	"testing"
)

func TestRsplitN(t *testing.T) {
	tests := []struct {
		s      string
		sep    string
		n      int
		result []string
	}{
		{"a/b/c/d/e", "/", 2, []string{"a/b/c", "d", "e"}},
		{"a/b/c/d/e", "/", 3, []string{"a/b", "c", "d", "e"}},
		{"a/b/c/d/e", "/", 4, []string{"a", "b", "c", "d", "e"}},
		{"a/b/c/d/e", "/", -1, nil},
		{"a/b/c/d/e", "/", 0, nil},
		{"a/b/c/d/e", "-", 2, []string{"a/b/c/d/e"}},
		{"a/b/c/d/e", "-", 3, []string{"a/b/c/d/e"}},
		{"a/b/c/d/e", "-", 4, []string{"a/b/c/d/e"}},
		{"a/b/c/d/e", "b", 1, []string{"a/", "/c/d/e"}},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			result := RsplitN(test.s, test.sep, test.n)
			if !reflect.DeepEqual(result, test.result) {
				t.Errorf("RsplitN(%q, %q, %d) = %v; want %v", test.s, test.sep, test.n, result, test.result)
			}
		})
	}
}
