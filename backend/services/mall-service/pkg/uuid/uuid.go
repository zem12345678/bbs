package uuid

import (
	"github.com/pkg/errors"
	"os/exec"
	"strings"

	"github.com/google/uuid"
)

func NewUUID() (uuidStr string, err error) {
	newUuid, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	uuidStr = newUuid.String()
	return uuidStr, nil
}

func NewUpperUUID() (uuidStr string, err error) {
	uuidStr, err = NewUUID()
	if err != nil {
		return uuidStr, err
	}
	return strings.ToUpper(uuidStr), nil
}

func GetHostUuid() (uuidStr string, err error) {
	dmidecode := exec.Command("dmidecode", "-s", "system-uuid")
	bytes, err := dmidecode.Output()
	if err != nil {
		return "", errors.Wrap(err, "get uuid command error")
	}
	uuidStr = strings.Trim(string(bytes), "\n")
	return uuidStr, nil
}
