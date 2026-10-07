package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/portico/backend/internal/secretbox"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	ConnectionTypeMySQL     = "mysql"
	ConnectionTypePostgres  = "postgres"
	ConnectionTypeTypesense = "typesense"
	ConnectionTypeMongoDB   = "mongodb"
	ConnectionTypeSQLite    = "sqlite"
	ConnectionTypeRedis     = "redis"
)

type Connection struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Name      string         `json:"name" gorm:"size:255;not null"`
	Type      string         `json:"type" gorm:"size:50;not null;index"`
	Config    datatypes.JSON `json:"config" gorm:"type:json;not null" swaggertype:"object"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// BeforeSave encrypts secret config fields so the connections table does not store them in plaintext.
func (c *Connection) BeforeSave(*gorm.DB) error {
	sealed, err := secretbox.Seal(c.Config)
	if err != nil {
		return fmt.Errorf("seal connection config: %w", err)
	}
	c.Config = sealed
	return nil
}

// AfterFind restores secret config fields for connectors. API responses redact them in MarshalJSON.
func (c *Connection) AfterFind(*gorm.DB) error {
	return c.openConfig()
}

// AfterSave restores the in-memory config after the sealed copy is written.
func (c *Connection) AfterSave(*gorm.DB) error {
	return c.openConfig()
}

func (c *Connection) openConfig() error {
	opened, err := secretbox.Open(c.Config)
	if err != nil {
		return fmt.Errorf("open connection config: %w", err)
	}
	c.Config = opened
	return nil
}

// MarshalJSON omits secret config values from API responses.
func (c Connection) MarshalJSON() ([]byte, error) {
	redacted, err := secretbox.Redact(c.Config)
	if err != nil {
		return nil, err
	}
	type alias Connection
	out := alias(c)
	out.Config = redacted
	return json.Marshal(out)
}
