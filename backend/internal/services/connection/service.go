package connection

import (
	"encoding/json"
	"errors"

	"github.com/portico/backend/internal/models"
	"github.com/portico/backend/internal/secretbox"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var (
	ErrNotFound  = errors.New("connection not found")
	ErrAmbiguous = errors.New("multiple connections match name and type")
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

type CreateInput struct {
	Name   string          `json:"name" binding:"required"`
	Type   string          `json:"type" binding:"required"`
	Config json.RawMessage `json:"config" binding:"required" swaggertype:"object"`
}

type UpdateInput struct {
	Name   *string         `json:"name"`
	Type   *string         `json:"type"`
	Config json.RawMessage `json:"config" swaggertype:"object"`
}

// CheckInput verifies credentials without saving. Optional ID merges blank secrets from an existing connection.
type CheckInput struct {
	ID     *uint           `json:"id"`
	Type   string          `json:"type" binding:"required"`
	Config json.RawMessage `json:"config" binding:"required" swaggertype:"object"`
}

func (s *Service) List() ([]models.Connection, error) {
	var items []models.Connection
	if err := s.db.Order("id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Service) Get(id uint) (*models.Connection, error) {
	var item models.Connection
	if err := s.db.First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &item, nil
}

// FindByName returns the single connection with the given name.
// ErrNotFound if none; ErrAmbiguous if more than one.
func (s *Service) FindByName(name string) (*models.Connection, error) {
	var items []models.Connection
	if err := s.db.Where("name = ?", name).Find(&items).Error; err != nil {
		return nil, err
	}
	switch len(items) {
	case 0:
		return nil, ErrNotFound
	case 1:
		return &items[0], nil
	default:
		return nil, ErrAmbiguous
	}
}

// FindByNameAndType returns the single connection with the given name and type.
// ErrNotFound if none; ErrAmbiguous if more than one.
func (s *Service) FindByNameAndType(name, typ string) (*models.Connection, error) {
	var items []models.Connection
	if err := s.db.Where("name = ? AND type = ?", name, typ).Find(&items).Error; err != nil {
		return nil, err
	}
	switch len(items) {
	case 0:
		return nil, ErrNotFound
	case 1:
		return &items[0], nil
	default:
		return nil, ErrAmbiguous
	}
}

func (s *Service) Create(in CreateInput) (*models.Connection, error) {
	item := models.Connection{
		Name:   in.Name,
		Type:   in.Type,
		Config: datatypes.JSON(in.Config),
	}
	if err := s.db.Create(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) Update(id uint, in UpdateInput) (*models.Connection, error) {
	item, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		item.Name = *in.Name
	}
	if in.Type != nil {
		item.Type = *in.Type
	}
	if len(in.Config) > 0 {
		merged, err := secretbox.Merge(item.Config, in.Config)
		if err != nil {
			return nil, err
		}
		item.Config = merged
	}
	if err := s.db.Save(item).Error; err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) Delete(id uint) error {
	res := s.db.Delete(&models.Connection{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// PrepareCheck builds an in-memory connection for a connectivity check.
func (s *Service) PrepareCheck(in CheckInput) (*models.Connection, error) {
	cfg := in.Config
	if in.ID != nil {
		existing, err := s.Get(*in.ID)
		if err != nil {
			return nil, err
		}
		merged, err := secretbox.Merge(existing.Config, in.Config)
		if err != nil {
			return nil, err
		}
		cfg = merged
	}
	return &models.Connection{
		Type:   in.Type,
		Config: datatypes.JSON(cfg),
	}, nil
}
