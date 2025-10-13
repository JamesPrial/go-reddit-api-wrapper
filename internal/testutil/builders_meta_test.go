package testutil

import (
	"encoding/json"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

func TestSubredditBuilder(t *testing.T) {
	t.Run("basic build", func(t *testing.T) {
		subreddit := NewSubreddit("golang").Build()

		if subreddit.ID != "sub123" {
			t.Errorf("Expected ID sub123, got %s", subreddit.ID)
		}
		if subreddit.Name != "t5_sub123" {
			t.Errorf("Expected Name t5_sub123, got %s", subreddit.Name)
		}
		if subreddit.DisplayName != "golang" {
			t.Errorf("Expected DisplayName golang, got %s", subreddit.DisplayName)
		}
		if subreddit.Subscribers != 10000 {
			t.Errorf("Expected Subscribers 10000, got %d", subreddit.Subscribers)
		}
	})

	t.Run("fluent customization", func(t *testing.T) {
		subreddit := NewSubreddit("programming").
			WithID("abc123").
			WithSubscribers(50000).
			WithTitle("Programming Community").
			WithDescription("A community for programmers").
			WithActiveUsers(250).
			WithNSFW(true).
			Build()

		if subreddit.ID != "abc123" {
			t.Errorf("Expected ID abc123, got %s", subreddit.ID)
		}
		if subreddit.Name != "t5_abc123" {
			t.Errorf("Expected Name t5_abc123, got %s", subreddit.Name)
		}
		if subreddit.Subscribers != 50000 {
			t.Errorf("Expected Subscribers 50000, got %d", subreddit.Subscribers)
		}
		if subreddit.Title != "Programming Community" {
			t.Errorf("Expected Title 'Programming Community', got %s", subreddit.Title)
		}
		if subreddit.AccountsActive != 250 {
			t.Errorf("Expected AccountsActive 250, got %d", subreddit.AccountsActive)
		}
		if !subreddit.Over18 {
			t.Error("Expected Over18 to be true")
		}
	})

	t.Run("ToThing", func(t *testing.T) {
		thing := NewSubreddit("test").WithID("xyz").ToThing()

		if thing.Kind != "t5" {
			t.Errorf("Expected kind t5, got %s", thing.Kind)
		}
		if thing.ID != "xyz" {
			t.Errorf("Expected ID xyz, got %s", thing.ID)
		}
		if thing.Name != "t5_xyz" {
			t.Errorf("Expected Name t5_xyz, got %s", thing.Name)
		}

		// Verify data can be unmarshaled
		var data types.SubredditData
		if err := json.Unmarshal(thing.Data, &data); err != nil {
			t.Fatalf("Failed to unmarshal thing data: %v", err)
		}
		if data.DisplayName != "test" {
			t.Errorf("Expected DisplayName test, got %s", data.DisplayName)
		}
	})

	t.Run("ToJSON", func(t *testing.T) {
		jsonData := NewSubreddit("askreddit").WithSubscribers(1000000).ToJSON()

		var data types.SubredditData
		if err := json.Unmarshal(jsonData, &data); err != nil {
			t.Fatalf("Failed to unmarshal JSON: %v", err)
		}
		if data.DisplayName != "askreddit" {
			t.Errorf("Expected DisplayName askreddit, got %s", data.DisplayName)
		}
		if data.Subscribers != 1000000 {
			t.Errorf("Expected Subscribers 1000000, got %d", data.Subscribers)
		}
	})
}

func TestAccountBuilder(t *testing.T) {
	t.Run("basic build", func(t *testing.T) {
		account := NewAccount("spez").Build()

		if account.ID != "user123" {
			t.Errorf("Expected ID user123, got %s", account.ID)
		}
		if account.Name != "t2_user123" {
			t.Errorf("Expected Name t2_user123, got %s", account.Name)
		}
		if account.LinkKarma != 1000 {
			t.Errorf("Expected LinkKarma 1000, got %d", account.LinkKarma)
		}
		if account.CommentKarma != 500 {
			t.Errorf("Expected CommentKarma 500, got %d", account.CommentKarma)
		}
	})

	t.Run("fluent customization", func(t *testing.T) {
		account := NewAccount("testuser").
			WithID("user456").
			WithLinkKarma(5000).
			WithCommentKarma(10000).
			WithGold(true).
			WithMod(true).
			Build()

		if account.ID != "user456" {
			t.Errorf("Expected ID user456, got %s", account.ID)
		}
		if account.Name != "t2_user456" {
			t.Errorf("Expected Name t2_user456, got %s", account.Name)
		}
		if account.LinkKarma != 5000 {
			t.Errorf("Expected LinkKarma 5000, got %d", account.LinkKarma)
		}
		if account.CommentKarma != 10000 {
			t.Errorf("Expected CommentKarma 10000, got %d", account.CommentKarma)
		}
		if !account.IsGold {
			t.Error("Expected IsGold to be true")
		}
		if !account.IsMod {
			t.Error("Expected IsMod to be true")
		}
	})

	t.Run("ToThing", func(t *testing.T) {
		thing := NewAccount("AutoModerator").WithID("auto").ToThing()

		if thing.Kind != "t2" {
			t.Errorf("Expected kind t2, got %s", thing.Kind)
		}
		if thing.ID != "auto" {
			t.Errorf("Expected ID auto, got %s", thing.ID)
		}
		if thing.Name != "t2_auto" {
			t.Errorf("Expected Name t2_auto, got %s", thing.Name)
		}

		// Verify data can be unmarshaled
		var data types.AccountData
		if err := json.Unmarshal(thing.Data, &data); err != nil {
			t.Fatalf("Failed to unmarshal thing data: %v", err)
		}
		if data.ID != "auto" {
			t.Errorf("Expected ID auto, got %s", data.ID)
		}
	})

	t.Run("ToJSON", func(t *testing.T) {
		jsonData := NewAccount("admin").WithLinkKarma(99999).ToJSON()

		var data types.AccountData
		if err := json.Unmarshal(jsonData, &data); err != nil {
			t.Fatalf("Failed to unmarshal JSON: %v", err)
		}
		if data.LinkKarma != 99999 {
			t.Errorf("Expected LinkKarma 99999, got %d", data.LinkKarma)
		}
	})
}

func TestMoreBuilder(t *testing.T) {
	t.Run("basic build", func(t *testing.T) {
		more := NewMore().Build()

		if more.ID != "more123" {
			t.Errorf("Expected ID more123, got %s", more.ID)
		}
		if more.Name != "more_more123" {
			t.Errorf("Expected Name more_more123, got %s", more.Name)
		}
		if len(more.Children) != 0 {
			t.Errorf("Expected empty Children, got %d items", len(more.Children))
		}
	})

	t.Run("fluent customization", func(t *testing.T) {
		children := []string{"abc123", "def456", "ghi789"}
		more := NewMore().
			WithID("more456").
			WithChildren(children).
			WithCount(15). // This is a no-op but should not error
			Build()

		if more.ID != "more456" {
			t.Errorf("Expected ID more456, got %s", more.ID)
		}
		if more.Name != "more_more456" {
			t.Errorf("Expected Name more_more456, got %s", more.Name)
		}
		if len(more.Children) != 3 {
			t.Errorf("Expected 3 children, got %d", len(more.Children))
		}
		if more.Children[0] != "abc123" {
			t.Errorf("Expected first child abc123, got %s", more.Children[0])
		}
	})

	t.Run("ToThing", func(t *testing.T) {
		thing := NewMore().WithID("more789").WithChildren([]string{"id1", "id2"}).ToThing()

		if thing.Kind != "more" {
			t.Errorf("Expected kind more, got %s", thing.Kind)
		}
		if thing.ID != "more789" {
			t.Errorf("Expected ID more789, got %s", thing.ID)
		}
		if thing.Name != "more_more789" {
			t.Errorf("Expected Name more_more789, got %s", thing.Name)
		}

		// Verify data can be unmarshaled
		var data types.MoreData
		if err := json.Unmarshal(thing.Data, &data); err != nil {
			t.Fatalf("Failed to unmarshal thing data: %v", err)
		}
		if len(data.Children) != 2 {
			t.Errorf("Expected 2 children, got %d", len(data.Children))
		}
	})

	t.Run("ToJSON", func(t *testing.T) {
		jsonData := NewMore().WithChildren([]string{"comment1", "comment2", "comment3"}).ToJSON()

		var data types.MoreData
		if err := json.Unmarshal(jsonData, &data); err != nil {
			t.Fatalf("Failed to unmarshal JSON: %v", err)
		}
		if len(data.Children) != 3 {
			t.Errorf("Expected 3 children, got %d", len(data.Children))
		}
	})
}

// Test chaining multiple builders together
func TestBuilderChaining(t *testing.T) {
	// Build a subreddit, account, and more object
	subreddit := NewSubreddit("golang").WithSubscribers(50000).Build()
	account := NewAccount("gopher").WithLinkKarma(10000).Build()
	more := NewMore().WithChildren([]string{"c1", "c2"}).Build()

	if subreddit.DisplayName != "golang" {
		t.Error("Subreddit builder failed")
	}
	if account.LinkKarma != 10000 {
		t.Error("Account builder failed")
	}
	if len(more.Children) != 2 {
		t.Error("More builder failed")
	}
}
