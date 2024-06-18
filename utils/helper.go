package utils

func HidePhoneNumber(phoneNumber string) string {
	if len(phoneNumber) != 11 {
		return phoneNumber
	}
	return phoneNumber[:3] + "****" + phoneNumber[7:]
}
