package encrypt

import (
	"fmt"
	"testing"
	"time"
)

func TestAzdg(t *testing.T) {
	t1 := time.Now()
	amap := map[string]bool{}
	azdg := NewAzdg("123")
	for i := 0; i < 10000; i++ {
		s := azdg.Encrypt("hello world 我是中文")
		_, ok := amap[s]
		if !ok {
			amap[s] = true
		} else {
			t.Error("出错了")
		}

		_ = azdg.Decrypt(s)
	}

	fmt.Println(time.Now().Sub(t1))
}
