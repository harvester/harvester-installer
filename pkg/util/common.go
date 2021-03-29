package util

func StringSliceContains(sSlice []string, s string) bool {
	for _, target := range sSlice {
		if target == s {
			return true
		}
	}
	return false
}

func DupStrings(src []string) []string {
	if src == nil {
		return nil
	}
	s := make([]string, len(src))
	copy(s, src)
	return s
}

func DupStringMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	m := make(map[string]string)
	for k, v := range src {
		m[k] = v
	}
	return m
}
