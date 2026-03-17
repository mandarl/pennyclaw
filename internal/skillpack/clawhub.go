package skillpack

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// ClawHubClient provides access to the ClawHub skill registry.
type ClawHubClient struct {
	registryURL string
	client      *http.Client
}

// ClawHubSearchResult represents a skill found on ClawHub.
type ClawHubSearchResult struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Author      string `json:"author"`
	Version     string `json:"version"`
	Downloads   int    `json:"downloads"`
	Stars       int    `json:"stars"`
	URL         string `json:"url"`
}

// NewClawHubClient creates a new ClawHub registry client.
func NewClawHubClient() *ClawHubClient {
	registryURL := os.Getenv("CLAWHUB_REGISTRY")
	if registryURL == "" {
		registryURL = "https://registry.clawhub.dev"
	}

	return &ClawHubClient{
		registryURL: registryURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Search queries ClawHub for skills matching the given query.
func (c *ClawHubClient) Search(query string, limit int) ([]ClawHubSearchResult, error) {
	if limit <= 0 {
		limit = 20
	}

	searchURL := fmt.Sprintf("%s/api/v1/skills/search?q=%s&limit=%d",
		c.registryURL, url.QueryEscape(query), limit)

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating search request: %w", err)
	}
	req.Header.Set("User-Agent", "PennyClaw/0.2.0")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("querying ClawHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("ClawHub returned status %d: %s", resp.StatusCode, string(body))
	}

	var results struct {
		Skills []ClawHubSearchResult `json:"skills"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("parsing ClawHub response: %w", err)
	}

	return results.Skills, nil
}

// GetSkillInfo retrieves detailed information about a specific skill.
func (c *ClawHubClient) GetSkillInfo(name string) (*ClawHubSearchResult, error) {
	infoURL := fmt.Sprintf("%s/api/v1/skills/%s", c.registryURL, url.PathEscape(name))

	req, err := http.NewRequest("GET", infoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "PennyClaw/0.2.0")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("querying ClawHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("skill %q not found on ClawHub", name)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ClawHub returned status %d", resp.StatusCode)
	}

	var result ClawHubSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return &result, nil
}
