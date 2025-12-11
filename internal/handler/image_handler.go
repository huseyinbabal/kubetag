package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/huseyinbabal/kubetag/internal/service"
)

// ImageHandler handles HTTP requests for images
type ImageHandler struct {
	service *service.ImageService
}

// NewImageHandler creates a new image handler
func NewImageHandler(service *service.ImageService) *ImageHandler {
	return &ImageHandler{
		service: service,
	}
}

// GetImages handles GET /api/images
func (h *ImageHandler) GetImages(c *fiber.Ctx) error {
	namespace := c.Query("namespace", "")

	images, err := h.service.GetImages(c.Context(), namespace)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(images)
}

// GetImageHistory handles GET /api/images/:name/history
func (h *ImageHandler) GetImageHistory(c *fiber.Ctx) error {
	imageName := c.Params("name")
	namespace := c.Query("namespace", "")

	if imageName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "image name is required",
		})
	}

	history, err := h.service.GetImageTagHistory(c.Context(), imageName, namespace)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(history)
}

// HealthCheck handles GET /health
func (h *ImageHandler) HealthCheck(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status": "healthy",
	})
}
