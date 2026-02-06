// Package tools provides the interface and utilities for agent tools.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// WebSearchTool searches the web using Brave Search API.
type WebSearchTool struct {
	BaseTool
	apiKey     string
	maxResults int
	client     *http.Client
}

// BraveSearchResult represents a single search result from Brave API.
type BraveSearchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

// BraveSearchResponse represents the response from Brave Search API.
type BraveSearchResponse struct {
	Web struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
		} `json:"results"`
	} `json:"web"`
}

// NewWebSearchTool creates a new WebSearchTool with the given API key and max results.
func NewWebSearchTool(apiKey string, maxResults int) *WebSearchTool {
	if maxResults <= 0 {
		maxResults = 5
	}
	if maxResults > 10 {
		maxResults = 10
	}

	parameters := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query",
			},
			"count": map[string]interface{}{
				"type":        "integer",
				"description": "Number of results (1-10, default 5)",
				"minimum":     1,
				"maximum":     10,
				"default":     5,
			},
		},
		"required": []string{"query"},
	}

	return &WebSearchTool{
		BaseTool: NewBaseTool(
			"web_search",
			"Search the web using Brave Search API. Returns formatted results with title, URL, and description.",
			parameters,
		),
		apiKey:     apiKey,
		maxResults: maxResults,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewWebSearchToolFromEnv creates a new WebSearchTool using the BRAVE_API_KEY environment variable.
func NewWebSearchToolFromEnv(maxResults int) *WebSearchTool {
	apiKey := os.Getenv("BRAVE_API_KEY")
	return NewWebSearchTool(apiKey, maxResults)
}

// Execute performs the web search with the given parameters.
func (t *WebSearchTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	// Check if API key is configured
	if t.apiKey == "" {
		return "", errors.New("web_search: Brave Search API key not configured (set BRAVE_API_KEY environment variable)")
	}

	query, err := GetStringParam(params, "query")
	if err != nil {
		return "", fmt.Errorf("web_search: %w", err)
	}

	if strings.TrimSpace(query) == "" {
		return "", errors.New("web_search: query cannot be empty")
	}

	count := GetIntParamOr(params, "count", t.maxResults)
	if count < 1 {
		count = 1
	}
	if count > 10 {
		count = 10
	}

	// Build request URL
	searchURL := fmt.Sprintf(
		"https://api.search.brave.com/res/v1/web/search?q=%s&count=%d",
		url.QueryEscape(query),
		count,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return "", fmt.Errorf("web_search: failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", t.apiKey)

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("web_search: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("web_search: API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("web_search: failed to read response: %w", err)
	}

	var braveResp BraveSearchResponse
	if err := json.Unmarshal(body, &braveResp); err != nil {
		return "", fmt.Errorf("web_search: failed to parse response: %w", err)
	}

	// Format results
	if len(braveResp.Web.Results) == 0 {
		return "No results found for the query.", nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Search results for %q:\n\n", query))

	for i, r := range braveResp.Web.Results {
		result.WriteString(fmt.Sprintf("%d. %s\n", i+1, r.Title))
		result.WriteString(fmt.Sprintf("   URL: %s\n", r.URL))
		if r.Description != "" {
			result.WriteString(fmt.Sprintf("   %s\n", r.Description))
		}
		result.WriteString("\n")
	}

	return result.String(), nil
}

// SetAPIKey updates the API key.
func (t *WebSearchTool) SetAPIKey(apiKey string) {
	t.apiKey = apiKey
}

// WebFetchTool fetches and parses web pages.
type WebFetchTool struct {
	BaseTool
	maxChars int
	client   *http.Client
}

// WebFetchResult represents the result of fetching a web page.
type WebFetchResult struct {
	URL       string `json:"url"`
	FinalURL  string `json:"final_url"`
	Status    int    `json:"status"`
	Title     string `json:"title,omitempty"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated,omitempty"`
}

// NewWebFetchTool creates a new WebFetchTool with the given max characters limit.
func NewWebFetchTool(maxChars int) *WebFetchTool {
	if maxChars <= 0 {
		maxChars = 50000
	}

	parameters := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "URL to fetch (http or https only)",
			},
			"extract_mode": map[string]interface{}{
				"type":        "string",
				"description": "Extraction mode: 'markdown' for structured content, 'text' for plain text, 'raw' for raw HTML",
				"enum":        []string{"markdown", "text", "raw"},
				"default":     "markdown",
			},
		},
		"required": []string{"url"},
	}

	return &WebFetchTool{
		BaseTool: NewBaseTool(
			"web_fetch",
			"Fetch and parse web pages. Extracts main content from HTML pages and returns as markdown, plain text, or raw HTML.",
			parameters,
		),
		maxChars: maxChars,
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return errors.New("too many redirects (max 5)")
				}
				return nil
			},
		},
	}
}

// Execute fetches and parses the web page with the given parameters.
func (t *WebFetchTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	rawURL, err := GetStringParam(params, "url")
	if err != nil {
		return "", fmt.Errorf("web_fetch: %w", err)
	}

	// Validate URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("web_fetch: invalid URL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", errors.New("web_fetch: only http and https URLs are supported")
	}

	// SSRF protection: block requests to internal/private network addresses
	if isInternalURL(rawURL) {
		return "", errors.New("web_fetch: access to internal/private network addresses is blocked")
	}

	extractMode := GetStringParamOr(params, "extract_mode", "markdown")
	if extractMode != "markdown" && extractMode != "text" && extractMode != "raw" {
		extractMode = "markdown"
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("web_fetch: failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; uBot/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("web_fetch: request failed: %w", err)
	}
	defer resp.Body.Close()

	result := WebFetchResult{
		URL:      rawURL,
		FinalURL: resp.Request.URL.String(),
		Status:   resp.StatusCode,
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("web_fetch: HTTP error %d: %s", resp.StatusCode, resp.Status)
	}

	// Check content type
	contentType := resp.Header.Get("Content-Type")

	if extractMode == "raw" {
		// Return raw HTML
		body, err := io.ReadAll(io.LimitReader(resp.Body, int64(t.maxChars)))
		if err != nil {
			return "", fmt.Errorf("web_fetch: failed to read response: %w", err)
		}
		result.Content = string(body)
		if len(body) >= t.maxChars {
			result.Truncated = true
		}
	} else if strings.Contains(contentType, "text/html") || strings.Contains(contentType, "application/xhtml") {
		content, title, err := extractHTMLContent(resp.Body, extractMode)
		if err != nil {
			return "", fmt.Errorf("web_fetch: failed to extract content: %w", err)
		}
		result.Title = title
		result.Content = truncateText(content, t.maxChars)
		result.Truncated = len(content) > t.maxChars
	} else {
		// For non-HTML content, just read the body
		body, err := io.ReadAll(io.LimitReader(resp.Body, int64(t.maxChars)))
		if err != nil {
			return "", fmt.Errorf("web_fetch: failed to read response: %w", err)
		}
		result.Content = string(body)
		if len(body) >= t.maxChars {
			result.Truncated = true
		}
	}

	// Format output
	var output strings.Builder
	output.WriteString(fmt.Sprintf("URL: %s\n", result.URL))
	if result.FinalURL != result.URL {
		output.WriteString(fmt.Sprintf("Redirected to: %s\n", result.FinalURL))
	}
	if result.Title != "" {
		output.WriteString(fmt.Sprintf("Title: %s\n", result.Title))
	}
	output.WriteString("\n")
	output.WriteString(result.Content)
	if result.Truncated {
		output.WriteString("\n\n[Content truncated]")
	}

	return output.String(), nil
}

// extractHTMLContent extracts main content from HTML using goquery.
func extractHTMLContent(r io.Reader, mode string) (content string, title string, err error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return "", "", err
	}

	// Extract title
	title = strings.TrimSpace(doc.Find("title").First().Text())

	// Remove script, style, nav, footer, header, and other non-content elements
	doc.Find("script, style, nav, footer, header, aside, noscript, iframe, form, .nav, .navigation, .menu, .sidebar, .advertisement, .ads").Remove()

	// Try to find main content in order of preference
	var contentEl *goquery.Selection
	selectors := []string{"article", "main", "[role=main]", ".content", "#content", ".post", ".article", ".entry-content", ".post-content", "body"}

	for _, selector := range selectors {
		selection := doc.Find(selector)
		if selection.Length() > 0 {
			contentEl = selection.First()
			break
		}
	}

	if contentEl == nil {
		contentEl = doc.Find("body")
	}

	if mode == "text" {
		return strings.TrimSpace(contentEl.Text()), title, nil
	}

	// Convert to markdown-like format
	return htmlToMarkdown(contentEl), title, nil
}

// htmlToMarkdown converts HTML content to a markdown-like format.
func htmlToMarkdown(s *goquery.Selection) string {
	var result strings.Builder
	visited := make(map[*goquery.Selection]bool)

	// Process block elements first
	s.Find("h1, h2, h3, h4, h5, h6, p, ul, ol, pre, blockquote, table").Each(func(i int, el *goquery.Selection) {
		if visited[el] {
			return
		}
		visited[el] = true

		tagName := goquery.NodeName(el)

		switch tagName {
		case "h1":
			text := strings.TrimSpace(el.Text())
			if text != "" {
				result.WriteString("# " + text + "\n\n")
			}
		case "h2":
			text := strings.TrimSpace(el.Text())
			if text != "" {
				result.WriteString("## " + text + "\n\n")
			}
		case "h3":
			text := strings.TrimSpace(el.Text())
			if text != "" {
				result.WriteString("### " + text + "\n\n")
			}
		case "h4":
			text := strings.TrimSpace(el.Text())
			if text != "" {
				result.WriteString("#### " + text + "\n\n")
			}
		case "h5":
			text := strings.TrimSpace(el.Text())
			if text != "" {
				result.WriteString("##### " + text + "\n\n")
			}
		case "h6":
			text := strings.TrimSpace(el.Text())
			if text != "" {
				result.WriteString("###### " + text + "\n\n")
			}
		case "p":
			text := processInlineElements(el)
			if text != "" {
				result.WriteString(text + "\n\n")
			}
		case "ul", "ol":
			el.Find("li").Each(func(j int, li *goquery.Selection) {
				text := strings.TrimSpace(li.Text())
				if text != "" {
					if tagName == "ol" {
						result.WriteString(fmt.Sprintf("%d. %s\n", j+1, text))
					} else {
						result.WriteString("- " + text + "\n")
					}
				}
			})
			result.WriteString("\n")
		case "pre":
			text := strings.TrimSpace(el.Text())
			if text != "" {
				result.WriteString("```\n" + text + "\n```\n\n")
			}
		case "blockquote":
			lines := strings.Split(strings.TrimSpace(el.Text()), "\n")
			for _, line := range lines {
				result.WriteString("> " + strings.TrimSpace(line) + "\n")
			}
			result.WriteString("\n")
		case "table":
			result.WriteString(processTable(el))
			result.WriteString("\n")
		}
	})

	// If no structured content found, fall back to plain text
	if result.Len() == 0 {
		return cleanText(s.Text())
	}

	return strings.TrimSpace(result.String())
}

// processInlineElements processes inline elements within a block.
func processInlineElements(el *goquery.Selection) string {
	var result strings.Builder

	el.Contents().Each(func(i int, s *goquery.Selection) {
		if goquery.NodeName(s) == "#text" {
			result.WriteString(s.Text())
		} else {
			switch goquery.NodeName(s) {
			case "a":
				href, exists := s.Attr("href")
				text := strings.TrimSpace(s.Text())
				if exists && href != "" && !strings.HasPrefix(href, "#") && text != "" {
					result.WriteString(fmt.Sprintf("[%s](%s)", text, href))
				} else if text != "" {
					result.WriteString(text)
				}
			case "strong", "b":
				text := strings.TrimSpace(s.Text())
				if text != "" {
					result.WriteString("**" + text + "**")
				}
			case "em", "i":
				text := strings.TrimSpace(s.Text())
				if text != "" {
					result.WriteString("*" + text + "*")
				}
			case "code":
				text := strings.TrimSpace(s.Text())
				if text != "" {
					result.WriteString("`" + text + "`")
				}
			default:
				result.WriteString(s.Text())
			}
		}
	})

	return strings.TrimSpace(result.String())
}

// processTable converts an HTML table to markdown format.
func processTable(table *goquery.Selection) string {
	var result strings.Builder
	var headers []string
	var rows [][]string

	// Extract headers
	table.Find("thead tr th, thead tr td, tr:first-child th").Each(func(i int, th *goquery.Selection) {
		headers = append(headers, strings.TrimSpace(th.Text()))
	})

	// Extract rows
	table.Find("tbody tr, tr").Each(func(i int, tr *goquery.Selection) {
		var row []string
		tr.Find("td").Each(func(j int, td *goquery.Selection) {
			row = append(row, strings.TrimSpace(td.Text()))
		})
		if len(row) > 0 {
			rows = append(rows, row)
		}
	})

	// Build markdown table
	if len(headers) > 0 {
		result.WriteString("| " + strings.Join(headers, " | ") + " |\n")
		result.WriteString("|" + strings.Repeat(" --- |", len(headers)) + "\n")
	}

	for _, row := range rows {
		result.WriteString("| " + strings.Join(row, " | ") + " |\n")
	}

	return result.String()
}

// cleanText cleans up text by removing excess whitespace.
func cleanText(text string) string {
	// Replace multiple whitespace with single space
	text = strings.Join(strings.Fields(text), " ")
	return strings.TrimSpace(text)
}

// truncateText truncates text to the specified maximum length.
func truncateText(text string, maxChars int) string {
	if len(text) <= maxChars {
		return text
	}
	// Try to cut at a sentence boundary
	truncated := text[:maxChars]
	if lastPeriod := strings.LastIndex(truncated, ". "); lastPeriod > maxChars/2 {
		return truncated[:lastPeriod+1]
	}
	// Try to cut at a word boundary
	if lastSpace := strings.LastIndex(truncated, " "); lastSpace > maxChars/2 {
		return truncated[:lastSpace] + "..."
	}
	return truncated + "..."
}

// isInternalURL checks whether a URL targets an internal or private network address.
// It returns true if the resolved IP is loopback, private, link-local, or a known
// cloud metadata address (169.254.169.254).
func isInternalURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return true
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		return true
	}

	ips, err := net.LookupIP(hostname)
	if err != nil {
		// If DNS resolution fails, allow the request — it will fail at HTTP level
		// with a clearer error. Blocking here would break fetch in no-DNS environments.
		return false
	}

	cloudMetadataIP := net.ParseIP("169.254.169.254")

	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return true
		}
		if ip.Equal(cloudMetadataIP) {
			return true
		}
	}

	return false
}
