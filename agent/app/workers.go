package app

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"
)

type workItem struct {
	name string
	run  func(context.Context) error
}

type workerStats struct {
	enqueued  atomic.Uint64
	completed atomic.Uint64
	failed    atomic.Uint64
}

func (s *Server) startWorkers() {
	workers := s.cfg.WebhookWorkers
	if workers <= 0 {
		workers = 1
	}
	for i := 0; i < workers; i++ {
		workerID := i + 1
		go func() {
			for item := range s.work {
				if err := s.runWorkItem(workerID, item); err != nil {
					s.stats.failed.Add(1)
					log.Printf("[worker %d] failed %s: %v", workerID, item.name, err)
				} else {
					s.stats.completed.Add(1)
				}
			}
		}()
	}
}

func (s *Server) runWorkItem(workerID int, item workItem) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.webhookJobTimeout())
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic processing %s: %v", item.name, recovered)
		}
	}()
	return item.run(ctx)
}

func (s *Server) webhookJobTimeout() time.Duration {
	if s != nil && s.cfg != nil && s.cfg.WebhookJobTimeout > 0 {
		return time.Duration(s.cfg.WebhookJobTimeout) * time.Millisecond
	}
	return defaultWebhookJobTimeout
}

func (s *Server) enqueue(item workItem) error {
	select {
	case s.work <- item:
		s.stats.enqueued.Add(1)
		return nil
	default:
		return fmt.Errorf("review queue is full")
	}
}
