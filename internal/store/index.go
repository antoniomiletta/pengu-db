package store

type Index struct {
	Maps map[string]int
}

func BuildIndex() *Index {
	return &Index{
		Maps: map[string]int{
			"ai": 0,
		},
	}
}
