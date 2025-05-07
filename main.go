package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/mitchellh/go-homedir"
	"gopkg.in/yaml.v3"
)

type Cookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
	Path   string `json:"path"`
}

type Config struct {
	Chrome struct {
		ProfileDir string `yaml:"profile_dir"`
	} `yaml:"chrome"`
	Server struct {
		IP   string `yaml:"ip"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
}

type RequestPayload struct {
	URL      string `json:"url"`
	Pattern  string `json:"pattern"`
	Headless bool   `json:"headless"`
}

var verbose bool

func main() {
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flag.Parse()

	config, err := loadConfig("config.yaml")
	if err != nil {
		log.Printf("Failed to load config, using defaults: %v", err)
	}

	serverIP := config.Server.IP
	if serverIP == "" {
		serverIP = "0.0.0.0"
	}
	serverPort := config.Server.Port
	if serverPort == 0 {
		serverPort = 8080
	}
	listenAddr := fmt.Sprintf("%s:%d", serverIP, serverPort)

	log.Printf("Starting server on %s", listenAddr)
	mux := http.NewServeMux()
	mux.HandleFunc("/fetch-cookies/", func(w http.ResponseWriter, r *http.Request) {
		handleFetchCookies(w, r, config)
	})
	if err := http.ListenAndServe(listenAddr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handleFetchCookies(w http.ResponseWriter, r *http.Request, config Config) {
	switch r.Method {
	case http.MethodGet:
		url := strings.TrimPrefix(r.URL.Path, "/fetch-cookies/")
		if url == "" {
			sendError(w, "Missing URL in path", http.StatusBadRequest)
			return
		}

		url = ensureHTTPS(url)
		if verbose {
			log.Printf("Processing URL: %s", url)
		}
		headless := r.URL.Query().Get("headless") != "false" && r.URL.Query().Get("headless") != "False"
		if verbose {
			log.Printf("Headless mode: %v", headless)
		}
		cookies, err := fetchCookies(url, "", headless, config)
		if err != nil {
			sendError(w, fmt.Sprintf("Failed to fetch cookies: %v", err), http.StatusInternalServerError)
			return
		}
		if verbose {
			log.Printf("Returning %d cookies for %s", len(cookies), url)
		}
		sendJSONResponse(w, cookies)

	case http.MethodPost:
		var payload RequestPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			sendError(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		if payload.URL == "" || payload.Pattern == "" {
			sendError(w, "URL and pattern are required", http.StatusBadRequest)
			return
		}

		url := ensureHTTPS(payload.URL)
		if verbose {
			log.Printf("Processing URL: %s", url)
		}
		if _, err := regexp.Compile(payload.Pattern); err != nil {
			sendError(w, fmt.Sprintf("Invalid regex pattern: %v", err), http.StatusBadRequest)
			return
		}

		cookies, err := fetchCookies(url, payload.Pattern, payload.Headless, config)
		if err != nil {
			sendError(w, fmt.Sprintf("Failed to fetch cookies: %v", err), http.StatusInternalServerError)
			return
		}
		if verbose {
			log.Printf("Returning %d cookies for %s", len(cookies), url)
		}
		sendJSONResponse(w, cookies)

	default:
		sendError(w, "Only GET and POST requests are supported", http.StatusMethodNotAllowed)
	}
}

func fetchCookies(url, pattern string, headless bool, config Config) ([]Cookie, error) {
	profileDir := config.Chrome.ProfileDir
	if profileDir == "" {
		profileDir = "~/AppData/Local/Google/Chrome/User Data/"
	}

	profile, err := homedir.Expand(profileDir)
	if err != nil {
		return nil, fmt.Errorf("failed to expand profile dir: %v", err)
	}
	if verbose {
		log.Printf("Using Chrome profile directory: %s", profile)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	browserCtx, cancel, err := setupChromeContext(ctx, profile, headless)
	if err != nil {
		return nil, fmt.Errorf("failed to setup Chrome context: %v", err)
	}
	defer cancel()

	var rawCookies []*network.Cookie
	actions := []chromedp.Action{
		chromedp.ActionFunc(func(ctx context.Context) error {
			if verbose {
				log.Printf("Navigating to %s", url)
			}
			return chromedp.Navigate(url).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if pattern != "" {
				if verbose {
					log.Printf("Waiting for URL to match pattern: %s", pattern)
				}
				if err := waitForURLPattern(ctx, pattern, 30*time.Second); err != nil {
					return fmt.Errorf("failed to wait for URL pattern: %v", err)
				}
			}
			return nil
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if verbose {
				log.Printf("Waiting for page body to load")
			}
			return chromedp.WaitVisible("body", chromedp.ByQuery).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if verbose {
				log.Printf("Waiting for network idle")
			}
			if err := waitForNetworkIdle(ctx, 2*time.Second, 30*time.Second); err != nil {
				return fmt.Errorf("failed to wait for network idle: %v", err)
			}
			return nil
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if verbose {
				log.Printf("Fetching cookies")
			}
			cookies, err := network.GetCookies().Do(ctx)
			if err != nil {
				return fmt.Errorf("failed to fetch cookies: %v", err)
			}
			rawCookies = cookies
			return nil
		}),
	}

	if err := chromedp.Run(browserCtx, actions...); err != nil {
		return nil, fmt.Errorf("failed to navigate or fetch cookies: %v", err)
	}

	var cookies []Cookie
	for _, c := range rawCookies {
		cookies = append(cookies, Cookie{
			Name:   c.Name,
			Value:  c.Value,
			Domain: c.Domain,
			Path:   c.Path,
		})
	}
	if verbose {
		log.Printf("Fetched %d cookies", len(cookies))
	}

	return cookies, nil
}

func setupChromeContext(parentCtx context.Context, profile string, headless bool) (context.Context, context.CancelFunc, error) {
	if verbose {
		log.Printf("Initializing Chrome with headless=%v", headless)
	}
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", headless),
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.UserDataDir(profile),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(parentCtx, opts...)
	browserCtx, browserCancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	return browserCtx, func() { browserCancel(); cancel() }, nil
}

func ensureHTTPS(url string) string {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		if verbose {
			log.Printf("Added https scheme: %s", "https://"+url)
		}
		return "https://" + url
	}
	return url
}

func sendError(w http.ResponseWriter, message string, statusCode int) {
	log.Printf("Error: %s (Status: %d)", message, statusCode)
	http.Error(w, message, statusCode)
}

func sendJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		sendError(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func waitForNetworkIdle(ctx context.Context, idleDuration, maxTimeout time.Duration) error {
	var mu sync.Mutex
	lastRequestTime := time.Now()

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if _, ok := ev.(*network.EventRequestWillBeSent); ok {
			mu.Lock()
			lastRequestTime = time.Now()
			mu.Unlock()
		}
	})

	if err := chromedp.Run(ctx, network.Enable()); err != nil {
		return fmt.Errorf("failed to enable network events: %v", err)
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(maxTimeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for network idle after %v", maxTimeout)
		case <-ticker.C:
			mu.Lock()
			idle := time.Since(lastRequestTime) >= idleDuration
			mu.Unlock()
			if idle {
				return nil
			}
		}
	}
}

func waitForURLPattern(ctx context.Context, pattern string, timeout time.Duration) error {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("failed to compile regex pattern: %v", err)
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeoutChan := time.After(timeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeoutChan:
			return fmt.Errorf("timeout waiting for URL to match pattern %s after %v", pattern, timeout)
		case <-ticker.C:
			var currentURL string
			err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
				currentIndex, entries, cmdErr := page.GetNavigationHistory().Do(ctx)
				if cmdErr != nil {
					return cmdErr
				}
				if len(entries) > 0 && currentIndex >= 0 && currentIndex < int64(len(entries)) {
					currentURL = entries[currentIndex].URL
				}
				return nil
			}))
			if err != nil {
				return fmt.Errorf("failed to get current URL: %v", err)
			}
			if regex.MatchString(currentURL) {
				if verbose {
					log.Printf("Current URL %s matches pattern %s", currentURL, pattern)
				}
				return nil
			}
		}
	}
}

func loadConfig(filename string) (Config, error) {
	var config Config
	data, err := os.ReadFile(filename)
	if err != nil {
		return config, fmt.Errorf("failed to read config file %s: %v", filename, err)
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("failed to parse config file %s: %v", filename, err)
	}
	return config, nil
}