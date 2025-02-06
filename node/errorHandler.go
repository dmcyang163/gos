package main

import (
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func HandleError(logger *logrus.Logger, err error, message string) {
	if err != nil {
		wrappedErr := errors.Wrap(err, message)
		logger.Errorf("Error: %v", wrappedErr)
	}
}
