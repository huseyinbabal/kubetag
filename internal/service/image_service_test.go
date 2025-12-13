package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/huseyinbabal/kubetag/internal/k8s"
	"github.com/huseyinbabal/kubetag/internal/mocks"
	"github.com/huseyinbabal/kubetag/internal/models"
)

func TestNewImageService(t *testing.T) {
	t.Run("should create service with dependencies", func(t *testing.T) {
		mockRepo := mocks.NewMockImageRepository(t)
		mockInformer := &k8s.InformerManager{}

		service := NewImageService(mockRepo, mockInformer)

		if service == nil {
			t.Fatal("Expected non-nil service")
		}
		if service.repo == nil {
			t.Error("Service repo should not be nil")
		}
		if service.informerManager == nil {
			t.Error("Service informerManager should not be nil")
		}
	})
}

func TestGetImages(t *testing.T) {
	tests := []struct {
		name          string
		namespace     string
		mockResponse  []models.ImageInfo
		mockError     error
		expectedTotal int
		expectedError bool
	}{
		{
			name:      "successfully get images without namespace",
			namespace: "",
			mockResponse: []models.ImageInfo{
				{
					Name:         "nginx",
					Tag:          "latest",
					ResourceType: "Deployment",
					ResourceName: "nginx-deployment",
					Namespace:    "default",
					Containers:   []string{"nginx"},
					FirstSeen:    "2024-01-01T00:00:00Z",
					LastSeen:     "2024-01-02T00:00:00Z",
				},
				{
					Name:         "redis",
					Tag:          "7.0",
					ResourceType: "Deployment",
					ResourceName: "redis-deployment",
					Namespace:    "default",
					Containers:   []string{"redis"},
					FirstSeen:    "2024-01-01T00:00:00Z",
					LastSeen:     "2024-01-02T00:00:00Z",
				},
			},
			mockError:     nil,
			expectedTotal: 2,
			expectedError: false,
		},
		{
			name:      "successfully get images with namespace filter",
			namespace: "production",
			mockResponse: []models.ImageInfo{
				{
					Name:         "api",
					Tag:          "v1.0.0",
					ResourceType: "Deployment",
					ResourceName: "api-deployment",
					Namespace:    "production",
					Containers:   []string{"api"},
					FirstSeen:    "2024-01-01T00:00:00Z",
					LastSeen:     "2024-01-02T00:00:00Z",
				},
			},
			mockError:     nil,
			expectedTotal: 1,
			expectedError: false,
		},
		{
			name:          "repository returns error",
			namespace:     "",
			mockResponse:  nil,
			mockError:     errors.New("database connection failed"),
			expectedTotal: 0,
			expectedError: true,
		},
		{
			name:          "empty result",
			namespace:     "non-existent",
			mockResponse:  []models.ImageInfo{},
			mockError:     nil,
			expectedTotal: 0,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockImageRepository(t)

			// Setup expectations
			mockRepo.EXPECT().
				GetAllImages(tt.namespace).
				Return(tt.mockResponse, tt.mockError).
				Once()

			service := NewImageService(mockRepo, nil)
			ctx := context.Background()

			// Execute
			result, err := service.GetImages(ctx, tt.namespace)

			// Verify error
			if tt.expectedError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if result != nil {
					t.Error("Expected nil result on error")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if result == nil {
					t.Fatal("Expected non-nil result")
				}

				// Verify result
				if result.Total != tt.expectedTotal {
					t.Errorf("Expected total %d, got %d", tt.expectedTotal, result.Total)
				}
				if len(result.Images) != tt.expectedTotal {
					t.Errorf("Expected %d images, got %d", tt.expectedTotal, len(result.Images))
				}

				// Verify image details
				for i, img := range result.Images {
					expectedImg := tt.mockResponse[i]
					if img.Name != expectedImg.Name {
						t.Errorf("Image[%d]: Expected name %q, got %q", i, expectedImg.Name, img.Name)
					}
					if img.Tag != expectedImg.Tag {
						t.Errorf("Image[%d]: Expected tag %q, got %q", i, expectedImg.Tag, img.Tag)
					}
					if img.Namespace != expectedImg.Namespace {
						t.Errorf("Image[%d]: Expected namespace %q, got %q", i, expectedImg.Namespace, img.Namespace)
					}
				}
			}

			// Verify all expectations were met
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestGetImageTagHistory(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name          string
		imageName     string
		namespace     string
		mockResponse  *models.ImageTagHistory
		mockError     error
		expectedError bool
		expectedTags  int
	}{
		{
			name:      "successfully get history without namespace",
			imageName: "nginx",
			namespace: "",
			mockResponse: &models.ImageTagHistory{
				ImageName: "nginx",
				Tags: []models.ImageTagDetails{
					{
						Tag:          "latest",
						FirstSeen:    now.Add(-24 * time.Hour),
						LastSeen:     now,
						ResourceType: "Deployment",
						ResourceName: "nginx-deployment",
						Namespace:    "default",
						Container:    "nginx",
						Active:       true,
					},
					{
						Tag:          "1.21",
						FirstSeen:    now.Add(-48 * time.Hour),
						LastSeen:     now.Add(-24 * time.Hour),
						ResourceType: "Deployment",
						ResourceName: "nginx-deployment",
						Namespace:    "default",
						Container:    "nginx",
						Active:       false,
					},
				},
			},
			mockError:     nil,
			expectedError: false,
			expectedTags:  2,
		},
		{
			name:      "successfully get history with namespace filter",
			imageName: "redis",
			namespace: "production",
			mockResponse: &models.ImageTagHistory{
				ImageName: "redis",
				Tags: []models.ImageTagDetails{
					{
						Tag:          "7.0",
						FirstSeen:    now,
						LastSeen:     now,
						ResourceType: "Deployment",
						ResourceName: "redis-deployment",
						Namespace:    "production",
						Container:    "redis",
						Active:       true,
					},
				},
			},
			mockError:     nil,
			expectedError: false,
			expectedTags:  1,
		},
		{
			name:          "repository returns error",
			imageName:     "nginx",
			namespace:     "",
			mockResponse:  nil,
			mockError:     errors.New("image not found"),
			expectedError: true,
			expectedTags:  0,
		},
		{
			name:      "image with no history",
			imageName: "unused",
			namespace: "",
			mockResponse: &models.ImageTagHistory{
				ImageName: "unused",
				Tags:      []models.ImageTagDetails{},
			},
			mockError:     nil,
			expectedError: false,
			expectedTags:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockImageRepository(t)

			// Setup expectations
			mockRepo.EXPECT().
				GetImageTagHistory(tt.imageName, tt.namespace).
				Return(tt.mockResponse, tt.mockError).
				Once()

			service := NewImageService(mockRepo, nil)
			ctx := context.Background()

			// Execute
			result, err := service.GetImageTagHistory(ctx, tt.imageName, tt.namespace)

			// Verify error
			if tt.expectedError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if result != nil {
					t.Error("Expected nil result on error")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if result == nil {
					t.Fatal("Expected non-nil result")
				}

				// Verify result
				if result.ImageName != tt.imageName {
					t.Errorf("Expected image name %q, got %q", tt.imageName, result.ImageName)
				}
				if len(result.Tags) != tt.expectedTags {
					t.Errorf("Expected %d tags, got %d", tt.expectedTags, len(result.Tags))
				}

				// Verify tag details
				for i, tag := range result.Tags {
					expectedTag := tt.mockResponse.Tags[i]
					if tag.Tag != expectedTag.Tag {
						t.Errorf("Tag[%d]: Expected tag %q, got %q", i, expectedTag.Tag, tag.Tag)
					}
					if tag.ResourceType != expectedTag.ResourceType {
						t.Errorf("Tag[%d]: Expected resource type %q, got %q", i, expectedTag.ResourceType, tag.ResourceType)
					}
					if tag.Active != expectedTag.Active {
						t.Errorf("Tag[%d]: Expected active %v, got %v", i, expectedTag.Active, tag.Active)
					}
				}
			}

			// Verify all expectations were met
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestHandleImageEvent(t *testing.T) {
	tests := []struct {
		name         string
		event        k8s.ImageEvent
		expectUpsert bool
		expectDelete bool
		upsertError  error
		deleteError  error
	}{
		{
			name: "add event calls upsert",
			event: k8s.ImageEvent{
				Type:          k8s.EventTypeAdd,
				ImageName:     "nginx",
				Repository:    "docker.io",
				ImageTag:      "latest",
				ResourceType:  "Deployment",
				ResourceName:  "nginx-deployment",
				Namespace:     "default",
				ContainerName: "nginx",
			},
			expectUpsert: true,
			expectDelete: false,
			upsertError:  nil,
		},
		{
			name: "update event calls upsert",
			event: k8s.ImageEvent{
				Type:          k8s.EventTypeUpdate,
				ImageName:     "redis",
				Repository:    "docker.io",
				ImageTag:      "7.0",
				ResourceType:  "Deployment",
				ResourceName:  "redis-deployment",
				Namespace:     "default",
				ContainerName: "redis",
			},
			expectUpsert: true,
			expectDelete: false,
			upsertError:  nil,
		},
		{
			name: "delete event calls delete",
			event: k8s.ImageEvent{
				Type:         k8s.EventTypeDelete,
				ResourceType: "Deployment",
				ResourceName: "old-deployment",
				Namespace:    "default",
			},
			expectUpsert: false,
			expectDelete: true,
			deleteError:  nil,
		},
		{
			name: "upsert error is logged but does not panic",
			event: k8s.ImageEvent{
				Type:          k8s.EventTypeAdd,
				ImageName:     "nginx",
				Repository:    "docker.io",
				ImageTag:      "latest",
				ResourceType:  "Deployment",
				ResourceName:  "nginx-deployment",
				Namespace:     "default",
				ContainerName: "nginx",
			},
			expectUpsert: true,
			expectDelete: false,
			upsertError:  errors.New("database error"),
		},
		{
			name: "delete error is logged but does not panic",
			event: k8s.ImageEvent{
				Type:         k8s.EventTypeDelete,
				ResourceType: "Deployment",
				ResourceName: "old-deployment",
				Namespace:    "default",
			},
			expectUpsert: false,
			expectDelete: true,
			deleteError:  errors.New("database error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mocks.NewMockImageRepository(t)

			// Setup expectations
			if tt.expectUpsert {
				mockRepo.EXPECT().
					UpsertImageTag(
						tt.event.ImageName,
						tt.event.Repository,
						tt.event.ImageTag,
						tt.event.ResourceType,
						tt.event.ResourceName,
						tt.event.Namespace,
						tt.event.ContainerName,
					).
					Return(tt.upsertError).
					Once()
			}

			if tt.expectDelete {
				mockRepo.EXPECT().
					DeleteImageTag(
						tt.event.ResourceType,
						tt.event.ResourceName,
						tt.event.Namespace,
					).
					Return(tt.deleteError).
					Once()
			}

			service := NewImageService(mockRepo, nil)

			// Execute - should not panic even on errors
			service.HandleImageEvent(tt.event)

			// Verify all expectations were met
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestHandleImageEventWithUnknownEventType(t *testing.T) {
	t.Run("unknown event type does nothing", func(t *testing.T) {
		mockRepo := mocks.NewMockImageRepository(t)

		// No expectations set - nothing should be called
		event := k8s.ImageEvent{
			Type:         "UNKNOWN",
			ImageName:    "nginx",
			Repository:   "docker.io",
			ImageTag:     "latest",
			ResourceType: "Deployment",
			ResourceName: "nginx-deployment",
			Namespace:    "default",
		}

		service := NewImageService(mockRepo, nil)

		// Execute
		service.HandleImageEvent(event)

		// Verify no unexpected calls were made
		mockRepo.AssertExpectations(t)
	})
}

func TestGetImagesWithContextCancellation(t *testing.T) {
	t.Run("context cancellation does not affect repository call", func(t *testing.T) {
		mockRepo := mocks.NewMockImageRepository(t)

		// Setup expectations
		mockRepo.EXPECT().
			GetAllImages("").
			Return([]models.ImageInfo{}, nil).
			Once()

		service := NewImageService(mockRepo, nil)

		// Create cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// Execute - context is passed but repository doesn't use it
		result, err := service.GetImages(ctx, "")

		// Verify no error from cancelled context
		if err != nil {
			t.Errorf("Expected no error but got: %v", err)
		}
		if result == nil {
			t.Error("Expected non-nil result")
		}

		mockRepo.AssertExpectations(t)
	})
}

func TestGetImageTagHistoryEdgeCases(t *testing.T) {
	t.Run("image name with special characters", func(t *testing.T) {
		mockRepo := mocks.NewMockImageRepository(t)
		imageName := "my-app/special-image"

		mockRepo.EXPECT().
			GetImageTagHistory(imageName, "").
			Return(&models.ImageTagHistory{
				ImageName: imageName,
				Tags:      []models.ImageTagDetails{},
			}, nil).
			Once()

		service := NewImageService(mockRepo, nil)
		result, err := service.GetImageTagHistory(context.Background(), imageName, "")

		if err != nil {
			t.Errorf("Expected no error but got: %v", err)
		}
		if result.ImageName != imageName {
			t.Errorf("Expected image name %q, got %q", imageName, result.ImageName)
		}

		mockRepo.AssertExpectations(t)
	})
}
