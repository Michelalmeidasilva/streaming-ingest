package sessions

import "context"

// Service provides business logic for benchmark sessions.
type Service struct {
	repo SessionRepository
}

// NewService constructs a Service backed by the given repository.
func NewService(repo SessionRepository) *Service {
	return &Service{repo: repo}
}

// CreateSession persists a new launched-session record.
func (s *Service) CreateSession(ctx context.Context, sess Session) error {
	return s.repo.Insert(ctx, sess)
}
