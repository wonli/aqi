package ws

import (
	"fmt"
	"hash/fnv"
	"log"
	"testing"
)

func TestHash(t *testing.T) {

	msgList := []string{
		"1",
		"2",
		"a",
		"aa",
		"123",
		"12",
		"hello",
		"hella",
		"用户注册失败",
		"用户注册失败1",
		"用户注册没失败",
		"注册失败",
		"失败",
		"用户失败",
	}

	for _, msg := range msgList {
		h := fnv.New32a()
		_, err := h.Write([]byte(msg))
		if err != nil {
			return
		}

		mHash := fmt.Sprintf("%03d", h.Sum32()%1000)
		mHash1 := fmt.Sprintf("%04d", h.Sum32()%10000)
		mHash2 := fmt.Sprintf("%03x", h.Sum32()%4096)
		log.Printf("%s -> (%s, %s, %d) -> （%s，%d) \n", msg, mHash, mHash1, h.Sum32(), mHash2, h.Sum32()%4096)
	}
}
