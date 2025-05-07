# CookieAPI

# CookieAPI
 is a Go-based HTTP server that uses ChromeDP to fetch cookies from a specified URL. It supports both GET and POST requests, allowing users to retrieve cookies either directly from a URL or by waiting for a URL pattern match. The tool is useful for web scraping, testing, or debugging scenarios where cookie data is needed.

## Features

- Fetch cookies from any URL via a simple HTTP API.
- Supports headless and non-headless Chrome modes.
- Configurable Chrome profile directory for persistent sessions.
- Optional URL pattern matching for POST requests.
- Verbose logging mode for detailed debugging.
- Configurable server IP and port via YAML.

## Prerequisites

- Go 1.16 or higher
- Google Chrome or Chromium browser installed
- Git (optional, for cloning the repository)

## Installation

1. **Clone the repository** (or download the source):
   ```bash
   git clone https://github.com/yourusername/cookieapi.git
   cd cookieapi
   ```

2. **Install dependencies**:
   ```bash
   go mod download
   ```

3. **Build the application**:
   ```bash
   go build -o cookieapi main.go
   ```

## Configuration

Create a `config.yaml` file in the project root to customize settings. Example:

```yaml
chrome:
  profile_dir: "~/AppData/Local/Google/Chrome/User Data/"
server:
  ip: "0.0.0.0"
  port: 8080
```

- `chrome.profile_dir`: Path to the Chrome user data directory (default: `~/AppData/Local/Google/Chrome/User Data/`).
- `server.ip`: IP address to bind the server (default: `0.0.0.0`).
- `server.port`: Port to run the server (default: `8080`).

## Usage

1. **Run the server**:
   ```bash
   ./cookieapi
   ```
   For verbose logging:
   ```bash
   ./cookieapi --verbose
   ```

2. **Make requests**:

   - **GET request** to fetch cookies from a URL:
     ```bash
     curl http://localhost:8080/fetch-cookies/example.com
     ```
     Optional: Disable headless mode by adding `?headless=false`:
     ```bash
     curl http://localhost:8080/fetch-cookies/example.com?headless=false
     ```

   - **POST request** to fetch cookies after matching a URL pattern:
     ```bash
     curl -X POST http://localhost:8080/fetch-cookies/ \
       -H "Content-Type: application/json" \
       -d '{"url":"https://example.com","pattern":".*login.*","headless":true}'
     ```

3. **Example response**:
   ```json
   [
       {
           "name": "session_id",
           "value": "abc123",
           "domain": ".example.com",
           "path": "/"
       },
       {
           "name": "user_token",
           "value": "xyz789",
           "domain": ".example.com",
           "path": "/"
       }
   ]
   ```

## API Endpoints

- **GET `/fetch-cookies/<url>`**
  - Fetches cookies from the specified URL.
  - Query parameters:
    - `headless`: Set to `false` to run Chrome in non-headless mode (default: `true`).
  - Example: `/fetch-cookies/example.com?headless=false`

- **POST `/fetch-cookies/`**
  - Fetches cookies after navigating to a URL and waiting for a URL matching the provided regex pattern.
  - Body (JSON):
    - `url`: Target URL (required).
    - `pattern`: Regex pattern to match the URL (required).
    - `headless`: Run Chrome in headless mode (default: `true`).
  - Example payload:
    ```json
    {
        "url": "https://example.com",
        "pattern": ".*login.*",
        "headless": true
    }
    ```

## Running Tests

To run the unit tests (if applicable):
```bash
go test ./...
```

## Contributing

Contributions are welcome! Please follow these steps:

1. Fork the repository.
2. Create a new branch (`git checkout -b feature/your-feature`).
3. Commit your changes (`git commit -m "Add your feature"`).
4. Push to the branch (`git push origin feature/your-feature`).
5. Open a Pull Request.

## License

This project is licensed under the GNU GENERAL PUBLIC LICENSE.

## Acknowledgments

- Built with [ChromeDP](https://github.com/chromedp/chromedp) for browser automation.
- Uses [Go-YAML](https://github.com/go-yaml/yaml) for configuration parsing.
- Inspired by the need for reliable cookie extraction in web scraping tasks.

## Contact

For issues or questions, please open an issue on GitHub or contact [your email or preferred contact method].

---

Happy fetching! 🍪
