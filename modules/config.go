package modules

import (
	"os"
	"strconv"

	"github.com/sirupsen/logrus"
)

var versionParameter = "{{RELEASE_TAG}}"
var ConfigFile = ConfigStruct{}

func getEnv(key, fallback string) string {
    if value, ok := os.LookupEnv(key); ok {
        return value
    }
    return fallback
}

var Config = ConfigStruct{}

func LoadConfig() (err error) {
	Config.LogLevel = getEnv("LOGLEVEL","info")
	// Set log level
	level, err := logrus.ParseLevel(Config.LogLevel)
	if err != nil {
		Log.Errorf("failed to init logging: %v", err)
		return err
  }
	Log.SetLevel(level)
	Log.Info("log level set to: " + level.String())

	Config.Timezone = getEnv("TZ","Europe/Paris")
	Log.Debugf("Timezone: %s",Config.Timezone)

	Config.Environment = getEnv("ENVIRONMENT","production")
	Log.Debugf("Environment: %s",Config.Environment)

	Config.Interval, err = strconv.Atoi(getEnv("INTERVAL","15"))
	if err != nil {
		Log.Errorf("failed to set interval: %v", err)
		return err
	}
	Log.Debugf("Interval: %d",Config.Interval)

	Config.Timeout, err = strconv.Atoi(getEnv("TIMEOUT","15"))
	if err != nil {
		Log.Errorf("failed to set timeout: %v", err)
		return err
	}
	Log.Debugf("Timeout (seconds): %d",Config.Timeout)

	Config.WaitForQBitTorrent, err = strconv.ParseBool(getEnv("WAITFORQBIT",strconv.FormatBool(Config.WaitForQBitTorrent)))
	if err != nil {
		Log.Errorf("failed to set WaitForQbit: %v", err)
		return err
	}
	Log.Debugf("WaitForQbit: %s",strconv.FormatBool(Config.WaitForQBitTorrent))

	Config.Gluetun.IP = getEnv("GLUETUNIP","localhost")
	Log.Debugf("Gluetun IP: %s",Config.Gluetun.IP)

	Config.Gluetun.Port, err = strconv.Atoi(getEnv("GLUETUNPORT","8000"))
	if err != nil {
		Log.Errorf("failed to set Gluetun Port: %v", err)
		return err
	}
	Log.Debugf("Gluetun Port: %d",Config.Gluetun.Port)

	Config.QBitTorrent.IP = getEnv("IP","localhost")
	Log.Debugf("QbitTorrent IP: %s",Config.QBitTorrent.IP)

	Config.QBitTorrent.Port, err = strconv.Atoi(getEnv("PORT","8080"))
	if err != nil {
		Log.Errorf("failed to set QbitTorrent Port: %v", err)
		return err
	}
	Log.Debugf("QbitTorrent Port: %d",Config.QBitTorrent.Port)

	Config.QBitTorrent.Username = getEnv("USERNAME","admin")
	Log.Debugf("QbitTorrent Username: %s",Config.QBitTorrent.Username)

	Config.QBitTorrent.Password = getEnv("PASSWORD","")
	if Config.QBitTorrent.Password != "" {
			Log.Debug("QbitTorrent Password: Present")
	}

	// return nil object
	return nil
}
