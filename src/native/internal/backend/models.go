package backend

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

// activeModelTimeout bounds the active-model descriptor, which the backend answers from cache.
const activeModelTimeout = 10 * time.Second

// ActiveModelQuery selects the model role and the request size the backend should price.
type ActiveModelQuery struct {
	// Role is "smart" or "fast" or an extension role the backend defines. Empty means the backend default.
	Role string
	// Chars is the prompt size in characters, so the backend can say whether the account can afford it. Zero
	// omits it.
	Chars int
	// ForceFree asks for the free-tier route even when the account has paid credits.
	ForceFree bool
}

// CreditBalance is the account's credit pools.
type CreditBalance struct {
	// Paid is the paid credit balance.
	Paid int `json:"paid"`
	// Free is the free credit balance.
	Free int `json:"free"`
	// Total is Paid plus Free, less what is reserved for in-flight requests.
	Total int `json:"total"`
}

// ModelDescriptor is the active model the backend routes the account to. API names the wire format the
// route speaks; ChatStream only speaks openai-completions, so check SpeaksChatCompletions first.
type ModelDescriptor struct {
	// ID is the model identifier for display and logs.
	ID string `json:"id"`
	// Name is the display name.
	Name string `json:"name"`
	// ContextWindow is the context size in tokens.
	ContextWindow int `json:"contextWindow"`
	// MaxInputTokens bounds the prompt, when the route sets a tighter limit than the context window.
	MaxInputTokens int `json:"maxInputTokens"`
	// MaxTokens bounds the completion.
	MaxTokens int `json:"maxTokens"`
	// Reasoning reports whether the model reasons before answering.
	Reasoning bool `json:"reasoning"`
	// ThinkingLevel is the reasoning effort the route uses.
	ThinkingLevel string `json:"thinkingLevel"`
	// Input lists the accepted input modalities, such as "text" and "image".
	Input []string `json:"input"`
	// API names the wire format: openai-completions, anthropic-messages, openai-responses or google-generative-ai.
	API string `json:"api"`
	// Provider names the upstream provider.
	Provider string `json:"provider"`
	// BaseURL is the origin the wire format is sent to.
	BaseURL string `json:"baseUrl"`
	// ModelTier is "pro" or "free".
	ModelTier string `json:"modelTier"`
	// CreditKind is "paid" or "free": which pool pays for the route.
	CreditKind string `json:"creditKind"`
	// Credits is the account's balance when the backend included it.
	Credits *CreditBalance `json:"credits"`
	// Notice is "ok", "low" or "blocked".
	Notice string `json:"notice"`
	// NoticeReason explains a notice, such as free_spend_cap_reached.
	NoticeReason string `json:"noticeReason"`
	// CostEstimate is the estimated cost of the query in credits, when the backend priced it.
	CostEstimate *int `json:"costEstimate"`
}

// SpeaksChatCompletions reports whether ChatStream can use this route's wire format.
func (d ModelDescriptor) SpeaksChatCompletions() bool {
	return d.API == "openai-completions"
}

// ActiveModel returns the model the backend routes this account to for query, with the credit balance and the
// spend-cap notice. It is an idempotent GET on the route the desktop app uses.
func (c *Client) ActiveModel(ctx context.Context, query ActiveModelQuery) (ModelDescriptor, error) {
	values := url.Values{}
	if query.Role != "" {
		values.Set("model", query.Role)
	}
	if query.Chars > 0 {
		values.Set("chars", strconv.Itoa(query.Chars))
	}
	if query.ForceFree {
		values.Set("tier", "free")
	}
	var descriptor ModelDescriptor
	err := c.callJSON(ctx, requestSpec{
		method:     "GET",
		path:       pathActiveModel,
		query:      values,
		auth:       authRequired,
		timeout:    activeModelTimeout,
		idempotent: true,
	}, &descriptor)
	if err != nil {
		return ModelDescriptor{}, err
	}
	return descriptor, nil
}
