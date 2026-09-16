package helpers

import (
	"log"
)

func FailOnError(err error, msg string) {
	if err != nil {
		LogErrorMessage(err, msg)
	}
}

func LogErrorMessage(err error, msg string) {
	log.Panicf("%s: %s", msg, err)
}
