package models

import (
	"time"

	"gorm.io/gorm"
)

// Image represents a Docker image tracked across the cluster
type Image struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Image identification
	Name       string `gorm:"index;not null" json:"name"`            // e.g., nginx, redis, myapp
	Repository string `gorm:"index;not null" json:"repository"`      // e.g., docker.io, gcr.io
	FullName   string `gorm:"uniqueIndex;not null" json:"full_name"` // e.g., docker.io/nginx

	// Relationship
	ImageTags []ImageTag `gorm:"foreignKey:ImageID" json:"image_tags,omitempty"`
}

// ImageTag represents a specific tag version of an image used in the cluster
type ImageTag struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Foreign key
	ImageID uint  `gorm:"index;not null" json:"image_id"`
	Image   Image `gorm:"constraint:OnDelete:CASCADE;" json:"image,omitempty"`

	// Tag information
	Tag           string    `gorm:"index;not null" json:"tag"`           // e.g., latest, v1.2.3
	FirstSeen     time.Time `gorm:"not null" json:"first_seen"`          // When first detected
	LastSeen      time.Time `gorm:"not null" json:"last_seen"`           // When last detected
	ResourceType  string    `gorm:"index;not null" json:"resource_type"` // Deployment, DaemonSet, CronJob
	ResourceName  string    `gorm:"index;not null" json:"resource_name"` // Name of the resource
	Namespace     string    `gorm:"index;not null" json:"namespace"`     // Kubernetes namespace
	ContainerName string    `gorm:"not null" json:"container_name"`      // Container name within the pod

	// Composite unique index to prevent duplicates
	// Same image tag can exist multiple times if used in different resources/containers
}

// TableName overrides the table name
func (Image) TableName() string {
	return "images"
}

// TableName overrides the table name
func (ImageTag) TableName() string {
	return "image_tags"
}

// ImageInfo represents a container image with its metadata (API response)
type ImageInfo struct {
	Name         string   `json:"name"`
	Tag          string   `json:"tag"`
	ResourceType string   `json:"resourceType"` // deployment, cronjob, daemonset
	ResourceName string   `json:"resourceName"`
	Namespace    string   `json:"namespace"`
	Containers   []string `json:"containers"` // container names using this image
	FirstSeen    string   `json:"first_seen"`
	LastSeen     string   `json:"last_seen"`
}

// ImagesResponse represents the API response
type ImagesResponse struct {
	Images []ImageInfo `json:"images"`
	Total  int         `json:"total"`
}

// ImageTagHistory represents the history of tags for a specific image
type ImageTagHistory struct {
	ImageName string            `json:"image_name"`
	Tags      []ImageTagDetails `json:"tags"`
}

// ImageTagDetails provides detailed information about a specific tag
type ImageTagDetails struct {
	Tag          string    `json:"tag"`
	FirstSeen    time.Time `json:"first_seen"`
	LastSeen     time.Time `json:"last_seen"`
	ResourceType string    `json:"resource_type"`
	ResourceName string    `json:"resource_name"`
	Namespace    string    `json:"namespace"`
	Container    string    `json:"container"`
	Active       bool      `json:"active"` // Currently in use
}
