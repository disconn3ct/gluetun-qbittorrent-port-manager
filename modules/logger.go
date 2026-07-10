package modules

import (
	"os"

	"github.com/sirupsen/logrus"
)

var Log *logrus.Logger

func InitLogger() {
	Log = logrus.New()

	// Set a plain text format with old-style timestamp
	Log.SetFormatter(&logrus.TextFormatter{})
	Log.SetOutput(os.Stdout)
}
