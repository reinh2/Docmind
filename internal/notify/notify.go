package notify

import (
	"context"
	"log/slog"
)

type Escalation struct{ ID, ConversationID, Reason string }
type Notifier interface {
	NotifyEscalation(context.Context, Escalation) error
}
type LogNotifier struct{ Log *slog.Logger }

func (n LogNotifier) NotifyEscalation(_ context.Context, event Escalation) error {
	n.Log.Warn("operator escalation requested", "escalation_id", event.ID, "conversation_id", event.ConversationID, "reason", event.Reason)
	return nil
}
