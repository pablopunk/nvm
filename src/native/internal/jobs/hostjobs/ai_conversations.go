package hostjobs

import (
	"context"
	"time"

	"github.com/pablopunk/nvm/native/internal/jobs"
)

// AI conversation cleanup removes the conversations idle past the retention window. It runs a second after launch and
// then hourly, as the desktop host job does.
const (
	AIConversationCleanupID      = "ai.conversations.cleanup"
	AIConversationCleanupEvery   = time.Hour
	AIConversationCleanupDelay   = time.Second
	AIConversationCleanupTimeout = 20 * time.Second
)

// AIConversationCleanup returns the job definition. sweep removes the expired conversations and returns the failure of
// a removal, which is the job's failure.
func AIConversationCleanup(sweep func(ctx context.Context) error) jobs.Definition {
	return jobs.Definition{
		ID:    AIConversationCleanupID,
		Title: "AI Conversation Cleanup",
		Owner: jobs.OwnerHost,
		Scope: "ai",
		Triggers: []jobs.Trigger{{
			Kind:  jobs.TriggerInterval,
			Every: AIConversationCleanupEvery,
			Delay: AIConversationCleanupDelay,
		}},
		Budget: jobs.Budget{Timeout: AIConversationCleanupTimeout},
		Run: func(ctx context.Context, _ jobs.Run) error {
			return sweep(ctx)
		},
	}
}
