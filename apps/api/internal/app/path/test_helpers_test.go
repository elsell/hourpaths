package path

func sequentialIDs(values ...string) func() string {
	index := 0
	return func() string {
		if index >= len(values) {
			return "extra-id"
		}
		value := values[index]
		index++
		return value
	}
}
