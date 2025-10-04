package test_generators

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// PostGenerator generates realistic Reddit posts for testing
type PostGenerator struct {
	rand           *rand.Rand
	titleTemplates []string
	bodyTemplates  []string
	subreddits     []string
	users          []string
	flairs         []string
}

// NewPostGenerator creates a new post generator
func NewPostGenerator(seed int64) *PostGenerator {
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	return &PostGenerator{
		rand: rand.New(rand.NewSource(seed)),
		titleTemplates: []string{
			"Ask %s: %s",
			"%s - %s",
			"[Discussion] %s",
			"[Meme] %s",
			"PSA: %s",
			"Unpopular Opinion: %s",
			"Shower Thought: %s",
			"TIL about %s",
			"Breaking: %s",
			"Analysis: %s",
		},
		bodyTemplates: []string{
			"What are your thoughts on %s? I've been thinking about this lately and wanted to get the community's opinion.",
			"Here's my take on %s. I've done some research and wanted to share what I found.",
			"Can someone explain %s to me? I'm having trouble understanding this concept.",
			"I just discovered %s and it's amazing! Here's why...",
			"Warning about %s. I learned this the hard way and wanted to share.",
			"Quick question about %s. Has anyone else experienced this?",
			"Deep dive into %s. This is more complex than it seems.",
			"Personal story about %s. This changed my perspective.",
			"Technical explanation of %s for those interested.",
			"Beginner's guide to %s. Everything you need to know.",
		},
		subreddits: []string{
			"programming", "gaming", "technology", "science", "askreddit",
			"todayilearned", "news", "funny", "pics", "videos",
			"aww", "music", "movies", "books", "sports",
			"food", "travel", "fitness", "DIY", "art",
		},
		users: []string{
			"tech_enthusiast", "casual_redditor", "expert_analyst", "curious_mind",
			"seasoned_veteran", "newbie_user", "power_user", "lurker_account",
			"active_contributor", "moderator_user", "content_creator", "commentator",
		},
		flairs: []string{
			"Discussion", "Question", "News", "Meme", "PSA", "Guide",
			"Analysis", "Personal", "Technical", "Beginner", "Advanced",
		},
	}
}

// GeneratePost creates a realistic Reddit post
func (pg *PostGenerator) GeneratePost() types.Post {
	title := pg.generateTitle()
	body := pg.generateBody()
	subreddit := pg.randElement(pg.subreddits)
	author := pg.randElement(pg.users)
	flair := pg.randElement(pg.flairs)

	now := time.Now()
	createdTime := now.Add(-time.Duration(pg.rand.Intn(86400)) * time.Second) // Random time in last 24h

	post := types.Post{
		ID:        pg.generatePostID(),
		Title:     title,
		Author:    author,
		Subreddit: subreddit,
		Score:     pg.generateScore(),
		Created:   createdTime,
		Edited:    pg.generateEditedTime(createdTime),
		Flair:     flair,
		Body:      body,
		URL:       pg.generatePostURL(subreddit, title),
		Permalink: pg.generatePermalink(subreddit, title),
		IsSelf:    len(body) > 0,
		NSFW:      pg.rand.Float32() < 0.05, // 5% chance
		Spoiler:   pg.rand.Float32() < 0.02, // 2% chance
		Locked:    pg.rand.Float32() < 0.01, // 1% chance
		Archived:  pg.rand.Float32() < 0.01, // 1% chance
	}

	// Add optional fields
	if pg.rand.Float32() < 0.3 { // 30% chance
		post.ThumbnailURL = pg.generateThumbnailURL()
	}

	if pg.rand.Float32() < 0.2 { // 20% chance
		post.Domain = pg.generateDomain()
	}

	return post
}

// GeneratePosts creates multiple posts
func (pg *PostGenerator) GeneratePosts(count int) []types.Post {
	posts := make([]types.Post, count)
	for i := 0; i < count; i++ {
		posts[i] = pg.GeneratePost()
	}
	return posts
}

// GeneratePostsWithOptions creates posts with specific characteristics
func (pg *PostGenerator) GeneratePostsWithOptions(count int, opts PostOptions) []types.Post {
	posts := make([]types.Post, count)
	for i := 0; i < count; i++ {
		posts[i] = pg.GeneratePostWithOptions(opts)
	}
	return posts
}

// PostOptions controls post generation characteristics
type PostOptions struct {
	MinScore     int
	MaxScore     int
	MinComments  int
	MaxComments  int
	NSFW         bool
	Spoiler      bool
	Locked       bool
	Archived     bool
	HasBody      bool
	HasFlair     bool
	HasThumbnail bool
	Subreddit    string
	Author       string
}

// GeneratePostWithOptions creates a post with specific options
func (pg *PostGenerator) GeneratePostWithOptions(opts PostOptions) types.Post {
	post := pg.GeneratePost()

	// Apply options
	if opts.MinScore > 0 && post.Score < opts.MinScore {
		post.Score = opts.MinScore + pg.rand.Intn(opts.MaxScore-opts.MinScore+1)
	}
	if opts.MaxScore > 0 && post.Score > opts.MaxScore {
		post.Score = opts.MinScore + pg.rand.Intn(opts.MaxScore-opts.MinScore+1)
	}

	if opts.MinComments > 0 {
		post.NumComments = opts.MinComments + pg.rand.Intn(opts.MaxComments-opts.MinComments+1)
	}

	post.NSFW = opts.NSFW
	post.Spoiler = opts.Spoiler
	post.Locked = opts.Locked
	post.Archived = opts.Archived

	if !opts.HasBody {
		post.Body = ""
		post.IsSelf = false
	}

	if !opts.HasFlair {
		post.Flair = ""
	}

	if !opts.HasThumbnail {
		post.ThumbnailURL = ""
	}

	if opts.Subreddit != "" {
		post.Subreddit = opts.Subreddit
		post.URL = pg.generatePostURL(post.Subreddit, post.Title)
		post.Permalink = pg.generatePermalink(post.Subreddit, post.Title)
	}

	if opts.Author != "" {
		post.Author = opts.Author
	}

	return post
}

// GenerateControversialPost creates a post with controversial characteristics
func (pg *PostGenerator) GenerateControversialPost() types.Post {
	post := pg.GeneratePost()

	// Controversial posts typically have high upvotes and downvotes
	post.Score = pg.rand.Intn(5000) + 1000
	post.UpvoteRatio = 0.4 + pg.rand.Float32()*0.2 // 40-60% upvote ratio
	post.NumComments = pg.rand.Intn(2000) + 500

	// Often controversial topics
	controversialTopics := []string{
		"politics", "religion", "social issues", "controversial opinion",
		"unpopular take", "debate topic", "divisive issue",
	}

	if pg.rand.Float32() < 0.7 {
		post.Flair = pg.randElement(controversialTopics)
	}

	return post
}

// GeneratePopularPost creates a post with popular characteristics
func (pg *PostGenerator) GeneratePopularPost() types.Post {
	post := pg.GeneratePost()

	// Popular posts have high scores and many comments
	post.Score = pg.rand.Intn(50000) + 10000
	post.UpvoteRatio = 0.85 + pg.rand.Float32()*0.14 // 85-99% upvote ratio
	post.NumComments = pg.rand.Intn(10000) + 1000

	// Often have awards
	if pg.rand.Float32() < 0.6 {
		post.TotalAwards = pg.rand.Intn(50) + 1
	}

	return post
}

// GenerateOldPost creates an old post
func (pg *PostGenerator) GenerateOldPost() types.Post {
	post := pg.GeneratePost()

	// Old posts are from months or years ago
	daysAgo := pg.rand.Intn(365*2) + 30 // 30 days to 2 years ago
	post.Created = time.Now().Add(-time.Duration(daysAgo) * 24 * time.Hour)
	post.Edited = time.Time{} // Usually not edited

	// Often archived
	if pg.rand.Float32() < 0.3 {
		post.Archived = true
	}

	return post
}

// Helper methods

func (pg *PostGenerator) generateTitle() string {
	template := pg.randElement(pg.titleTemplates)
	topic := pg.generateTopic()

	switch template {
	case "Ask %s: %s":
		subreddit := pg.randElement([]string{"Reddit", "Tech", "Science", "Gaming"})
		return fmt.Sprintf(template, subreddit, topic)
	case "%s - %s":
		prefix := pg.randElement([]string{"Breaking", "Update", "Analysis", "Opinion"})
		return fmt.Sprintf(template, prefix, topic)
	default:
		return fmt.Sprintf(template, topic)
	}
}

func (pg *PostGenerator) generateBody() string {
	if pg.rand.Float32() < 0.3 { // 30% chance of no body (link post)
		return ""
	}

	template := pg.randElement(pg.bodyTemplates)
	topic := pg.generateTopic()
	body := fmt.Sprintf(template, topic)

	// Add some random paragraphs
	if pg.rand.Float32() < 0.5 {
		body += "\n\n" + pg.generateParagraph()
	}
	if pg.rand.Float32() < 0.3 {
		body += "\n\n" + pg.generateParagraph()
	}

	return body
}

func (pg *PostGenerator) generateTopic() string {
	topics := []string{
		"the future of AI", "climate change solutions", "space exploration",
		"quantum computing", "renewable energy", "cryptocurrency trends",
		"social media impact", "remote work culture", "mental health awareness",
		"sustainable living", "educational reform", "healthcare innovation",
		"privacy concerns", "digital transformation", "cybersecurity threats",
		"machine learning applications", "blockchain technology", "virtual reality",
		"automation effects", "data science trends", "cloud computing",
	}
	return pg.randElement(topics)
}

func (pg *PostGenerator) generateParagraph() string {
	sentences := pg.rand.Intn(5) + 2
	paragraph := ""

	for i := 0; i < sentences; i++ {
		if i > 0 {
			paragraph += " "
		}
		paragraph += pg.generateSentence()
	}

	return paragraph
}

func (pg *PostGenerator) generateSentence() string {
	words := pg.rand.Intn(15) + 5
	sentence := ""

	wordList := []string{
		"the", "a", "an", "and", "or", "but", "in", "on", "at", "to", "for",
		"of", "with", "by", "from", "up", "about", "into", "through", "during",
		"before", "after", "above", "below", "between", "among", "this", "that",
		"these", "those", "is", "are", "was", "were", "be", "been", "being",
		"have", "has", "had", "do", "does", "did", "will", "would", "could",
		"should", "may", "might", "must", "can", "important", "significant",
		"interesting", "complex", "simple", "difficult", "easy", "challenging",
		"valuable", "useful", "helpful", "effective", "efficient", "successful",
		"approach", "method", "technique", "strategy", "solution", "problem",
		"issue", "challenge", "opportunity", "benefit", "advantage", "disadvantage",
	}

	for i := 0; i < words; i++ {
		if i > 0 {
			sentence += " "
		}
		sentence += pg.randElement(wordList)
	}

	return strings.Title(sentence) + "."
}

func (pg *PostGenerator) generateScore() int {
	// Most posts have low scores, few have high scores (power law distribution)
	rand := pg.rand.Float64()
	if rand < 0.7 {
		return pg.rand.Intn(100) // 70% of posts: 0-99 score
	} else if rand < 0.9 {
		return pg.rand.Intn(900) + 100 // 20% of posts: 100-999 score
	} else if rand < 0.98 {
		return pg.rand.Intn(9000) + 1000 // 8% of posts: 1000-9999 score
	} else {
		return pg.rand.Intn(90000) + 10000 // 2% of posts: 10000+ score
	}
}

func (pg *PostGenerator) generatePostID() string {
	return fmt.Sprintf("t3_%x", pg.rand.Int63())
}

func (pg *PostGenerator) generateEditedTime(created time.Time) time.Time {
	if pg.rand.Float32() < 0.8 { // 80% chance not edited
		return time.Time{}
	}

	// Edited sometime after creation
	editDelay := time.Duration(pg.rand.Intn(3600)) * time.Second // Within 1 hour
	return created.Add(editDelay)
}

func (pg *PostGenerator) generatePostURL(subreddit, title string) string {
	if pg.rand.Float32() < 0.3 { // 30% chance it's a link post
		return pg.generateExternalURL()
	}
	return fmt.Sprintf("https://reddit.com/r/%s/comments/%s", subreddit, pg.generatePostID()[3:])
}

func (pg *PostGenerator) generatePermalink(subreddit, title string) string {
	slug := strings.ToLower(strings.ReplaceAll(title, " ", "_"))
	slug = strings.ReplaceAll(slug, "?", "")
	slug = strings.ReplaceAll(slug, "!", "")
	slug = strings.ReplaceAll(slug, ".", "")

	if len(slug) > 50 {
		slug = slug[:50]
	}

	return fmt.Sprintf("/r/%s/comments/%s/%s", subreddit, pg.generatePostID()[3:], slug)
}

func (pg *PostGenerator) generateExternalURL() string {
	domains := []string{
		"github.com", "youtube.com", "twitter.com", "news.ycombinator.com",
		"medium.com", "dev.to", "stackoverflow.com", "wikipedia.org",
		"nytimes.com", "bbc.com", "cnn.com", "techcrunch.com",
	}

	return fmt.Sprintf("https://%s/%s", pg.randElement(domains), pg.randString(10))
}

func (pg *PostGenerator) generateThumbnailURL() string {
	if pg.rand.Float32() < 0.5 {
		return "" // No thumbnail
	}

	return fmt.Sprintf("https://preview.redd.it/%s.jpg?width=320&crop=smart", pg.randString(20))
}

func (pg *PostGenerator) generateDomain() string {
	domains := []string{
		"self." + pg.randElement(pg.subreddits), // Self post
		"youtube.com", "imgur.com", "github.com", "twitter.com",
		"medium.com", "dev.to", "news.ycombinator.com", "wikipedia.org",
	}
	return pg.randElement(domains)
}

func (pg *PostGenerator) randElement(slice []string) string {
	return slice[pg.rand.Intn(len(slice))]
}

func (pg *PostGenerator) randString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[pg.rand.Intn(len(charset))]
	}
	return string(b)
}

// Preset generators for common scenarios

func (pg *PostGenerator) GenerateTechPosts(count int) []types.Post {
	opts := PostOptions{
		Subreddit: "programming",
		HasBody:   true,
		HasFlair:  true,
		MinScore:  10,
		MaxScore:  5000,
	}
	return pg.GeneratePostsWithOptions(count, opts)
}

func (pg *PostGenerator) GenerateGamingPosts(count int) []types.Post {
	opts := PostOptions{
		Subreddit: "gaming",
		HasBody:   pg.rand.Float32() < 0.6,
		HasFlair:  true,
		MinScore:  50,
		MaxScore:  10000,
	}
	return pg.GeneratePostsWithOptions(count, opts)
}

func (pg *PostGenerator) GenerateAskRedditPosts(count int) []types.Post {
	opts := PostOptions{
		Subreddit:   "askreddit",
		HasBody:     false,
		MinScore:    100,
		MaxScore:    50000,
		MinComments: 100,
		MaxComments: 10000,
	}
	return pg.GeneratePostsWithOptions(count, opts)
}
