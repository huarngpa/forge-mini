package app

import "time"

func (s *Service) SetNow(now func() time.Time) {
	s.now = now
}
