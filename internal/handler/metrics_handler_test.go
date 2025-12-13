package handler

import (
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/huseyinbabal/kubetag/internal/mocks"
	"github.com/huseyinbabal/kubetag/internal/models"
	"github.com/huseyinbabal/kubetag/internal/service"
	"github.com/prometheus/client_golang/prometheus"
)

func TestNewMetricsHandler(t *testing.T) {
	t.Run("should create metrics handler with service", func(t *testing.T) {
		// Reset Prometheus registry to avoid duplicate registration
		registry := prometheus.NewRegistry()
		prometheus.DefaultRegisterer = registry
		prometheus.DefaultGatherer = registry

		mockRepo := mocks.NewMockImageRepository(t)
		imageService := service.NewImageService(mockRepo, nil)

		handler := NewMetricsHandler(imageService)

		if handler == nil {
			t.Error("Expected non-nil handler")
		}

		if handler.service == nil {
			t.Error("Expected handler to have a service")
		}

		if handler.imageGauge == nil {
			t.Error("Expected handler to have imageGauge")
		}

		if handler.imageTagInfoGauge == nil {
			t.Error("Expected handler to have imageTagInfoGauge")
		}

		if handler.imageVersionGauge == nil {
			t.Error("Expected handler to have imageVersionGauge")
		}
	})
}

func TestGetMetrics(t *testing.T) {
	// Create a new registry for each test to avoid conflicts
	setupMetricsTest := func() (*fiber.App, *MetricsHandler, *mocks.MockImageRepository) {
		registry := prometheus.NewRegistry()
		prometheus.DefaultRegisterer = registry
		prometheus.DefaultGatherer = registry

		mockRepo := mocks.NewMockImageRepository(t)
		imageService := service.NewImageService(mockRepo, nil)
		handler := NewMetricsHandler(imageService)

		app := fiber.New()
		app.Get("/metrics", handler.GetMetrics)

		return app, handler, mockRepo
	}

	t.Run("successfully export metrics with images", func(t *testing.T) {
		app, _, mockRepo := setupMetricsTest()

		// Mock response
		mockRepo.On("GetAllImages", "").Return([]models.ImageInfo{
			{
				Name:         "nginx",
				Tag:          "latest",
				ResourceType: "Deployment",
				ResourceName: "nginx-deploy",
				Namespace:    "default",
				Containers:   []string{"nginx"},
				FirstSeen:    "2024-01-01T00:00:00Z",
				LastSeen:     "2024-01-02T00:00:00Z",
			},
			{
				Name:         "redis",
				Tag:          "7.0",
				ResourceType: "Deployment",
				ResourceName: "redis-deploy",
				Namespace:    "default",
				Containers:   []string{"redis"},
				FirstSeen:    "2024-01-01T00:00:00Z",
				LastSeen:     "2024-01-02T00:00:00Z",
			},
		}, nil)

		req := httptest.NewRequest("GET", "/metrics", nil)
		resp, err := app.Test(req, -1)

		if err != nil {
			t.Fatalf("Failed to execute request: %v", err)
		}

		if resp.StatusCode != fiber.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		// Read response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}
		bodyStr := string(body)

		// Verify metrics are present
		if !strings.Contains(bodyStr, "kubetag_image_info") {
			t.Error("Expected response to contain kubetag_image_info metric")
		}

		if !strings.Contains(bodyStr, "kubetag_image_tag_info") {
			t.Error("Expected response to contain kubetag_image_tag_info metric")
		}

		if !strings.Contains(bodyStr, "kubetag_image_version_count") {
			t.Error("Expected response to contain kubetag_image_version_count metric")
		}

		// Verify image names are in metrics
		if !strings.Contains(bodyStr, "nginx") {
			t.Error("Expected response to contain nginx image")
		}

		if !strings.Contains(bodyStr, "redis") {
			t.Error("Expected response to contain redis image")
		}

		mockRepo.AssertExpectations(t)
	})

	t.Run("handle service error gracefully", func(t *testing.T) {
		app, _, mockRepo := setupMetricsTest()

		// Mock error response
		mockRepo.On("GetAllImages", "").Return(
			[]models.ImageInfo(nil),
			errors.New("database error"),
		)

		req := httptest.NewRequest("GET", "/metrics", nil)
		resp, err := app.Test(req, -1)

		if err != nil {
			t.Fatalf("Failed to execute request: %v", err)
		}

		// Should still return 200 even with error
		if resp.StatusCode != fiber.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		// Metrics endpoint should still work, just with no/stale data
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}
		bodyStr := string(body)

		// Should still have metric definitions
		if !strings.Contains(bodyStr, "HELP") || !strings.Contains(bodyStr, "TYPE") {
			t.Error("Expected response to contain Prometheus metric format")
		}

		mockRepo.AssertExpectations(t)
	})

	t.Run("handle empty image list", func(t *testing.T) {
		app, _, mockRepo := setupMetricsTest()

		// Mock empty response
		mockRepo.On("GetAllImages", "").Return([]models.ImageInfo{}, nil)

		req := httptest.NewRequest("GET", "/metrics", nil)
		resp, err := app.Test(req, -1)

		if err != nil {
			t.Fatalf("Failed to execute request: %v", err)
		}

		if resp.StatusCode != fiber.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}
		bodyStr := string(body)

		// Should have Prometheus format even with no data
		if !strings.Contains(bodyStr, "# HELP") || !strings.Contains(bodyStr, "# TYPE") {
			t.Error("Expected response to contain Prometheus metric format")
		}

		mockRepo.AssertExpectations(t)
	})

	t.Run("handle multiple containers per image", func(t *testing.T) {
		app, _, mockRepo := setupMetricsTest()

		// Mock response with multiple containers
		mockRepo.On("GetAllImages", "").Return([]models.ImageInfo{
			{
				Name:         "nginx",
				Tag:          "latest",
				ResourceType: "Deployment",
				ResourceName: "nginx-deploy",
				Namespace:    "default",
				Containers:   []string{"nginx", "sidecar", "init"},
				FirstSeen:    "2024-01-01T00:00:00Z",
				LastSeen:     "2024-01-02T00:00:00Z",
			},
		}, nil)

		req := httptest.NewRequest("GET", "/metrics", nil)
		resp, err := app.Test(req, -1)

		if err != nil {
			t.Fatalf("Failed to execute request: %v", err)
		}

		if resp.StatusCode != fiber.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}
		bodyStr := string(body)

		// Should have separate metrics for each container
		if !strings.Contains(bodyStr, "nginx") {
			t.Error("Expected response to contain nginx container")
		}
		if !strings.Contains(bodyStr, "sidecar") {
			t.Error("Expected response to contain sidecar container")
		}
		if !strings.Contains(bodyStr, "init") {
			t.Error("Expected response to contain init container")
		}

		mockRepo.AssertExpectations(t)
	})

	t.Run("track version counts correctly", func(t *testing.T) {
		app, _, mockRepo := setupMetricsTest()

		// Mock response with multiple versions of same image
		mockRepo.On("GetAllImages", "").Return([]models.ImageInfo{
			{
				Name:         "nginx",
				Tag:          "1.20",
				ResourceType: "Deployment",
				ResourceName: "nginx-v1",
				Namespace:    "default",
				Containers:   []string{"nginx"},
			},
			{
				Name:         "nginx",
				Tag:          "1.21",
				ResourceType: "Deployment",
				ResourceName: "nginx-v2",
				Namespace:    "default",
				Containers:   []string{"nginx"},
			},
			{
				Name:         "nginx",
				Tag:          "1.22",
				ResourceType: "Deployment",
				ResourceName: "nginx-v3",
				Namespace:    "production",
				Containers:   []string{"nginx"},
			},
		}, nil)

		req := httptest.NewRequest("GET", "/metrics", nil)
		resp, err := app.Test(req, -1)

		if err != nil {
			t.Fatalf("Failed to execute request: %v", err)
		}

		if resp.StatusCode != fiber.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}
		bodyStr := string(body)

		// Should track version counts
		if !strings.Contains(bodyStr, "kubetag_image_version_count") {
			t.Error("Expected response to contain version count metric")
		}

		// Should have metrics for both namespaces
		if !strings.Contains(bodyStr, "default") {
			t.Error("Expected response to contain default namespace")
		}
		if !strings.Contains(bodyStr, "production") {
			t.Error("Expected response to contain production namespace")
		}

		mockRepo.AssertExpectations(t)
	})
}

func TestUpdateMetrics(t *testing.T) {
	t.Run("updateMetrics resets metrics before updating", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		prometheus.DefaultRegisterer = registry
		prometheus.DefaultGatherer = registry

		mockRepo := mocks.NewMockImageRepository(t)
		imageService := service.NewImageService(mockRepo, nil)
		handler := NewMetricsHandler(imageService)

		// Mock first call
		mockRepo.On("GetAllImages", "").Return([]models.ImageInfo{
			{
				Name:         "nginx",
				Tag:          "latest",
				ResourceType: "Deployment",
				ResourceName: "nginx-deploy",
				Namespace:    "default",
				Containers:   []string{"nginx"},
			},
		}, nil).Once()

		// Call updateMetrics
		handler.updateMetrics()

		// Mock second call with different data
		mockRepo.On("GetAllImages", "").Return([]models.ImageInfo{
			{
				Name:         "redis",
				Tag:          "7.0",
				ResourceType: "Deployment",
				ResourceName: "redis-deploy",
				Namespace:    "default",
				Containers:   []string{"redis"},
			},
		}, nil).Once()

		// Call updateMetrics again - should reset old metrics
		handler.updateMetrics()

		mockRepo.AssertExpectations(t)
	})

	t.Run("updateMetrics handles context properly", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		prometheus.DefaultRegisterer = registry
		prometheus.DefaultGatherer = registry

		mockRepo := mocks.NewMockImageRepository(t)
		imageService := service.NewImageService(mockRepo, nil)
		handler := NewMetricsHandler(imageService)

		// Verify updateMetrics calls GetAllImages
		mockRepo.On("GetAllImages", "").Return([]models.ImageInfo{}, nil)

		handler.updateMetrics()

		mockRepo.AssertExpectations(t)
	})
}
