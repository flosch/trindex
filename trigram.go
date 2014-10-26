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

	trigrams = make([]string, 0, len(data_runes)-2)

	dl := len(data_runes)
	for i := 0; i < dl-2; i++ {
		trigrams = append(trigrams, string(data_runes[i:i+3]))
	}

	trigrams = append(trigrams, string(data_runes[dl-2:dl]))
	trigrams = append(trigrams, string(data_runes[dl-1:dl]))
	trigrams = append(trigrams, string(data_runes[0:1]))
	trigrams = append(trigrams, string(data_runes[0:2]))

	return
}
