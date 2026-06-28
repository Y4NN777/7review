package app

import "time"

const deliveryRetention = 24 * time.Hour

func (s *Server) claimDelivery(key string) bool {
	if key == "" {
		return true
	}
	s.seenMu.Lock()
	defer s.seenMu.Unlock()
	if s.seen == nil {
		s.seen = make(map[string]time.Time)
	}
	s.purgeExpiredDeliveriesLocked(time.Now().UTC())
	if _, ok := s.seen[key]; ok {
		return false
	}
	s.seen[key] = time.Now().UTC()
	return true
}

func (s *Server) releaseDelivery(key string) {
	if key == "" {
		return
	}
	s.seenMu.Lock()
	defer s.seenMu.Unlock()
	delete(s.seen, key)
}

func (s *Server) purgeExpiredDeliveriesLocked(now time.Time) {
	for key, seenAt := range s.seen {
		if now.Sub(seenAt) > deliveryRetention {
			delete(s.seen, key)
		}
	}
}
