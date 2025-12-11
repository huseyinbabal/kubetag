package service

import (
	"context"
	"fmt"
	"log"

	"github.com/huseyinbabal/kubetag/internal/models"
	"github.com/huseyinbabal/kubetag/internal/repository"
	"github.com/huseyinbabal/kubetag/pkg/k8s"
)

// ImageService handles business logic for image operations
type ImageService struct {
	repo            *repository.ImageRepository
	informerManager *k8s.InformerManager
}

// NewImageService creates a new image service
func NewImageService(repo *repository.ImageRepository, informerManager *k8s.InformerManager) *ImageService {
	return &ImageService{
		repo:            repo,
		informerManager: informerManager,
	}
}

// HandleImageEvent processes image events from Kubernetes informers
func (s *ImageService) HandleImageEvent(event k8s.ImageEvent) {
	log.Printf("Image event: %s - %s/%s:%s in %s/%s/%s",
		event.Type,
		event.Repository,
		event.ImageName,
		event.ImageTag,
		event.Namespace,
		event.ResourceType,
		event.ResourceName,
	)

	switch event.Type {
	case k8s.EventTypeAdd, k8s.EventTypeUpdate:
		err := s.repo.UpsertImageTag(
			event.ImageName,
			event.Repository,
			event.ImageTag,
			event.ResourceType,
			event.ResourceName,
			event.Namespace,
			event.ContainerName,
		)
		if err != nil {
			log.Printf("Error upserting image tag: %v", err)
		}

	case k8s.EventTypeDelete:
		err := s.repo.DeleteImageTag(
			event.ResourceType,
			event.ResourceName,
			event.Namespace,
		)
		if err != nil {
			log.Printf("Error deleting image tag: %v", err)
		}
	}
}

// GetImages retrieves all images from the database
func (s *ImageService) GetImages(ctx context.Context, namespace string) (*models.ImagesResponse, error) {
	images, err := s.repo.GetAllImages(namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get images: %w", err)
	}

	return &models.ImagesResponse{
		Images: images,
		Total:  len(images),
	}, nil
}

// GetImageTagHistory retrieves the tag history for a specific image
func (s *ImageService) GetImageTagHistory(ctx context.Context, imageName, namespace string) (*models.ImageTagHistory, error) {
	history, err := s.repo.GetImageTagHistory(imageName, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get image tag history: %w", err)
	}

	return history, nil
}
