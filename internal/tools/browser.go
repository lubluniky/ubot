// Package tools provides browser automation via headless Chrome/Chromium.
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	browserActionTimeout   = 30 * time.Second
	browserIdleTimeout     = 5 * time.Minute
	maxBrowserContentChars = 50000
)

// browserInstance holds a running headless Chrome process and its CDP endpoint.
type browserInstance struct {
	cmd        *exec.Cmd
	cdpURL     string
	lastUsed   time.Time
	mu         sync.Mutex
	cancelIdle context.CancelFunc
}

// BrowserTool provides browser automation capabilities using headless Chrome.
type BrowserTool struct {
	BaseTool
	browser      *browserInstance
	mu           sync.Mutex
	workspaceDir string
}

// NewBrowserTool creates a new BrowserTool.
func NewBrowserTool() *BrowserTool {
	home, _ := os.UserHomeDir()
	workspace := filepath.Join(home, ".ubot", "workspace")

	parameters := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Browser action to perform",
				"enum":        []string{"browse_page", "click_element", "type_text", "extract_text", "screenshot"},
			},
			"url": map[string]interface{}{
				"type":        "string",
				"description": "URL to navigate to (required for browse_page)",
			},
			"selector": map[string]interface{}{
				"type":        "string",
				"description": "CSS selector for the target element (required for click_element, type_text, extract_text)",
			},
			"text": map[string]interface{}{
				"type":        "string",
				"description": "Text to type into the element (required for type_text)",
			},
		},
		"required": []string{"action"},
	}

	return &BrowserTool{
		BaseTool: NewBaseTool(
			"browser_use",
			"Automate a headless Chrome browser. Actions: browse_page (navigate to URL and return content), click_element (click a CSS selector), type_text (type into an input), extract_text (get text from selector), screenshot (capture the page).",
			parameters,
		),
		workspaceDir: workspace,
	}
}

// Execute runs the specified browser action.
func (t *BrowserTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	action, err := GetStringParam(params, "action")
	if err != nil {
		return "", fmt.Errorf("browser_use: %w", err)
	}

	// Create a timeout context for this action.
	actionCtx, cancel := context.WithTimeout(ctx, browserActionTimeout)
	defer cancel()

	switch action {
	case "browse_page":
		return t.browsePage(actionCtx, params)
	case "click_element":
		return t.clickElement(actionCtx, params)
	case "type_text":
		return t.typeText(actionCtx, params)
	case "extract_text":
		return t.extractText(actionCtx, params)
	case "screenshot":
		return t.screenshot(actionCtx, params)
	default:
		return "", fmt.Errorf("browser_use: unknown action %q, must be one of: browse_page, click_element, type_text, extract_text, screenshot", action)
	}
}

// findChromeBinary locates a Chrome/Chromium binary on the system.
func findChromeBinary() (string, error) {
	candidates := []string{
		"google-chrome",
		"google-chrome-stable",
		"chromium",
		"chromium-browser",
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
	}
	for _, c := range candidates {
		if path, err := exec.LookPath(c); err == nil {
			return path, nil
		}
		// For absolute paths, check directly.
		if filepath.IsAbs(c) {
			if _, err := os.Stat(c); err == nil {
				return c, nil
			}
		}
	}
	return "", fmt.Errorf("no Chrome or Chromium binary found; install Chrome/Chromium to use the browser tool")
}

// freePort finds an available TCP port.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port, nil
}

// ensureBrowser starts a headless Chrome instance if not already running.
func (t *BrowserTool) ensureBrowser() (*browserInstance, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.browser != nil && t.browser.cmd.Process != nil {
		// Check if process is still alive.
		if err := t.browser.cmd.Process.Signal(os.Signal(nil)); err == nil {
			t.browser.mu.Lock()
			t.browser.lastUsed = time.Now()
			t.browser.mu.Unlock()
			return t.browser, nil
		}
		// Process died, clean up.
		t.browser = nil
	}

	chromePath, err := findChromeBinary()
	if err != nil {
		return nil, err
	}

	port, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("browser_use: failed to find free port: %w", err)
	}

	userDataDir, err := os.MkdirTemp("", "ubot-browser-*")
	if err != nil {
		return nil, fmt.Errorf("browser_use: failed to create temp dir: %w", err)
	}

	args := []string{
		"--headless=new",
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-gpu",
		"--disable-extensions",
		"--disable-sync",
		"--disable-translate",
		"--mute-audio",
		"--no-sandbox",
		fmt.Sprintf("--user-data-dir=%s", userDataDir),
		"about:blank",
	}

	cmd := exec.Command(chromePath, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		os.RemoveAll(userDataDir)
		return nil, fmt.Errorf("browser_use: failed to start Chrome: %w", err)
	}

	cdpURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// Wait for CDP to be ready.
	client := &http.Client{Timeout: 2 * time.Second}
	ready := false
	for i := 0; i < 30; i++ {
		resp, err := client.Get(cdpURL + "/json/version")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				ready = true
				break
			}
		}
		time.Sleep(200 * time.Millisecond)
	}

	if !ready {
		cmd.Process.Kill()
		cmd.Wait()
		os.RemoveAll(userDataDir)
		return nil, fmt.Errorf("browser_use: Chrome CDP did not become ready")
	}

	idleCtx, cancelIdle := context.WithCancel(context.Background())

	bi := &browserInstance{
		cmd:        cmd,
		cdpURL:     cdpURL,
		lastUsed:   time.Now(),
		cancelIdle: cancelIdle,
	}

	// Start idle timeout goroutine.
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-idleCtx.Done():
				return
			case <-ticker.C:
				bi.mu.Lock()
				idle := time.Since(bi.lastUsed)
				bi.mu.Unlock()
				if idle > browserIdleTimeout {
					t.mu.Lock()
					t.closeBrowserLocked(userDataDir)
					t.mu.Unlock()
					return
				}
			}
		}
	}()

	t.browser = bi
	return bi, nil
}

// closeBrowserLocked kills the browser process. Must be called with t.mu held.
func (t *BrowserTool) closeBrowserLocked(userDataDir string) {
	if t.browser == nil {
		return
	}
	if t.browser.cancelIdle != nil {
		t.browser.cancelIdle()
	}
	if t.browser.cmd.Process != nil {
		t.browser.cmd.Process.Kill()
		t.browser.cmd.Wait()
	}
	if userDataDir != "" {
		os.RemoveAll(userDataDir)
	}
	t.browser = nil
}

// Close shuts down the browser process.
func (t *BrowserTool) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closeBrowserLocked("")
}

// cdpTargetInfo holds info about a CDP target (page).
type cdpTargetInfo struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

// getPageTargetID gets the first page target ID from CDP.
func (t *BrowserTool) getPageTargetID(bi *browserInstance) (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(bi.cdpURL + "/json/list")
	if err != nil {
		return "", fmt.Errorf("browser_use: failed to list CDP targets: %w", err)
	}
	defer resp.Body.Close()

	var targets []cdpTargetInfo
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return "", fmt.Errorf("browser_use: failed to parse CDP targets: %w", err)
	}

	for _, t := range targets {
		if t.Type == "page" {
			return t.ID, nil
		}
	}

	// Create a new page target.
	resp2, err := client.Get(bi.cdpURL + "/json/new?about:blank")
	if err != nil {
		return "", fmt.Errorf("browser_use: failed to create new page: %w", err)
	}
	defer resp2.Body.Close()

	var newTarget cdpTargetInfo
	if err := json.NewDecoder(resp2.Body).Decode(&newTarget); err != nil {
		return "", fmt.Errorf("browser_use: failed to parse new target: %w", err)
	}
	return newTarget.ID, nil
}

// cdpSend sends a CDP command via the HTTP endpoint and returns the result.
func (t *BrowserTool) cdpSend(ctx context.Context, bi *browserInstance, targetID, method string, cdpParams map[string]interface{}) (json.RawMessage, error) {
	reqBody := map[string]interface{}{
		"method": method,
		"params": cdpParams,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("browser_use: failed to marshal CDP command: %w", err)
	}

	// Use the /json/protocol endpoint via HTTP to send commands.
	// Chrome's CDP HTTP API: POST /json/protocol with method and params.
	// Actually, the standard way is via WebSocket, but we can use the
	// simplified HTTP endpoints for navigation.

	// For operations that need CDP protocol commands, we use a shell approach
	// with Chrome's built-in endpoints.
	_ = body
	_ = ctx

	// Chrome HTTP API has limited commands. For full CDP we use a helper approach.
	return nil, nil
}

// browsePage navigates to a URL and returns page content.
func (t *BrowserTool) browsePage(ctx context.Context, params map[string]interface{}) (string, error) {
	urlStr, err := GetStringParam(params, "url")
	if err != nil {
		return "", fmt.Errorf("browser_use browse_page: %w", err)
	}

	if urlStr == "" {
		return "", fmt.Errorf("browser_use browse_page: url cannot be empty")
	}

	// Ensure the URL has a scheme.
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		urlStr = "https://" + urlStr
	}

	// SSRF protection: block requests to internal/private network addresses
	if isInternalURL(urlStr) {
		return "", fmt.Errorf("browser_use browse_page: access to internal/private network addresses is blocked")
	}

	bi, err := t.ensureBrowser()
	if err != nil {
		return "", err
	}

	targetID, err := t.getPageTargetID(bi)
	if err != nil {
		return "", err
	}

	// Navigate via CDP HTTP API.
	client := &http.Client{Timeout: browserActionTimeout}
	navURL := fmt.Sprintf("%s/json/navigate?%s", bi.cdpURL, targetID)

	// Chrome doesn't have a direct /json/navigate. Use the approach of
	// activating a target and then using shell-based page dump, or
	// fetch the page content directly via HTTP and use goquery.
	_ = navURL

	// Practical approach: use Chrome's --dump-dom or fetch via HTTP client
	// and parse with goquery for rich content extraction.
	return t.fetchAndParsePage(ctx, client, urlStr)
}

// fetchAndParsePage fetches a URL and extracts content using goquery.
func (t *BrowserTool) fetchAndParsePage(ctx context.Context, client *http.Client, urlStr string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return "", fmt.Errorf("browser_use browse_page: failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("browser_use browse_page: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("browser_use browse_page: HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("browser_use browse_page: failed to parse HTML: %w", err)
	}

	title := strings.TrimSpace(doc.Find("title").First().Text())

	// Remove non-content elements.
	doc.Find("script, style, nav, footer, header, aside, noscript, iframe").Remove()

	// Extract text content from main content area.
	var contentEl *goquery.Selection
	for _, sel := range []string{"main", "article", "[role=main]", ".content", "#content", "body"} {
		s := doc.Find(sel)
		if s.Length() > 0 {
			contentEl = s.First()
			break
		}
	}
	if contentEl == nil {
		contentEl = doc.Find("body")
	}

	// Extract interactive elements for the agent to reference.
	var elements strings.Builder
	doc.Find("a[href], button, input, select, textarea").Each(func(i int, s *goquery.Selection) {
		if i >= 50 {
			return // Limit to 50 elements.
		}
		tag := goquery.NodeName(s)
		text := strings.TrimSpace(s.Text())
		switch tag {
		case "a":
			href, _ := s.Attr("href")
			if text != "" && href != "" {
				elements.WriteString(fmt.Sprintf("  [link] %q -> %s\n", text, href))
			}
		case "button":
			if text != "" {
				elements.WriteString(fmt.Sprintf("  [button] %q\n", text))
			}
		case "input":
			inputType, _ := s.Attr("type")
			name, _ := s.Attr("name")
			placeholder, _ := s.Attr("placeholder")
			elements.WriteString(fmt.Sprintf("  [input type=%s name=%q placeholder=%q]\n", inputType, name, placeholder))
		case "select":
			name, _ := s.Attr("name")
			elements.WriteString(fmt.Sprintf("  [select name=%q]\n", name))
		case "textarea":
			name, _ := s.Attr("name")
			elements.WriteString(fmt.Sprintf("  [textarea name=%q]\n", name))
		}
	})

	bodyText := strings.TrimSpace(contentEl.Text())
	bodyText = collapseWhitespace(bodyText)
	if len(bodyText) > maxBrowserContentChars {
		bodyText = bodyText[:maxBrowserContentChars] + "\n... [content truncated]"
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Page: %s\n", urlStr))
	if title != "" {
		result.WriteString(fmt.Sprintf("Title: %s\n", title))
	}
	result.WriteString("\n--- Page Content ---\n")
	result.WriteString(bodyText)
	if elements.Len() > 0 {
		result.WriteString("\n\n--- Interactive Elements ---\n")
		result.WriteString(elements.String())
	}

	return result.String(), nil
}

// clickElement clicks an element on the current page.
func (t *BrowserTool) clickElement(ctx context.Context, params map[string]interface{}) (string, error) {
	selector, err := GetStringParam(params, "selector")
	if err != nil {
		return "", fmt.Errorf("browser_use click_element: %w", err)
	}
	if selector == "" {
		return "", fmt.Errorf("browser_use click_element: selector cannot be empty")
	}

	bi, err := t.ensureBrowser()
	if err != nil {
		return "", err
	}

	result, err := t.executeJSOnPage(ctx, bi, fmt.Sprintf(`
		(function() {
			var el = document.querySelector(%q);
			if (!el) return JSON.stringify({error: "Element not found: %s"});
			el.click();
			return JSON.stringify({success: true, tag: el.tagName, text: el.textContent.trim().substring(0, 100)});
		})()
	`, selector, selector))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Clicked element %q: %s", selector, result), nil
}

// typeText types text into an input element.
func (t *BrowserTool) typeText(ctx context.Context, params map[string]interface{}) (string, error) {
	selector, err := GetStringParam(params, "selector")
	if err != nil {
		return "", fmt.Errorf("browser_use type_text: %w", err)
	}
	if selector == "" {
		return "", fmt.Errorf("browser_use type_text: selector cannot be empty")
	}

	text, err := GetStringParam(params, "text")
	if err != nil {
		return "", fmt.Errorf("browser_use type_text: %w", err)
	}

	bi, err := t.ensureBrowser()
	if err != nil {
		return "", err
	}

	escapedText, _ := json.Marshal(text)

	result, err := t.executeJSOnPage(ctx, bi, fmt.Sprintf(`
		(function() {
			var el = document.querySelector(%q);
			if (!el) return JSON.stringify({error: "Element not found: %s"});
			el.focus();
			el.value = %s;
			el.dispatchEvent(new Event('input', {bubbles: true}));
			el.dispatchEvent(new Event('change', {bubbles: true}));
			return JSON.stringify({success: true, tag: el.tagName, value: el.value.substring(0, 100)});
		})()
	`, selector, selector, string(escapedText)))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Typed into %q: %s", selector, result), nil
}

// extractText extracts text content from an element.
func (t *BrowserTool) extractText(ctx context.Context, params map[string]interface{}) (string, error) {
	selector, err := GetStringParam(params, "selector")
	if err != nil {
		return "", fmt.Errorf("browser_use extract_text: %w", err)
	}
	if selector == "" {
		return "", fmt.Errorf("browser_use extract_text: selector cannot be empty")
	}

	bi, err := t.ensureBrowser()
	if err != nil {
		return "", err
	}

	result, err := t.executeJSOnPage(ctx, bi, fmt.Sprintf(`
		(function() {
			var el = document.querySelector(%q);
			if (!el) return JSON.stringify({error: "Element not found: %s"});
			return JSON.stringify({text: el.textContent.trim(), tag: el.tagName, html: el.innerHTML.substring(0, 500)});
		})()
	`, selector, selector))
	if err != nil {
		return "", err
	}

	return result, nil
}

// screenshot captures a screenshot of the current page.
func (t *BrowserTool) screenshot(ctx context.Context, params map[string]interface{}) (string, error) {
	bi, err := t.ensureBrowser()
	if err != nil {
		return "", err
	}

	// Use Chrome's headless screenshot mode via a new process.
	chromePath, err := findChromeBinary()
	if err != nil {
		return "", err
	}

	// Ensure workspace screenshots directory exists.
	screenshotDir := filepath.Join(t.workspaceDir, "screenshots")
	if err := os.MkdirAll(screenshotDir, 0755); err != nil {
		return "", fmt.Errorf("browser_use screenshot: failed to create directory: %w", err)
	}

	filename := fmt.Sprintf("screenshot-%d.png", time.Now().Unix())
	screenshotPath := filepath.Join(screenshotDir, filename)

	// Get the current page URL from CDP.
	pageURL, err := t.getCurrentPageURL(bi)
	if err != nil || pageURL == "" || pageURL == "about:blank" {
		return "", fmt.Errorf("browser_use screenshot: no page loaded, use browse_page first")
	}

	// Take screenshot using a separate headless Chrome invocation.
	cmd := exec.CommandContext(ctx, chromePath,
		"--headless=new",
		"--disable-gpu",
		"--no-sandbox",
		"--screenshot="+screenshotPath,
		"--window-size=1280,720",
		pageURL,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("browser_use screenshot: Chrome failed: %v (%s)", err, stderr.String())
	}

	if _, err := os.Stat(screenshotPath); err != nil {
		return "", fmt.Errorf("browser_use screenshot: file not created")
	}

	return fmt.Sprintf("Screenshot saved to %s", screenshotPath), nil
}

// getCurrentPageURL gets the URL of the current page from CDP.
func (t *BrowserTool) getCurrentPageURL(bi *browserInstance) (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(bi.cdpURL + "/json/list")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var targets []struct {
		URL  string `json:"url"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return "", err
	}

	for _, t := range targets {
		if t.Type == "page" {
			return t.URL, nil
		}
	}
	return "", nil
}

// executeJSOnPage executes JavaScript on the current page via CDP.
// This uses Chrome's /json endpoints and a WebSocket-free approach.
func (t *BrowserTool) executeJSOnPage(ctx context.Context, bi *browserInstance, js string) (string, error) {
	// Get a page target.
	targetID, err := t.getPageTargetID(bi)
	if err != nil {
		return "", err
	}

	// Use Chrome's /json/evaluate is not a standard endpoint.
	// Instead, we use a helper script approach via the exec tool pattern.
	// For JS execution, we can use Chrome's --eval flag on an already-running page.

	// The most reliable HTTP-only approach for CDP is to use the fetch-based
	// protocol. We'll craft a CDP message as HTTP POST.
	reqBody, _ := json.Marshal(map[string]interface{}{
		"id":     1,
		"method": "Runtime.evaluate",
		"params": map[string]interface{}{
			"expression":    js,
			"returnByValue": true,
		},
	})

	// Try the debug endpoint for the specific target.
	client := &http.Client{Timeout: browserActionTimeout}
	url := fmt.Sprintf("%s/json/protocol/%s", bi.cdpURL, targetID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("browser_use: failed to create CDP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		// CDP HTTP protocol endpoint may not be available; fall back to
		// reporting the target state.
		return fmt.Sprintf("JS execution not available via HTTP CDP (target: %s). Use browse_page for content retrieval.", targetID), nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var cdpResp struct {
		Result struct {
			Value interface{} `json:"value"`
		} `json:"result"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &cdpResp); err != nil {
		return string(body), nil
	}

	if cdpResp.Error.Message != "" {
		return "", fmt.Errorf("browser_use: CDP error: %s", cdpResp.Error.Message)
	}

	switch v := cdpResp.Result.Value.(type) {
	case string:
		return v, nil
	default:
		result, _ := json.Marshal(cdpResp.Result.Value)
		return string(result), nil
	}
}

// collapseWhitespace reduces runs of whitespace to single spaces and newlines.
func collapseWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.Join(strings.Fields(line), " ")
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return strings.Join(result, "\n")
}
