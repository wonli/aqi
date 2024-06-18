package encrypt

import (
	"encoding/base64"
	"fmt"
	"testing"
	"time"
)

func TestAes(t *testing.T) {
	t1 := time.Now()
	for i := 0; i < 10000; i++ {
		s, _ := AesEncrypt([]byte("hello world 我是中文"), "hello")
		ss := base64.RawURLEncoding.EncodeToString(s)
		//fmt.Println(ss)

		ss1, _ := base64.RawURLEncoding.DecodeString(ss)
		_, err := AesDecrypt(ss1, "hello")
		if err != nil {
			t.Error(err.Error())
		}
	}

	fmt.Println(time.Now().Sub(t1))
}
