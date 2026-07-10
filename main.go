package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"time"

	"github.com/disconn3ct/gluetun-qbittorrent-port-manager/modules"
)

var Config = &modules.Config

func baseURL() string {
	httpScheme := "http"
	if Config.QBitTorrent.HTTPS {
		httpScheme = "https"
	}

	return fmt.Sprintf("%s://%s:%d", httpScheme, Config.QBitTorrent.IP, Config.QBitTorrent.Port)
}

func gluetunBaseURL() string {
	return fmt.Sprintf("http://%s:%d", Config.Gluetun.IP, Config.Gluetun.Port)
}

// newClient creates an HTTP client with a cookie jar for session management
func newClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, Timeout: time.Duration(Config.Timeout) * time.Second}
}

// login authenticates with qBittorrent and returns a session-bearing client
func login() (*http.Client, error) {
	client := newClient()
	// must use form data type data
	resp, err := client.PostForm(baseURL()+"/api/v2/auth/login", url.Values{
		"username": {Config.QBitTorrent.Username},
		"password": {Config.QBitTorrent.Password},
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

// readPortFromApi reads the port from the Gluetun API
func readPortFromApi() (int, error) {
	glueClient := newClient()
	url := gluetunBaseURL() + "/v1/portforward"
	modules.Log.Debugf("Fetching %s", url)
	resp, err := glueClient.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return 0, fmt.Errorf("Gluetun not authenticated (403). Authentication is not currently supported")
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("get portforward returned status %d", resp.StatusCode)
	}

	var portforward modules.GluetunPortForward

	if err := json.NewDecoder(resp.Body).Decode(&portforward); err != nil {
		return 0, fmt.Errorf("decoding portforward: %w", err)
	}
	return portforward.Port, nil
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
	gluetunPort, err := readPortFromApi()
	if err != nil {
		modules.Log.Errorf("readPortFromApi: %s",err)
		return
	}
	modules.Log.Debugf("Gluetun Port: %d",gluetunPort)
	if gluetunPort == 0 {
		modules.Log.Warn("no port found, not updating qBit")
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

	if currentPort == gluetunPort {
		modules.Log.Debugf("qBittorrent already using port %d, no change needed", gluetunPort)
		return
	}

	modules.Log.Infof("port mismatch (qBit: %d, file: %d), updating...", currentPort, gluetunPort)
	if err := setPort(client, gluetunPort); err != nil {
		modules.Log.Errorf("failed to set port: %v", err)
		return
	}
	modules.Log.Infof("successfully updated qBittorrent listen port to %d", gluetunPort)
}

func main() {
	modules.InitLogger()

	// Output to stdout
	modules.Log.Infof("running gluetun-qbittorrent-port-manager version: %s", Config.Version)

	// load config file
	err := modules.LoadConfig()
	if err != nil {
		modules.Log.Errorf("failed to load configuration. error: %s", err.Error())
		os.Exit(1)
	}
	modules.Log.Info("environment loaded")

	// Set time zone from config if it is not empty
	if Config.Timezone != "" {
		loc, err := time.LoadLocation(Config.Timezone)
		if err != nil {
			modules.Log.Errorf("failed to set time zone from config. error: %s", err.Error())
			os.Exit(1)
		} else {
			time.Local = loc
		}
	}
	modules.Log.Info("timezone set")

	// wait for the port before proceeding
	for {
		if _, err := readPortFromApi(); err != nil {
			modules.Log.Warnf("Failed to get port: %s", err.Error())
		 	modules.Log.Warn("Retrying in 10s")
			time.Sleep(10 * time.Second)
		} else {
			break
		}
	}

	// wait for qBitTorrent to connect before proceeding
	var client *http.Client = nil
	if Config.WaitForQBitTorrent {
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

	// periodic verification ticker, catches any drift not caught by the watcher
	ticker := time.NewTicker(time.Duration(Config.Interval) * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		modules.Log.Debug("periodic check triggered")
		syncPort(nil)
	}
}
