package strings

import "strings"

// SplitN splits a string into substrings by the given separator.
// Max number of substrings returned is n.
func RsplitN(s string, sep string, n int) []string {
	if n <= 0 {
		return nil
	}
	result := []string{}
	for i := 0; i < n; i++ {
		index := strings.LastIndex(s, sep)
		if index == -1 {
			break
		}
		result = append([]string{s[index+len(sep):]}, result...)
		s = s[:index]
	}
	result = append([]string{s}, result...)
	return result
}