package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Simple test to see what Reddit actually returns
func main() {
	clientID := os.Getenv("REDDIT_CLIENT_ID")
	clientSecret := os.Getenv("REDDIT_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		fmt.Println("REDDIT_CLIENT_ID and REDDIT_CLIENT_SECRET required")
		return
	}

	// Get OAuth token
	token, err := getOAuthToken(clientID, clientSecret)
	if err != nil {
		fmt.Printf("Failed to get token: %v\n", err)
		return
	}

	fmt.Printf("Got token: %.20s...\n", token)

	// Test fetching comments
	testPostID := "1h5wokb" // A known post ID from r/golang
	resp, err := fetchComments(token, "golang", testPostID)
	if err != nil {
		fmt.Printf("Failed to fetch: %v\n", err)
		return
	}

	// Parse and display structure
	var result []interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		fmt.Printf("Failed to parse: %v\n", err)
		fmt.Printf("Raw response: %.500s...\n", string(resp))
		return
	}

	fmt.Printf("\nReddit API Response Structure:\n")
	fmt.Printf("Number of top-level elements: %d\n", len(result))

	for i, elem := range result {
		if m, ok := elem.(map[string]interface{}); ok {
			kind := m["kind"]
			fmt.Printf("\nElement %d:\n", i)
			fmt.Printf("  Kind: %v\n", kind)

			if data, ok := m["data"].(map[string]interface{}); ok {
				if children, ok := data["children"].([]interface{}); ok {
					fmt.Printf("  Children count: %d\n", len(children))

					// Show first child's kind
					if len(children) > 0 {
						if child, ok := children[0].(map[string]interface{}); ok {
							fmt.Printf("  First child kind: %v\n", child["kind"])
							if childData, ok := child["data"].(map[string]interface{}); ok {
								// Check if it's a post (t3) or comment (t1)
								if child["kind"] == "t3" {
									fmt.Printf("    Post title: %v\n", childData["title"])
								} else if child["kind"] == "t1" {
									body := fmt.Sprintf("%v", childData["body"])
									if len(body) > 50 {
										body = body[:50] + "..."
									}
									fmt.Printf("    Comment: %s\n", body)
								}
							}
						}
					}
				}
			}
		}
	}
}

func getOAuthToken(clientID, clientSecret string) (string, error) {
	req, err := http.NewRequest("POST", "https://www.reddit.com/api/v1/access_token", strings.NewReader("grant_type=client_credentials"))
	if err != nil {
		return "", err
	}

	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("User-Agent", "TestBot/1.0")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", err
	}

	if tokenResp.Error != "" {
		return "", fmt.Errorf("auth error: %s", tokenResp.Error)
	}

	return tokenResp.AccessToken, nil
}

func fetchComments(token, subreddit, postID string) ([]byte, error) {
	url := fmt.Sprintf("https://oauth.reddit.com/r/%s/comments/%s?limit=5", subreddit, postID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "TestBot/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}