package uploadstate

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
)

type Repository interface {
	SaveState(ctx context.Context, state UploadState) error
	GetState(ctx context.Context, sessionID string) (*UploadState, error)
	DeleteSession(ctx context.Context, sessionID string) error
	SaveVideo(ctx context.Context, video Video) error
	GetVideo(ctx context.Context, videoID string) (*Video, error)
	ListVideos(ctx context.Context, query string) ([]Video, error)
	PatchVideo(ctx context.Context, videoID string, patch bson.M) (*Video, error)
	DeleteVideo(ctx context.Context, videoID string) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) SaveState(ctx context.Context, state UploadState) error {
	return s.repo.SaveState(ctx, state)
}

func (s *Service) GetState(ctx context.Context, sessionID string) (*UploadState, error) {
	return s.repo.GetState(ctx, sessionID)
}

func (s *Service) DeleteSession(ctx context.Context, sessionID string) error {
	return s.repo.DeleteSession(ctx, sessionID)
}

func (s *Service) SaveVideo(ctx context.Context, video Video) error {
	return s.repo.SaveVideo(ctx, video)
}

func (s *Service) GetVideo(ctx context.Context, videoID string) (*Video, error) {
	return s.repo.GetVideo(ctx, videoID)
}

func (s *Service) ListVideos(ctx context.Context, query string) ([]Video, error) {
	return s.repo.ListVideos(ctx, query)
}

func (s *Service) PatchVideo(ctx context.Context, videoID string, patch map[string]any) (*Video, error) {
	return s.repo.PatchVideo(ctx, videoID, bson.M(patch))
}

func (s *Service) DeleteVideo(ctx context.Context, videoID string) error {
	return s.repo.DeleteVideo(ctx, videoID)
}
