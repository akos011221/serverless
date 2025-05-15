// This package manages persistent storage for the platform. It uses SQLite and GORM, handles function metadata,
// enabling the platform to register and retrieve details about the deployed functions.
package storage

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Function represents a deployed function.
type Function struct {
	gorm.Model
	Name    string `gorm:"unique"`
	Image   string
	Runtime string
}

// Store manages function metadata.
type Store struct {
	db  *gorm.DB
	log *logrus.Logger
}

// NewStore initializes the store.
func NewStore(dbPath string, log *logrus.Logger) (*Store, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Auto-migrate schema.
	if err := db.AutoMigrate(&Function{}); err != nil {
		return nil, fmt.Errorf("failed to migrate schema: %v", err)
	}

	return &Store{db: db, log: log}, nil
}

// CreateFunction stores a new function.
func (s *Store) CreateFunction(name, image, runtime string) error {
	function := Function{
		Name:    name,
		Image:   image,
		Runtime: runtime,
	}
	if err := s.db.Create(&function).Error; err != nil {
		return fmt.Errorf("failed to create function: %v", err)
	}
	s.log.WithField("function", name).Info("Function stored")
	return nil
}

// GetFunction retrieves a function by name.
func (s *Store) GetFunction(name string) (*Function, error) {
	var function Function
	if err := s.db.Where("name = ?", name).First(&function).Error; err != nil {
		return nil, fmt.Errorf("function not found: %v", err)
	}
	return &function, nil
}
