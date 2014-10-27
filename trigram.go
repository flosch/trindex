package trindex

import (
	"strings"
)

func trigramize(data string) (trigrams []string) {
	data = strings.ToLower(data)
	data_runes := []rune(data)

	for len(data_runes) < 3 {
		data_runes = append(data_runes, ' ')
	}

	empty := struct{}{}
	trigram_set := make(map[string]struct{})

	dl := len(data_runes)
	for i := 0; i < dl-2; i++ {
		trigram_set[string(data_runes[i:i+3])] = empty
	}

	trigram_set[string(data_runes[dl-2:dl])] = empty
	trigram_set[string(data_runes[dl-1:dl])] = empty
	trigram_set[string(data_runes[0:1])] = empty
	trigram_set[string(data_runes[0:2])] = empty

	for k, _ := range trigram_set {
		trigrams = append(trigrams, k)
	}

	return
}
