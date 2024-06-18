package regx

import (
	"strconv"
	"strings"
	"time"
)

func CheckID(idCard string, justCheckLength bool) bool {
	// verify length
	var lengthValidate bool
	if len(idCard) == 18 {
		lengthValidate = true
	} else if len(idCard) == 15 {
		lengthValidate = true
	} else {
		lengthValidate = false
	}

	if justCheckLength {
		return lengthValidate
	}

	if !lengthValidate {
		return false
	}

	cityCode := map[string]bool{
		"11": true, "12": true, "13": true, "14": true, "15": true,
		"21": true, "22": true, "23": true,
		"31": true, "32": true, "33": true, "34": true, "35": true, "36": true, "37": true,
		"41": true, "42": true, "43": true, "44": true, "45": true, "46": true,
		"50": true, "51": true, "52": true, "53": true, "54": true,
		"61": true, "62": true, "63": true, "64": true, "65": true,
		"71": true,
		"81": true, "82": true,
		"91": true,
	}

	// verify area
	if _, ok := cityCode[idCard[0:2]]; !ok {
		return false
	}

	factor := []int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
	verifyNumberList := []string{"1", "0", "X", "9", "8", "7", "6", "5", "4", "3", "2"}

	makeVerifyBit := func(idCard string) string {
		if len(idCard) != 17 {
			return ""
		}

		checksum := 0
		for i := 0; i < 17; i++ {
			b, _ := strconv.Atoi(string(idCard[i]))
			checksum += b * factor[i]
		}

		mod := checksum % 11
		return verifyNumberList[mod]
	}

	if len(idCard) == 15 {
		if idCard[12:15] == "996" || idCard[12:15] == "997" || idCard[12:15] == "998" || idCard[12:15] == "999" {
			idCard = idCard[0:6] + "18" + idCard[6:9]
		} else {
			idCard = idCard[0:6] + "19" + idCard[6:9]
		}

		idCard += makeVerifyBit(idCard)
	} else {
		if strings.ToUpper(idCard[17:]) != strings.ToUpper(makeVerifyBit(idCard[0:17])) {
			return false
		}
	}

	// verify birthday
	birthDay := idCard[6:14]
	d, err := time.Parse("20060102", birthDay)
	if err != nil || d.Year() > time.Now().Year() || int(d.Month()) > 12 || d.Day() > 31 {
		return false
	}

	return true
}
