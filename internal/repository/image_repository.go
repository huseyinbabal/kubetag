package repository

import (
	"fmt"
	"time"

	"github.com/huseyinbabal/kubetag/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ImageRepository handles database operations for images
type ImageRepository struct {
	db *gorm.DB
}

// NewImageRepository creates a new image repository
func NewImageRepository(db *gorm.DB) *ImageRepository {
	return &ImageRepository{db: db}
}

// UpsertImageTag creates or updates an image tag record
func (r *ImageRepository) UpsertImageTag(
	imageName, repository, tag, resourceType, resourceName, namespace, containerName string,
) error {
	// First, get or create the image
	fullName := fmt.Sprintf("%s/%s", repository, imageName)

	var image models.Image
	err := r.db.Where("full_name = ?", fullName).FirstOrCreate(&image, models.Image{
		Name:       imageName,
		Repository: repository,
		FullName:   fullName,
	}).Error

	if err != nil {
		return fmt.Errorf("failed to upsert image: %w", err)
	}

	// Now upsert the image tag
	now := time.Now().UTC()

	imageTag := models.ImageTag{
		ImageID:       image.ID,
		Tag:           tag,
		ResourceType:  resourceType,
		ResourceName:  resourceName,
		Namespace:     namespace,
		ContainerName: containerName,
		FirstSeen:     now,
		LastSeen:      now,
	}

	// Use ON CONFLICT to update LastSeen if record exists
	err = r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "image_id"},
			{Name: "tag"},
			{Name: "resource_type"},
			{Name: "resource_name"},
			{Name: "namespace"},
			{Name: "container_name"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"last_seen", "updated_at"}),
	}).Create(&imageTag).Error

	if err != nil {
		return fmt.Errorf("failed to upsert image tag: %w", err)
	}

	return nil
}

// DeleteImageTag soft deletes an image tag
func (r *ImageRepository) DeleteImageTag(
	resourceType, resourceName, namespace string,
) error {
	return r.db.Where(
		"resource_type = ? AND resource_name = ? AND namespace = ?",
		resourceType, resourceName, namespace,
	).Delete(&models.ImageTag{}).Error
}

// GetAllImages returns all active images grouped by image name, showing only the latest tag per resource
func (r *ImageRepository) GetAllImages(namespace string) ([]models.ImageInfo, error) {
	var imageTags []models.ImageTag

	query := r.db.Preload("Image").Where("deleted_at IS NULL")

	if namespace != "" {
		query = query.Where("namespace = ?", namespace)
	}

	if err := query.Find(&imageTags).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch image tags: %w", err)
	}

	// Group by image name+resource to find the latest tag
	// Key: image_name|resource_type|resource_name|namespace
	latestTagMap := make(map[string]*models.ImageTag)

	for i := range imageTags {
		it := &imageTags[i]
		resourceKey := fmt.Sprintf("%s|%s|%s|%s",
			it.Image.Name, it.ResourceType, it.ResourceName, it.Namespace)

		if existing, found := latestTagMap[resourceKey]; found {
			// Keep the tag with the most recent LastSeen
			if it.LastSeen.After(existing.LastSeen) {
				latestTagMap[resourceKey] = it
			}
		} else {
			latestTagMap[resourceKey] = it
		}
	}

	// Now aggregate containers for each unique image+tag+resource combination
	imageMap := make(map[string]*models.ImageInfo)

	for _, it := range latestTagMap {
		key := fmt.Sprintf("%s|%s|%s|%s|%s",
			it.Image.Name, it.Tag, it.ResourceType, it.ResourceName, it.Namespace)

		if existing, found := imageMap[key]; found {
			existing.Containers = append(existing.Containers, it.ContainerName)
		} else {
			imageMap[key] = &models.ImageInfo{
				Name:         it.Image.Name,
				Tag:          it.Tag,
				ResourceType: it.ResourceType,
				ResourceName: it.ResourceName,
				Namespace:    it.Namespace,
				Containers:   []string{it.ContainerName},
				FirstSeen:    it.FirstSeen.Format(time.RFC3339),
				LastSeen:     it.LastSeen.Format(time.RFC3339),
			}
		}
	}

	// Convert map to slice
	var result []models.ImageInfo
	for _, img := range imageMap {
		result = append(result, *img)
	}

	return result, nil
}

// GetImageTagHistory returns the history of all tags for a specific image
func (r *ImageRepository) GetImageTagHistory(imageName, namespace string) (*models.ImageTagHistory, error) {
	var image models.Image

	// Find image by name (could be from multiple repositories)
	if err := r.db.Where("name = ?", imageName).First(&image).Error; err != nil {
		return nil, fmt.Errorf("image not found: %w", err)
	}

	var imageTags []models.ImageTag
	query := r.db.Unscoped().Where("image_id = ?", image.ID)

	if namespace != "" {
		query = query.Where("namespace = ?", namespace)
	}

	if err := query.Order("first_seen DESC").Find(&imageTags).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch image tag history: %w", err)
	}

	// Group by tag to find which tags are currently active
	// A tag is active if it has at least one non-deleted record
	tagActiveMap := make(map[string]bool)
	for _, it := range imageTags {
		if it.DeletedAt.Time.IsZero() {
			tagActiveMap[it.Tag] = true
		}
	}

	// Convert to response format
	var tagDetails []models.ImageTagDetails
	for _, it := range imageTags {
		tagDetails = append(tagDetails, models.ImageTagDetails{
			Tag:          it.Tag,
			FirstSeen:    it.FirstSeen,
			LastSeen:     it.LastSeen,
			ResourceType: it.ResourceType,
			ResourceName: it.ResourceName,
			Namespace:    it.Namespace,
			Container:    it.ContainerName,
			Active:       tagActiveMap[it.Tag], // Active if any instance of this tag is not deleted
		})
	}

	return &models.ImageTagHistory{
		ImageName: imageName,
		Tags:      tagDetails,
	}, nil
}
