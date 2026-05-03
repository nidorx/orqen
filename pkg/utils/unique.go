package utils

func Unique(strSlice []string) []string {
	keys := make(map[string]struct{})
	list := []string{}

	for _, entry := range strSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = struct{}{}
			list = append(list, entry)
		}
	}
	return list
}
