package utils

func Map[S ~[]I, I any, O any](input S, mapper func(I) O) []O {
	output := make([]O, len(input))
	for _, v := range input {
		output = append(output, mapper(v))
	}
	return output
}
