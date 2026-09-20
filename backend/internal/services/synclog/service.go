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

func (s *Service) List(syncJobID *uint, page, pageSize int) ([]models.SyncLog, int64, error) {
	q := s.db.Model(&models.SyncLog{})
	if syncJobID != nil {
		q = q.Where("sync_job_id = ?", *syncJobID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.SyncLog
	if err := q.Preload("SyncJob").
		Order("id desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
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
