package channel

import (
	"context"
	"log"
	"strings"
)

type LogChannel struct {
	name string
}

func NewLogChannel(name string) LogChannel {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "log"
	}
	return LogChannel{name: name}
}

func (c LogChannel) Name() string {
	return c.name
}

func (c LogChannel) SendDraft(_ context.Context, msg DraftMessage) (DeliveryReceipt, error) {
	log.Printf("[channel:%s] draft ready run=%s provider=%s change=%s url=%s bytes=%d", c.name, msg.RunID, msg.Provider, msg.ChangeID, msg.WebURL, len(msg.DraftReport))
	return DeliveryReceipt{Channel: c.name, ExternalID: "log:" + msg.RunID}, nil
}

func (c LogChannel) SendFinalConfirmation(_ context.Context, msg FinalConfirmationMessage) error {
	log.Printf("[channel:%s] final published run=%s bytes=%d", c.name, msg.RunID, len(msg.FinalReport))
	return nil
}
