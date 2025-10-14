package test_generators

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// CommentGenerator generates realistic Reddit comments for testing
type CommentGenerator struct {
	rand             *rand.Rand
	commentTemplates []string
	replies          []string
	users            []string
}

// NewCommentGenerator creates a new comment generator
func NewCommentGenerator(seed int64) *CommentGenerator {
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	return &CommentGenerator{
		rand: rand.New(rand.NewSource(seed)),
		commentTemplates: []string{
			"I completely agree with %s. This is exactly what I was thinking.",
			"Actually, %s is not entirely accurate. Let me explain...",
			"Great point about %s! I've had a similar experience.",
			"I disagree with %s. Here's my perspective...",
			"This reminds me of %s. Has anyone else noticed this?",
			"Can someone elaborate on %s? I'm not sure I understand.",
			"Excellent analysis of %s. Very well said.",
			"I think you're missing the point about %s. It's more about...",
			"This is so true! %s has been on my mind lately.",
			"Counterpoint: %s. What do you all think?",
		},
		replies: []string{
			"You're absolutely right!",
			"I see what you mean, but...",
			"Interesting perspective!",
			"I hadn't thought of it that way.",
			"Thanks for explaining!",
			"Could you provide more details?",
			"I respectfully disagree.",
			"This makes a lot of sense.",
			"I'm not convinced by this argument.",
			"Great contribution to the discussion!",
		},
		users: []string{
			"thoughtful_commenter", "expert_analyst", "casual_observer", "debate_enthusiast",
			"helpful_explainer", "skeptic_user", "supportive_member", "critical_thinker",
			"experienced_redditor", "new_participant", "moderator_assistant", "community_builder",
		},
	}
}

// GenerateComment creates a realistic Reddit comment
func (cg *CommentGenerator) GenerateComment() types.Comment {
	now := time.Now()
	createdTime := now.Add(-time.Duration(cg.rand.Intn(86400)) * time.Second) // Random time in last 24h
	createdUTC := float64(createdTime.Unix())

	comment := types.Comment{
		ThingData: types.ThingData{
			ID:   cg.generateCommentID(),
			Name: "t1_" + cg.generateCommentID(),
		},
		Votable: types.Votable{
			Score: cg.generateCommentScore(),
		},
		Created: types.Created{
			Created:    createdUTC,
			CreatedUTC: createdUTC,
		},
		Author:    cg.randElement(cg.users),
		Body:      cg.generateCommentBody(),
		Edited:    cg.generateEditedTime(createdTime),
		ParentID:  cg.generateParentID(),
		LinkID:    cg.generatePostID(),
		Subreddit: cg.randElement([]string{"programming", "gaming", "technology", "science", "askreddit"}),
		Replies:   make([]*types.Comment, 0),
	}

	// Add optional characteristics
	if cg.rand.Float32() < 0.08 { // 8% chance
		comment.Gilded = cg.rand.Intn(5) + 1
	}

	return comment
}

// GenerateCommentThread creates a nested comment thread
func (cg *CommentGenerator) GenerateCommentThread(maxDepth int, maxComments int) []types.Comment {
	if maxDepth <= 0 || maxComments <= 0 {
		return []types.Comment{}
	}

	rootComment := cg.GenerateComment()

	remainingComments := maxComments - 1
	if remainingComments <= 0 {
		return []types.Comment{rootComment}
	}

	// Generate replies to create thread structure
	replies := cg.generateReplies(&rootComment, 1, maxDepth, remainingComments)
	rootComment.Replies = replies

	return []types.Comment{rootComment}
}

// GenerateCommentThreadsWithOptions creates multiple comment threads with specific characteristics
func (cg *CommentGenerator) GenerateCommentThreadsWithOptions(threadCount, maxDepth, maxComments int, opts CommentThreadOptions) []types.Comment {
	var allComments []types.Comment

	for i := 0; i < threadCount; i++ {
		thread := cg.GenerateCommentThreadWithOptions(maxDepth, maxComments/threadCount, opts)
		allComments = append(allComments, thread...)
	}

	return allComments
}

// GenerateCommentThreadWithOptions creates a comment thread with specific options
func (cg *CommentGenerator) GenerateCommentThreadWithOptions(maxDepth, maxComments int, opts CommentThreadOptions) []types.Comment {
	if maxDepth <= 0 || maxComments <= 0 {
		return []types.Comment{}
	}

	rootComment := cg.GenerateCommentWithOptions(opts)

	remainingComments := maxComments - 1
	if remainingComments <= 0 {
		return []types.Comment{rootComment}
	}

	// Generate replies with options
	replies := cg.generateRepliesWithOptions(&rootComment, 1, maxDepth, remainingComments, opts)
	rootComment.Replies = replies

	return []types.Comment{rootComment}
}

// CommentThreadOptions controls comment thread generation characteristics
type CommentThreadOptions struct {
	MinScore      int
	MaxScore      int
	MinReplies    int
	MaxReplies    int
	HasEdited     bool
	HasAwards     bool
	Controversial bool
	Popular       bool
	Subreddit     string
	PostID        string
}

// GenerateCommentWithOptions creates a comment with specific options
func (cg *CommentGenerator) GenerateCommentWithOptions(opts CommentThreadOptions) types.Comment {
	comment := cg.GenerateComment()

	// Apply options
	if opts.MinScore > 0 && comment.Score < opts.MinScore {
		comment.Score = opts.MinScore + cg.rand.Intn(opts.MaxScore-opts.MinScore+1)
	}
	if opts.MaxScore > 0 && comment.Score > opts.MaxScore {
		comment.Score = opts.MinScore + cg.rand.Intn(opts.MaxScore-opts.MinScore+1)
	}

	if opts.Controversial {
		comment.Score = cg.rand.Intn(1000) + 100
	}

	if opts.Popular {
		comment.Score = cg.rand.Intn(5000) + 1000
		if cg.rand.Float32() < 0.3 {
			comment.Gilded = cg.rand.Intn(10) + 1
		}
	}

	if !opts.HasEdited {
		comment.Edited = types.Edited{IsEdited: false, Timestamp: 0}
	}

	if !opts.HasAwards {
		comment.Gilded = 0
	}

	if opts.Subreddit != "" {
		comment.Subreddit = opts.Subreddit
	}

	if opts.PostID != "" {
		comment.LinkID = opts.PostID
	}

	return comment
}

// GenerateReplies generates replies for a comment
func (cg *CommentGenerator) generateReplies(parent *types.Comment, currentDepth, maxDepth, remainingComments int) []*types.Comment {
	if currentDepth > maxDepth || remainingComments <= 0 {
		return []*types.Comment{}
	}

	replyCount := cg.rand.Intn(3) + 1 // 1-3 replies per comment
	if replyCount > remainingComments {
		replyCount = remainingComments
	}

	var replies []*types.Comment
	usedComments := 0

	for i := 0; i < replyCount && usedComments < remainingComments; i++ {
		reply := cg.GenerateComment()
		reply.ParentID = parent.ID
		reply.LinkID = parent.LinkID
		reply.Subreddit = parent.Subreddit

		// Generate nested replies
		nestedRemaining := remainingComments - usedComments - 1
		if nestedRemaining > 0 && currentDepth < maxDepth {
			nestedReplies := cg.generateReplies(&reply, currentDepth+1, maxDepth, nestedRemaining)
			reply.Replies = nestedReplies
			usedComments += len(nestedReplies)
		}

		replies = append(replies, &reply)
		usedComments++
	}

	return replies
}

// GenerateRepliesWithOptions generates replies with specific options
func (cg *CommentGenerator) generateRepliesWithOptions(parent *types.Comment, currentDepth, maxDepth, remainingComments int, opts CommentThreadOptions) []*types.Comment {
	if currentDepth > maxDepth || remainingComments <= 0 {
		return []*types.Comment{}
	}

	replyCount := opts.MinReplies + cg.rand.Intn(opts.MaxReplies-opts.MinReplies+1)
	if replyCount > remainingComments {
		replyCount = remainingComments
	}

	var replies []*types.Comment
	usedComments := 0

	for i := 0; i < replyCount && usedComments < remainingComments; i++ {
		reply := cg.GenerateCommentWithOptions(opts)
		reply.ParentID = parent.ID
		reply.LinkID = parent.LinkID
		reply.Subreddit = parent.Subreddit

		// Generate nested replies
		nestedRemaining := remainingComments - usedComments - 1
		if nestedRemaining > 0 && currentDepth < maxDepth {
			nestedReplies := cg.generateRepliesWithOptions(&reply, currentDepth+1, maxDepth, nestedRemaining, opts)
			reply.Replies = nestedReplies
			usedComments += len(nestedReplies)
		}

		replies = append(replies, &reply)
		usedComments++
	}

	return replies
}

// GenerateControversialThread creates a controversial comment thread
func (cg *CommentGenerator) GenerateControversialThread(maxDepth, maxComments int) []types.Comment {
	opts := CommentThreadOptions{
		Controversial: true,
		MinScore:      10,
		MaxScore:      1000,
		MinReplies:    2,
		MaxReplies:    5,
	}

	return cg.GenerateCommentThreadWithOptions(maxDepth, maxComments, opts)
}

// GeneratePopularThread creates a popular comment thread
func (cg *CommentGenerator) GeneratePopularThread(maxDepth, maxComments int) []types.Comment {
	opts := CommentThreadOptions{
		Popular:    true,
		MinScore:   100,
		MaxScore:   10000,
		MinReplies: 3,
		MaxReplies: 8,
		HasAwards:  true,
	}

	return cg.GenerateCommentThreadWithOptions(maxDepth, maxComments, opts)
}

// GenerateFlatComments creates a flat list of comments (no nesting)
func (cg *CommentGenerator) GenerateFlatComments(count int) []types.Comment {
	comments := make([]types.Comment, count)
	for i := 0; i < count; i++ {
		comments[i] = cg.GenerateComment()
	}
	return comments
}

// GenerateFlatCommentsWithOptions creates flat comments with specific options
func (cg *CommentGenerator) GenerateFlatCommentsWithOptions(count int, opts CommentThreadOptions) []types.Comment {
	comments := make([]types.Comment, count)
	for i := 0; i < count; i++ {
		comments[i] = cg.GenerateCommentWithOptions(opts)
	}
	return comments
}

// Helper methods

func (cg *CommentGenerator) generateCommentBody() string {
	if cg.rand.Float32() < 0.3 { // 30% chance of a short reply
		return cg.randElement(cg.replies)
	}

	template := cg.randElement(cg.commentTemplates)
	topic := cg.generateTopic()
	body := fmt.Sprintf(template, topic)

	// Add some random sentences
	if cg.rand.Float32() < 0.4 {
		body += " " + cg.generateSentence()
	}
	if cg.rand.Float32() < 0.2 {
		body += " " + cg.generateSentence()
	}

	// Add formatting
	if cg.rand.Float32() < 0.1 {
		body = "**" + body + "**" // Bold
	} else if cg.rand.Float32() < 0.1 {
		body = "*" + body + "*" // Italic
	}

	return body
}

func (cg *CommentGenerator) generateTopic() string {
	topics := []string{
		"this approach", "the methodology", "the conclusion", "the premise",
		"the argument", "the evidence", "the reasoning", "the logic",
		"the implementation", "the design", "the strategy", "the outcome",
		"the impact", "the consequences", "the benefits", "the drawbacks",
		"the alternatives", "the possibilities", "the limitations", "the potential",
	}
	return cg.randElement(topics)
}

func (cg *CommentGenerator) generateSentence() string {
	sentences := []string{
		"This is an important consideration.",
		"I think we need to look at this more carefully.",
		"Has anyone considered the implications?",
		"There might be a better way to approach this.",
		"I'm not sure this is the best solution.",
		"This deserves more attention.",
		"The context here is really important.",
		"We should think about the long-term effects.",
		"This changes my perspective on the issue.",
		"I appreciate you bringing this up.",
	}
	return cg.randElement(sentences)
}

func (cg *CommentGenerator) generateCommentScore() int {
	// Most comments have low scores, few have high scores
	rand := cg.rand.Float64()
	if rand < 0.8 {
		return cg.rand.Intn(50) - 10 // 80% of comments: -10 to 39
	} else if rand < 0.95 {
		return cg.rand.Intn(200) + 40 // 15% of comments: 40 to 239
	} else if rand < 0.99 {
		return cg.rand.Intn(1000) + 240 // 4% of comments: 240 to 1239
	} else {
		return cg.rand.Intn(5000) + 1240 // 1% of comments: 1240+
	}
}

func (cg *CommentGenerator) generateCommentID() string {
	return fmt.Sprintf("t1_%x", cg.rand.Int63())
}

func (cg *CommentGenerator) generateParentID() string {
	// 50% chance of being a top-level comment (parent is post)
	if cg.rand.Float32() < 0.5 {
		return fmt.Sprintf("t3_%x", cg.rand.Int63()) // Post ID
	}
	return fmt.Sprintf("t1_%x", cg.rand.Int63()) // Comment ID
}

func (cg *CommentGenerator) generatePostID() string {
	return fmt.Sprintf("t3_%x", cg.rand.Int63())
}

func (cg *CommentGenerator) generateEditedTime(created time.Time) types.Edited {
	if cg.rand.Float32() < 0.85 { // 85% chance not edited
		return types.Edited{IsEdited: false, Timestamp: 0}
	}

	// Edited sometime after creation
	editDelay := time.Duration(cg.rand.Intn(7200)) * time.Second // Within 2 hours
	editedTime := created.Add(editDelay)
	return types.Edited{IsEdited: true, Timestamp: float64(editedTime.Unix())}
}

func (cg *CommentGenerator) randElement(slice []string) string {
	return slice[cg.rand.Intn(len(slice))]
}

// Preset generators for common scenarios

func (cg *CommentGenerator) GenerateTechComments(count int) []types.Comment {
	opts := CommentThreadOptions{
		Subreddit: "programming",
		MinScore:  5,
		MaxScore:  1000,
		HasEdited: true,
	}
	return cg.GenerateFlatCommentsWithOptions(count, opts)
}

func (cg *CommentGenerator) GenerateGamingComments(count int) []types.Comment {
	opts := CommentThreadOptions{
		Subreddit: "gaming",
		MinScore:  1,
		MaxScore:  500,
		HasAwards: true,
	}
	return cg.GenerateFlatCommentsWithOptions(count, opts)
}

func (cg *CommentGenerator) GenerateAskRedditComments(count int) []types.Comment {
	opts := CommentThreadOptions{
		Subreddit:  "askreddit",
		MinScore:   10,
		MaxScore:   10000,
		MinReplies: 1,
		MaxReplies: 3,
	}

	var allComments []types.Comment
	for i := 0; i < count; i++ {
		thread := cg.GenerateCommentThreadWithOptions(2, 1, opts) // Max depth 2, 1 comment per thread
		allComments = append(allComments, thread...)
	}

	return allComments
}

// Utility functions for working with generated comments

// FlattenComments converts a nested comment structure to a flat list
func FlattenComments(comments []types.Comment) []types.Comment {
	var flat []types.Comment

	for _, comment := range comments {
		flat = append(flat, comment)
		if len(comment.Replies) > 0 {
			// Convert []*Comment to []Comment for recursion
			var replyComments []types.Comment
			for _, reply := range comment.Replies {
				if reply != nil {
					replyComments = append(replyComments, *reply)
				}
			}
			flat = append(flat, FlattenComments(replyComments)...)
		}
	}

	return flat
}

// CountComments counts total comments in a nested structure
func CountComments(comments []types.Comment) int {
	count := len(comments)
	for _, comment := range comments {
		if len(comment.Replies) > 0 {
			// Convert []*Comment to []Comment for recursion
			var replyComments []types.Comment
			for _, reply := range comment.Replies {
				if reply != nil {
					replyComments = append(replyComments, *reply)
				}
			}
			count += CountComments(replyComments)
		}
	}
	return count
}

// GetMaxDepth finds the maximum depth in a nested comment structure
func GetMaxDepth(comments []types.Comment) int {
	return getMaxDepthHelper(comments, 0)
}

func getMaxDepthHelper(comments []types.Comment, currentDepth int) int {
	if len(comments) == 0 {
		return currentDepth
	}

	maxDepth := currentDepth
	for _, comment := range comments {
		if len(comment.Replies) > 0 {
			// Convert []*Comment to []Comment for recursion
			var replyComments []types.Comment
			for _, reply := range comment.Replies {
				if reply != nil {
					replyComments = append(replyComments, *reply)
				}
			}
			replyDepth := getMaxDepthHelper(replyComments, currentDepth+1)
			if replyDepth > maxDepth {
				maxDepth = replyDepth
			}
		}
	}
	return maxDepth
}
