package shutdowns

import (
	"sync"

	"github.com/Roma7-7-7/sso-notifier/models"
	"go.uber.org/zap"
)

const shutdownsTableKey = "table"

type TableLoader func() (models.ShutdownsTable, error)

type Repository interface {
	Get(key string) (models.ShutdownsTable, bool, error)
	Put(models.ShutdownsTable) (models.ShutdownsTable, error)
}

type Service struct {
	repo   Repository
	loader TableLoader

	refreshMx sync.Mutex
}

func (s *Service) GetShutdownsTable() (models.ShutdownsTable, bool, error) {
	return s.repo.Get(shutdownsTableKey)
}

func (s *Service) RefreshShutdownsTable() {
	s.refreshMx.Lock()
	defer s.refreshMx.Unlock()

	table, err := s.loader()
	if err != nil {
		zap.L().Error("failed to parse page", zap.Error(err))
		return
	}
	table.ID = shutdownsTableKey
	if _, err = s.repo.Put(table); err != nil {
		zap.L().Error("failed to update shutdowns table", zap.Error(err))
		return
	}
}

func NewShutdownsService(repo Repository, loader TableLoader) *Service {
	return &Service{
		repo:   repo,
		loader: loader,
	}
}
