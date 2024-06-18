package regx

import (
	"fmt"
	"regexp"
)

func VerifyPhoneNumber(phoneNumber string) error {
	if phoneNumber == "" {
		return fmt.Errorf("phone number cannot be empty")
	}

	matched, err := regexp.MatchString(`^1[3456789]\d{9}$`, phoneNumber)
	if err != nil {
		return err
	}

	if !matched {
		return fmt.Errorf("invalid phone number")
	}

	return nil
}
