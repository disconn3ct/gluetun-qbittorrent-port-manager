package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aunefyren/gluetun-qbittorrent-port-manager/modules"
	"github.com/fsnotify/fsnotify"
)

func baseURL() string {
	httpScheme := "http"
	if modules.ConfigFile.QBitTorrent.HTTPS {
		httpScheme = "https"
	}

	return fmt.Sprintf("%s://%s:%d", httpScheme, modules.ConfigFile.QBitTorrent.IP, modules.ConfigFile.QBitTorrent.Port)
}

// newClient creates an HTTP client with a cookie jar for session management
func newClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, Timeout: 10 * time.Second}
}

// login authenticates with qBittorrent and returns a session-bearing client
func login() (*http.Client, error) {
	client := newClient()
	// must use form data type data
	resp, err := client.PostForm(baseURL()+"/api/v2/auth/login", url.Values{
		"username": {modules.ConfigFile.QBitTorrent.Username},
		"password": {modules.ConfigFile.QBitTorrent.Password},
	})
	if err != nil {
		return nil, fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return nil, fmt.Errorf("login returned status %d", resp.StatusCode)
	}
	return client, nil
}

// readPortFromFile reads the port written by Gluetun
func readPortFromFile() (*int, error) {
	data, err := os.ReadFile(modules.ConfigFile.PortFile)
	if err != nil {
		return nil, fmt.Errorf("reading port file: %w", err)
	}
	cleanData := strings.TrimSpace(string(data))
	if cleanData == "" {
		modules.Log.Debug("empty file, no port found")
		return nil, nil
	}
	port, err := strconv.Atoi(cleanData)
	if err != nil {
		return nil, fmt.Errorf("parsing port: %w", err)
	}
	return &port, nil
}

// getCurrentPort fetches the listen port currently configured in qBittorrent
func getCurrentPort(client *http.Client) (int, error) {
	resp, err := client.Get(baseURL() + "/api/v2/app/preferences")
	if err != nil {
		return 0, fmt.Errorf("get preferences: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return 0, fmt.Errorf("not authenticated (403), re-login required")
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("get preferences returned status %d", resp.StatusCode)
	}

	var preferences modules.QBitTorrentAppPreferences
	if err := json.NewDecoder(resp.Body).Decode(&preferences); err != nil {
		return 0, fmt.Errorf("decoding preferences: %w", err)
	}

	return preferences.ListenPort, nil
}

// setPort updates qBittorrent's listen port
func setPort(client *http.Client, port int) error {
	payload := fmt.Sprintf(`{"listen_port": %d}`, port)
	resp, err := client.PostForm(baseURL()+"/api/v2/app/setPreferences", url.Values{"json": {payload}})
	if err != nil {
		return fmt.Errorf("setPreferences request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("setPreferences returned status %d", resp.StatusCode)
	}
	return nil
}

// syncPort performs full read-compare-update cycle
func syncPort(client *http.Client) {
	filePort, err := readPortFromFile()
	if err != nil {
		modules.Log.Warnf("%v", err)
		return
	}

	if filePort == nil {
		modules.Log.Warn("empty port file, not updating qBit")
		return
	}

	if client == nil {
		client, err = login()
		if err != nil {
			modules.Log.Errorf("login failed: %v", err)
			return
		}
	}

	currentPort, err := getCurrentPort(client)
	if err != nil {
		modules.Log.Errorf("could not read current qBit port: %v", err)
		return
	}

	if currentPort == *filePort {
		modules.Log.Debugf("qBittorrent already using port %d, no change needed", *filePort)
		return
	}

	modules.Log.Infof("port mismatch (qBit: %d, file: %d), updating...", currentPort, *filePort)
	if err := setPort(client, *filePort); err != nil {
		modules.Log.Errorf("failed to set port: %v", err)
		return
	}
	modules.Log.Infof("successfully updated qBittorrent listen port to %d", *filePort)
}

// watchFile triggers syncPort whenever the port file is written
func watchFile(path string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		modules.Log.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	if err := watcher.Add(path); err != nil {
		modules.Log.Fatalf("failed to watch %s: %v", path, err)
	}
	modules.Log.Infof("watching %s for changes", path)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				modules.Log.Infof("file change detected: %s", event.Name)
				syncPort(nil)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			modules.Log.Errorf("watcher error: %v", err)
		}
	}
}

func main() {
	// create files directory
	newPath := filepath.Join(".", "config")
	err := os.MkdirAll(newPath, os.ModePerm)
	if err != nil {
		fmt.Println("failed to create 'files' directory. error: " + err.Error())
		os.Exit(1)
	}
	fmt.Println("directory 'config' valid")

	// load config file
	err = modules.LoadConfig()
	if err != nil {
		fmt.Println("failed to load configuration file. error: " + err.Error())
		os.Exit(1)
	}
	fmt.Println("configuration file loaded")

	// create and define file for logging
	modules.InitLogger(modules.ConfigFile)

	modules.Log.Info("running gluetun-qbittorrent-port-manager version: " + modules.ConfigFile.Version)

	// Change the config to respect flags
	modules.ConfigFile = parseFlags(modules.ConfigFile)
	if err != nil {
		modules.Log.Fatal("failed to parse input flags. error: " + err.Error())
		os.Exit(1)
	}
	modules.Log.Info("flags parsed")

	err = modules.SaveConfig()
	if err != nil {
		modules.Log.Fatal("failed to save new config. error: " + err.Error())
		os.Exit(1)
	}

	// Set time zone from config if it is not empty
	if modules.ConfigFile.Timezone != "" {
		loc, err := time.LoadLocation(modules.ConfigFile.Timezone)
		if err != nil {
			modules.Log.Info("failed to set time zone from config. error: " + err.Error())
			modules.Log.Info("removing value...")

			modules.ConfigFile.Timezone = ""
			err = modules.SaveConfig()
			if err != nil {
				modules.Log.Fatal("failed to set new time zone in the config. error: " + err.Error())
				os.Exit(1)
			}

		} else {
			time.Local = loc
		}
	}
	modules.Log.Info("timezone set")

	// wait for the port file to appear before proceeding
	for {
		if _, err := os.Stat(modules.ConfigFile.PortFile); err == nil {
			break
		}
		modules.Log.Warnf("port file %s not found, retrying in 10s", modules.ConfigFile.PortFile)
		time.Sleep(10 * time.Second)
	}

	// wait for qBitTorrent to connect before proceeding
	var client *http.Client = nil
	if modules.ConfigFile.WaitForQBitTorrent {
		modules.Log.Info("trying to log into qBitTorrent to check it is available")
		for {
			client, err = login()
			if err == nil {
				modules.Log.Info("qBitTorrent is now available")
				break
			}
			modules.Log.Warnf("qBitTorrent login failed, retrying in 10s. error: %s", err.Error())
			time.Sleep(10 * time.Second)
		}
	}

	// initial sync on startup
	syncPort(client)

	// file watcher, reacts immediately to Gluetun writing a new port
	go watchFile(modules.ConfigFile.PortFile)

	// periodic verification ticker, catches any drift not caught by the watcher
	ticker := time.NewTicker(time.Duration(modules.ConfigFile.Interval) * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		modules.Log.Debug("periodic check triggered")
		syncPort(nil)
	}
}

func parseFlags(configFile modules.ConfigStruct) modules.ConfigStruct {
	// default value for flag
	defaultBoolStringHTTPS := "true"
	if !configFile.QBitTorrent.HTTPS {
		defaultBoolStringHTTPS = "false"
	}
	defaultBoolStringWait := "true"
	if !configFile.WaitForQBitTorrent {
		defaultBoolStringWait = "false"
	}

	// define flag variables with the configuration file as default values
	var https = flag.String("https", defaultBoolStringHTTPS, "The protocol qBitTorrent is listening on.")
	var port = flag.Int("port", configFile.QBitTorrent.Port, "The port qBitTorrent is listening on.")
	var ip = flag.String("ip", configFile.QBitTorrent.IP, "The IP qBitTorrent is listening on.")
	var username = flag.String("username", configFile.QBitTorrent.Username, "The username used for logging into qBitTorrent.")
	var password = flag.String("password", configFile.QBitTorrent.Password, "The password used for logging into qBitTorrent.")

	var timezone = flag.String("tz", configFile.Timezone, "The timezone the manager is running in.")
	var environment = flag.String("environment", configFile.Environment, "The environment the manager is running in.")
	var interval = flag.Int("interval", configFile.Interval, "The interval (minutes) between when the manager performs a check.")
	var waitOnqBit = flag.String("waitforqbit ", defaultBoolStringWait, "Wait for qBitTorrent to start before the manager starts working.")
	var portFile = flag.String("portfile", configFile.PortFile, "The port file the manager monitors.")
	var loglevel = flag.String("loglevel", configFile.LogLevel, "The log amount the manager prints.")

	// parse the flags from input
	flag.Parse()
	trueString := "true"
	falseString := "false"

	// respect the flag if config is empty
	if https != nil && strings.EqualFold(*https, trueString) {
		configFile.QBitTorrent.HTTPS = true
	} else if https != nil && strings.EqualFold(*https, falseString) {
		configFile.QBitTorrent.HTTPS = false
	}
	if port != nil {
		configFile.QBitTorrent.Port = *port
	}
	if ip != nil {
		configFile.QBitTorrent.IP = *ip
	}
	if username != nil {
		configFile.QBitTorrent.Username = *username
	}
	if password != nil {
		configFile.QBitTorrent.Password = *password
	}

	// respect the flag if config is empty
	if timezone != nil {
		configFile.Timezone = *timezone
	}
	if environment != nil {
		configFile.Environment = *environment
	}
	if interval != nil {
		configFile.Interval = *interval
	}
	if waitOnqBit != nil && strings.EqualFold(*waitOnqBit, trueString) {
		configFile.WaitForQBitTorrent = true
	} else if waitOnqBit != nil && strings.EqualFold(*waitOnqBit, falseString) {
		configFile.WaitForQBitTorrent = false
	}
	if portFile != nil {
		configFile.PortFile = *portFile
	}
	if loglevel != nil {
		configFile.LogLevel = *loglevel
	}

	return configFile
}
