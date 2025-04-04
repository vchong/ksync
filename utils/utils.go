package utils

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/KYVENetwork/ksync/logger"
	"github.com/KYVENetwork/ksync/metrics"
	"io"
	"math"
	"net/http"
	"os"
	"runtime"
	runtimeDebug "runtime/debug"
	"strconv"
	"strings"
	"time"
)

func GetVersion() string {
	version, ok := runtimeDebug.ReadBuildInfo()
	if !ok {
		panic("failed to get ksync version")
	}

	if version.Main.Version == "" {
		return "dev"
	}

	return strings.TrimSpace(version.Main.Version)
}

// GetFromUrlWithErr tries to fetch data from url with a custom User-Agent header
func GetFromUrlWithErr(url string) ([]byte, error) {
	// Log debug info
	logger.Logger.Debug().Str("url", url).Msg("GET")

	// Create a custom http.Client with the desired User-Agent header
	httpClient := &http.Client{Transport: http.DefaultTransport}

	// Create a new GET request
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Set the User-Agent header
	version := GetVersion()

	if version != "" {
		if strings.HasPrefix(version, "v") {
			version = strings.TrimPrefix(version, "v")
		}
		request.Header.Set("User-Agent", fmt.Sprintf("ksync/%s (%s / %s / %s)", version, runtime.GOOS, runtime.GOARCH, runtime.Version()))
	} else {
		request.Header.Set("User-Agent", fmt.Sprintf("ksync/dev (%s / %s / %s)", runtime.GOOS, runtime.GOARCH, runtime.Version()))
	}

	// Perform the request
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return nil, fmt.Errorf("got status code %d", response.StatusCode)
	}

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// GetFromUrl tries to fetch data from url with exponential backoff, we usually
// always want a request to succeed so it is implemented by default
func GetFromUrl(url string) (data []byte, err error) {
	for i := 0; i < BackoffMaxRetries; i++ {
		data, err = GetFromUrlWithErr(url)
		if err != nil {
			metrics.IncreaseFailedRequests()
			delaySec := math.Pow(2, float64(i))

			logger.Logger.Error().Msgf("failed to fetch from url \"%s\" with error \"%s\", retrying in %d seconds", url, err, int(delaySec))
			time.Sleep(time.Duration(delaySec) * time.Second)

			continue
		}

		metrics.IncreaseSuccessfulRequests()

		// only log success message if there were errors previously
		if i > 0 {
			logger.Logger.Info().Msgf("successfully fetched data from url %s", url)
		}
		return
	}

	logger.Logger.Error().Msgf("failed to fetch data from url within maximum retry limit of %d", BackoffMaxRetries)
	return
}

func CreateSha256Checksum(input []byte) (hash string) {
	h := sha256.New()
	h.Write(input)
	bs := h.Sum(nil)
	return fmt.Sprintf("%x", bs)
}

func DecompressGzip(input []byte) ([]byte, error) {
	var out bytes.Buffer
	r, err := gzip.NewReader(bytes.NewBuffer(input))
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(&out, r); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

func ParseSnapshotFromKey(key string) (height int64, chunkIndex int64, err error) {
	// if key is empty we are at height 0
	if key == "" {
		return
	}

	s := strings.Split(key, "/")

	if len(s) != 2 {
		return height, chunkIndex, fmt.Errorf("error parsing key %s", key)
	}

	height, err = strconv.ParseInt(s[0], 10, 64)
	if err != nil {
		return height, chunkIndex, fmt.Errorf("could not parse int from %s: %w", s[0], err)
	}

	chunkIndex, err = strconv.ParseInt(s[1], 10, 64)
	if err != nil {
		return height, chunkIndex, fmt.Errorf("could not parse int from %s: %w", s[1], err)
	}

	return
}

func IsUpgradeHeight(homePath string, height int64) bool {
	upgradeInfoPath := fmt.Sprintf("%s/data/upgrade-info.json", homePath)

	upgradeInfo, err := os.ReadFile(upgradeInfoPath)
	if err != nil {
		return false
	}

	var upgrade struct {
		Height int64 `json:"height"`
	}

	if err := json.Unmarshal(upgradeInfo, &upgrade); err != nil {
		return false
	}

	return upgrade.Height == height
}

func GetUserConfirmationInput() (bool, error) {
	startTime := time.Now()
	answer := ""

	if _, err := fmt.Scan(&answer); err != nil {
		return false, fmt.Errorf("failed to read in user input: %w", err)
	}

	metrics.SetUserConfirmationInput(answer)
	metrics.SetUserConfirmationDuration(time.Since(startTime))

	if strings.ToLower(answer) != "y" {
		logger.Logger.Info().Msg("abort")
		return false, nil
	}

	return true, nil
}
