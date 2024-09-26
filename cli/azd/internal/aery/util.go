package aery

func commonPrefix(a, b string) string {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[:i]
		}
	}

	if len(a) < len(b) {
		return a
	}

	return b
}
