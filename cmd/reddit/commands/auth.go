// Package commands provides command implementations for the Reddit CLI.
package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit/output"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
)

// TestAuth tests the authentication by calling client.Me() and returns success or error.
// If verbose is true, prints additional authentication details.
func TestAuth(ctx context.Context, client *graw.Reddit, verbose bool) error {
	if client == nil {
		return fmt.Errorf("client cannot be nil")
	}

	account, err := client.Me(ctx)
	if err != nil {
		return fmt.Errorf("authentication test failed: %w", err)
	}

	if verbose {
		fmt.Printf("Authentication successful.\n")
		fmt.Printf("Username: %s\n", account.Name)
		fmt.Printf("Link Karma: %d\n", account.LinkKarma)
		fmt.Printf("Comment Karma: %d\n", account.CommentKarma)
	} else {
		fmt.Printf("Authentication successful as user: %s\n", account.Name)
	}

	return nil
}

// ShowAuthInfo displays information about the authenticated user using a formatter.
func ShowAuthInfo(ctx context.Context, client *graw.Reddit) error {
	if client == nil {
		return fmt.Errorf("client cannot be nil")
	}

	account, err := client.Me(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve authentication info: %w", err)
	}

	// Create a formatter to display the user info
	formatter, err := output.New(output.Config{
		Writer:       os.Stdout,
		Format:       "text",
		ColorEnabled: true,
		Compact:      false,
	})
	if err != nil {
		return fmt.Errorf("failed to create formatter: %w", err)
	}

	return formatter.FormatUser(account)
}
