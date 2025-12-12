package handler

import (
	"context"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/huseyinbabal/kubetag/internal/service"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsHandler handles Prometheus metrics
type MetricsHandler struct {
	service           *service.ImageService
	imageGauge        *prometheus.GaugeVec
	imageTagInfoGauge *prometheus.GaugeVec
	imageVersionGauge *prometheus.GaugeVec
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(service *service.ImageService) *MetricsHandler {
	// Define Prometheus metrics
	imageGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kubetag_image_info",
			Help: "Information about container images running in the cluster",
		},
		[]string{"image_name", "tag", "repository", "resource_type", "resource_name", "namespace", "container"},
	)

	imageTagInfoGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kubetag_image_tag_info",
			Help: "Detailed information about image tags with timestamps",
		},
		[]string{"image_name", "tag", "resource_type", "resource_name", "namespace"},
	)

	imageVersionGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kubetag_image_version_count",
			Help: "Count of different versions per image",
		},
		[]string{"image_name", "namespace"},
	)

	// Register metrics with Prometheus
	prometheus.MustRegister(imageGauge)
	prometheus.MustRegister(imageTagInfoGauge)
	prometheus.MustRegister(imageVersionGauge)

	return &MetricsHandler{
		service:           service,
		imageGauge:        imageGauge,
		imageTagInfoGauge: imageTagInfoGauge,
		imageVersionGauge: imageVersionGauge,
	}
}

// GetMetrics handles GET /metrics
func (h *MetricsHandler) GetMetrics(c *fiber.Ctx) error {
	// Update metrics before serving
	h.updateMetrics()

	// Use Fiber's adaptor to wrap the Prometheus HTTP handler
	handler := adaptor.HTTPHandler(promhttp.Handler())
	return handler(c)
}

// updateMetrics updates all Prometheus metrics with current data
func (h *MetricsHandler) updateMetrics() {
	// Reset all metrics
	h.imageGauge.Reset()
	h.imageTagInfoGauge.Reset()
	h.imageVersionGauge.Reset()

	// Get all images
	ctx := context.Background()
	images, err := h.service.GetImages(ctx, "")
	if err != nil {
		return
	}

	// Track version counts per image
	versionCounts := make(map[string]map[string]int) // namespace -> image_name -> count

	for _, img := range images.Images {
		// Set image info metric
		for _, container := range img.Containers {
			h.imageGauge.WithLabelValues(
				img.Name,
				img.Tag,
				"", // repository not available in ImageInfo, could be enhanced
				img.ResourceType,
				img.ResourceName,
				img.Namespace,
				container,
			).Set(1)
		}

		// Set image tag info metric
		h.imageTagInfoGauge.WithLabelValues(
			img.Name,
			img.Tag,
			img.ResourceType,
			img.ResourceName,
			img.Namespace,
		).Set(1)

		// Count versions per image per namespace
		if versionCounts[img.Namespace] == nil {
			versionCounts[img.Namespace] = make(map[string]int)
		}
		versionCounts[img.Namespace][img.Name]++
	}

	// Set version count metrics
	for namespace, imageCounts := range versionCounts {
		for imageName, count := range imageCounts {
			h.imageVersionGauge.WithLabelValues(
				imageName,
				namespace,
			).Set(float64(count))
		}
	}
}
