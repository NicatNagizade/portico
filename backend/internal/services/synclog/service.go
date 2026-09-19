package synclog

import (
	"errors"

	"github.com/portico/backend/internal/models"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("sync log not found")

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) List(syncJobID *uint) ([]models.SyncLog, error) {
	q := s.db.Order("id desc")
	if syncJobID != nil {
		q = q.Where("sync_job_id = ?", *syncJobID)
	}
	var items []models.SyncLog
	if err := q.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Service) Get(id uint) (*models.SyncLog, error) {
	var item models.SyncLog
	if err := s.db.First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &item, nil
}
