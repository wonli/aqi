package sensitive

import (
	"strings"
	"sync"

	"github.com/importcjj/sensitive"
)

var Entity *sensitive.Filter
var sensitiveOnce sync.Once

func Init() {
	sensitiveOnce.Do(func() {
		Entity = sensitive.New()
		dd, err := Words.ReadDir("words")
		if err != nil {
			return
		}
		for _, v := range dd {
			data, err := Words.ReadFile("words/" + v.Name())
			if err != nil {
				continue
			}

			Entity.AddWord(strings.Split(string(data), "\n")...)
		}
	})
}

func Replace(msg string) string {
	return Entity.Replace(msg, '*')
}
