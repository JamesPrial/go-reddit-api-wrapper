// Package commands provides command implementations for the Reddit CLI.
package commands

import (
	"context"
	"fmt"

	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit/output"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
)

// GetMe retrieves and displays information about the authenticated user.
// It calls client.Me() to fetch account data and uses the provided formatter to display results.
// Returns an error if the API call fails or formatting fails.
func GetMe(ctx context.Context, client *graw.Reddit, formatter output.Formatter) error {
	if client == nil {
		return fmt.Errorf("client cannot be nil")
	}
	if formatter == nil {
		return fmt.Errorf("formatter cannot be nil")
	}

	account, err := client.Me(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve user account info: %w", err)
	}

	if account == nil {
		return fmt.Errorf("received nil account data from API")
	}

	return formatter.FormatUser(account)
}
