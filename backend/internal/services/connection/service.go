package connection

import (
	"encoding/json"
	"errors"

	"github.com/portico/backend/internal/models"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("connection not found")

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
		item.Config = datatypes.JSON(in.Config)
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
