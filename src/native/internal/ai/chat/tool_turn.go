package chat

import (
	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/backend"
)

// endReply closes the streaming reply of a model turn that asked for tools: it keeps the text and attaches the calls.
func (s *Service) endReply(id string, calls []backend.ToolCall) {
	_ = s.mutate(id, func(c *Chat) {
		if index := streamingIndex(c); index >= 0 {
			c.Messages[index].Status = StatusComplete
			c.Messages[index].Calls = calls
		}
	})
}

// beginReply starts the streaming reply of the next model turn of a run.
func (s *Service) beginReply(id string) {
	_ = s.mutate(id, func(c *Chat) {
		c.Messages = append(c.Messages, Message{Author: AuthorAssistant, Status: StatusStreaming, At: s.opts.Clock.Now()})
	})
}

// startToolRun adds the activity row of a call that is about to run.
func (s *Service) startToolRun(id string, call agent.CallInfo) {
	_ = s.mutate(id, func(c *Chat) {
		c.Messages = append(c.Messages, Message{Author: AuthorTool, Status: StatusRunning, Tool: toolRunOf(call), At: s.opts.Clock.Now()})
	})
}

// setToolStatus moves the row of a call to status. A non-empty outcome replaces the row's line.
func (s *Service) setToolStatus(id, callID string, status Status, outcome string) {
	_ = s.mutate(id, func(c *Chat) {
		if index := toolIndex(c, callID); index >= 0 {
			c.Messages[index].Status = status
			if outcome != "" {
				c.Messages[index].Tool.Outcome = outcome
			}
		}
	})
}

// endToolRun settles the row of a call with how it ended and the text the model reads.
func (s *Service) endToolRun(id string, event agent.Event) {
	_ = s.mutate(id, func(c *Chat) {
		index := toolIndex(c, event.Call.ID)
		if index < 0 {
			c.Messages = append(c.Messages, Message{Author: AuthorTool, Tool: toolRunOf(event.Call), At: s.opts.Clock.Now()})
			index = len(c.Messages) - 1
		}
		c.Messages[index].Status = statusOfCall(event.Status)
		c.Messages[index].Text = event.Result
		c.Messages[index].Tool.Outcome = event.Outcome
	})
}

// toolRunOf is the transcript description of a call.
func toolRunOf(call agent.CallInfo) *ToolRun {
	return &ToolRun{CallID: call.ID, ParentID: call.ParentID, Name: call.Name, Label: call.Label, Input: call.Input}
}

// toolIndex returns the position of the tool entry of a call, or -1.
func toolIndex(c *Chat, callID string) int {
	for i := len(c.Messages) - 1; i >= 0; i-- {
		if tool := c.Messages[i].Tool; c.Messages[i].Author == AuthorTool && tool != nil && tool.CallID == callID {
			return i
		}
	}
	return -1
}

// statusOfCall maps how a call ended to the status of its transcript row.
func statusOfCall(status agent.CallStatus) Status {
	switch status {
	case agent.CallRan:
		return StatusComplete
	case agent.CallRefused:
		return StatusRefused
	case agent.CallCancelled:
		return StatusCancelled
	}
	return StatusFailed
}
