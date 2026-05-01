package conf

func create[T any]() (func(T), func() *T) {
	var (
		t *T
		s = func(v T) {
			t = &v
		}
		g = func() *T {
			return t
		}
	)

	return s, g
}
