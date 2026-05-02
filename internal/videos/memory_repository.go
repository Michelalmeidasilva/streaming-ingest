package videos

import (
	"context"
	"strings"
	"sync"
)

type MemoryRepository struct {
	mu     sync.RWMutex
	videos []Video
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		videos: make([]Video, 0),
	}
}

func (r *MemoryRepository) Save(ctx context.Context, video *Video) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.videos {
		if r.videos[i].VideoID == video.VideoID {
			r.videos[i] = *video
			return nil
		}
	}

	r.videos = append(r.videos, *video)
	return nil
}

func (r *MemoryRepository) ListAll(ctx context.Context) ([]Video, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Video, len(r.videos))
	copy(out, r.videos)
	return out, nil
}

func (r *MemoryRepository) Search(ctx context.Context, query string) ([]Video, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query = strings.ToLower(query)
	out := make([]Video, 0)
	for _, video := range r.videos {
		if strings.Contains(strings.ToLower(video.Filename), query) {
			out = append(out, video)
		}
	}

	return out, nil
}

var _ VideoRepository = (*MemoryRepository)(nil)
