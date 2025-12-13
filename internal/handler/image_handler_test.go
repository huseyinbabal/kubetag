package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/huseyinbabal/kubetag/internal/mocks"
	"github.com/huseyinbabal/kubetag/internal/models"
	"github.com/stretchr/testify/mock"
)

func TestHealthCheck(t *testing.T) {
	// Health check doesn't depend on service, so we can pass nil
	handler := &ImageHandler{service: nil}

	app := fiber.New()
	app.Get("/health", handler.HealthCheck)

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)

	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var response map[string]string
	json.Unmarshal(body, &response)

	if response["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", response["status"])
	}
}

func TestNewImageHandler(t *testing.T) {
	t.Run("should create handler with service", func(t *testing.T) {
		// Create a mock service
		mockSvc := mocks.NewMockImageService(t)

		// Create handler
		handler := NewImageHandler(mockSvc)

		// Assertions
		if handler == nil {
			t.Fatal("Expected non-nil handler")
		}

		if handler.service == nil {
			t.Error("Handler service should not be nil")
		}
	})
}

func TestGetImages(t *testing.T) {
	tests := []struct {
		name           string
		namespace      string
		mockResponse   *models.ImagesResponse
		mockError      error
		expectedStatus int
		expectedImages int
	}{
		{
			name:      "successfully get images without namespace filter",
			namespace: "",
			mockResponse: &models.ImagesResponse{
				Images: []models.ImageInfo{
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
				Total: 2,
			},
			mockError:      nil,
			expectedStatus: fiber.StatusOK,
			expectedImages: 2,
		},
		{
			name:      "successfully get images with namespace filter",
			namespace: "default",
			mockResponse: &models.ImagesResponse{
				Images: []models.ImageInfo{
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
				},
				Total: 1,
			},
			mockError:      nil,
			expectedStatus: fiber.StatusOK,
			expectedImages: 1,
		},
		{
			name:           "service returns error",
			namespace:      "",
			mockResponse:   nil,
			mockError:      errors.New("database connection failed"),
			expectedStatus: fiber.StatusInternalServerError,
			expectedImages: 0,
		},
		{
			name:      "empty result",
			namespace: "non-existent",
			mockResponse: &models.ImagesResponse{
				Images: []models.ImageInfo{},
				Total:  0,
			},
			mockError:      nil,
			expectedStatus: fiber.StatusOK,
			expectedImages: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock service
			mockSvc := mocks.NewMockImageService(t)

			// Setup expectations using mockery's expecter pattern
			mockSvc.EXPECT().
				GetImages(mock.Anything, tt.namespace).
				Return(tt.mockResponse, tt.mockError).
				Once()

			// Create handler with mock service
			handler := NewImageHandler(mockSvc)

			// Setup Fiber app
			app := fiber.New()
			app.Get("/api/images", handler.GetImages)

			// Create request
			url := "/api/images"
			if tt.namespace != "" {
				url += "?namespace=" + tt.namespace
			}
			req := httptest.NewRequest("GET", url, nil)

			// Execute request
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}

			// Verify status code
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			// Parse response body
			body, _ := io.ReadAll(resp.Body)

			if tt.mockError != nil {
				// Verify error response
				var errResponse map[string]string
				json.Unmarshal(body, &errResponse)
				if errResponse["error"] == "" {
					t.Error("Expected error in response")
				}
				if errResponse["error"] != tt.mockError.Error() {
					t.Errorf("Expected error message %q, got %q", tt.mockError.Error(), errResponse["error"])
				}
			} else {
				// Verify success response
				var response models.ImagesResponse
				if err := json.Unmarshal(body, &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				if len(response.Images) != tt.expectedImages {
					t.Errorf("Expected %d images, got %d", tt.expectedImages, len(response.Images))
				}

				if response.Total != tt.expectedImages {
					t.Errorf("Expected total %d, got %d", tt.expectedImages, response.Total)
				}

				// Verify image details if present
				if tt.expectedImages > 0 && tt.mockResponse != nil {
					for i, img := range response.Images {
						expectedImg := tt.mockResponse.Images[i]
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
			}

			// Verify all expectations were met
			mockSvc.AssertExpectations(t)
		})
	}
}

func TestGetImageHistory(t *testing.T) {
	tests := []struct {
		name           string
		imageName      string
		namespace      string
		mockResponse   *models.ImageTagHistory
		mockError      error
		expectedStatus int
		shouldCallMock bool
	}{
		{
			name:      "successfully get image history",
			imageName: "nginx",
			namespace: "default",
			mockResponse: &models.ImageTagHistory{
				ImageName: "nginx",
				Tags: []models.ImageTagDetails{
					{
						Tag:          "latest",
						FirstSeen:    time.Now().Add(-24 * time.Hour),
						LastSeen:     time.Now(),
						ResourceType: "Deployment",
						ResourceName: "nginx-deployment",
						Namespace:    "default",
						Container:    "nginx",
						Active:       true,
					},
					{
						Tag:          "1.21",
						FirstSeen:    time.Now().Add(-48 * time.Hour),
						LastSeen:     time.Now().Add(-24 * time.Hour),
						ResourceType: "Deployment",
						ResourceName: "nginx-deployment",
						Namespace:    "default",
						Container:    "nginx",
						Active:       false,
					},
				},
			},
			mockError:      nil,
			expectedStatus: fiber.StatusOK,
			shouldCallMock: true,
		},
		{
			name:           "empty image name returns not found from router",
			imageName:      "",
			namespace:      "",
			mockResponse:   nil,
			mockError:      nil,
			expectedStatus: fiber.StatusNotFound,
			shouldCallMock: false,
		},
		{
			name:           "service returns error",
			imageName:      "nginx",
			namespace:      "default",
			mockResponse:   nil,
			mockError:      errors.New("database error"),
			expectedStatus: fiber.StatusInternalServerError,
			shouldCallMock: true,
		},
		{
			name:      "image with no history",
			imageName: "redis",
			namespace: "default",
			mockResponse: &models.ImageTagHistory{
				ImageName: "redis",
				Tags:      []models.ImageTagDetails{},
			},
			mockError:      nil,
			expectedStatus: fiber.StatusOK,
			shouldCallMock: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock service
			mockSvc := mocks.NewMockImageService(t)

			// Setup expectations only if the mock should be called
			if tt.shouldCallMock {
				mockSvc.EXPECT().
					GetImageTagHistory(mock.Anything, tt.imageName, tt.namespace).
					Return(tt.mockResponse, tt.mockError).
					Once()
			}

			// Create handler with mock service
			handler := NewImageHandler(mockSvc)

			// Setup Fiber app
			app := fiber.New()
			app.Get("/api/images/:name/history", handler.GetImageHistory)

			// Create request
			url := "/api/images/" + tt.imageName + "/history"
			if tt.namespace != "" {
				url += "?namespace=" + tt.namespace
			}
			req := httptest.NewRequest("GET", url, nil)

			// Execute request
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}

			// Verify status code
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			// Parse response body
			body, _ := io.ReadAll(resp.Body)

			if tt.expectedStatus == fiber.StatusBadRequest || tt.expectedStatus == fiber.StatusNotFound || tt.mockError != nil {
				// For error cases where we expect an error response (not 404 from router)
				if tt.mockError != nil {
					var errResponse map[string]string
					json.Unmarshal(body, &errResponse)
					if errResponse["error"] == "" {
						t.Error("Expected error in response")
					}

					if errResponse["error"] != tt.mockError.Error() {
						t.Errorf("Expected error message %q, got %q", tt.mockError.Error(), errResponse["error"])
					}
				}
				// For 404, we don't need to verify the error message structure
			} else {
				// Verify success response
				var response models.ImageTagHistory
				if err := json.Unmarshal(body, &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				if response.ImageName != tt.mockResponse.ImageName {
					t.Errorf("Expected image name %q, got %q", tt.mockResponse.ImageName, response.ImageName)
				}

				if len(response.Tags) != len(tt.mockResponse.Tags) {
					t.Errorf("Expected %d tags, got %d", len(tt.mockResponse.Tags), len(response.Tags))
				}

				// Verify tag details if present
				for i, tag := range response.Tags {
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
			mockSvc.AssertExpectations(t)
		})
	}
}
