package k8s

import (
	"context"
	"fmt"
	"strings"

	"github.com/huseyinbabal/kubetag/internal/models"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"path/filepath"
)

// Client wraps the Kubernetes clientset
type Client struct {
	clientset kubernetes.Interface
}

// GetClientset returns the underlying Kubernetes clientset
func (c *Client) GetClientset() kubernetes.Interface {
	return c.clientset
}

// NewClient creates a new Kubernetes client
// It tries in-cluster config first, then falls back to kubeconfig
func NewClient() (*Client, error) {
	var config *rest.Config
	var err error

	// Try in-cluster config first
	config, err = rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig
		home := homedir.HomeDir()
		if home == "" {
			return nil, fmt.Errorf("unable to find home directory")
		}

		kubeconfig := filepath.Join(home, ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &Client{clientset: clientset}, nil
}

// GetAllImages collects images from Deployments, DaemonSets, and CronJobs
func (c *Client) GetAllImages(ctx context.Context, namespace string) ([]models.ImageInfo, error) {
	var allImages []models.ImageInfo

	// Get images from Deployments
	deploymentImages, err := c.getDeploymentImages(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment images: %w", err)
	}
	allImages = append(allImages, deploymentImages...)

	// Get images from DaemonSets
	daemonsetImages, err := c.getDaemonSetImages(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get daemonset images: %w", err)
	}
	allImages = append(allImages, daemonsetImages...)

	// Get images from CronJobs
	cronjobImages, err := c.getCronJobImages(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get cronjob images: %w", err)
	}
	allImages = append(allImages, cronjobImages...)

	return allImages, nil
}

func (c *Client) getDeploymentImages(ctx context.Context, namespace string) ([]models.ImageInfo, error) {
	deployments, err := c.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var images []models.ImageInfo
	for _, deployment := range deployments.Items {
		images = append(images, extractImagesFromPodSpec(
			deployment.Spec.Template.Spec,
			"Deployment",
			deployment.Name,
			deployment.Namespace,
		)...)
	}

	return images, nil
}

func (c *Client) getDaemonSetImages(ctx context.Context, namespace string) ([]models.ImageInfo, error) {
	daemonsets, err := c.clientset.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var images []models.ImageInfo
	for _, daemonset := range daemonsets.Items {
		images = append(images, extractImagesFromPodSpec(
			daemonset.Spec.Template.Spec,
			"DaemonSet",
			daemonset.Name,
			daemonset.Namespace,
		)...)
	}

	return images, nil
}

func (c *Client) getCronJobImages(ctx context.Context, namespace string) ([]models.ImageInfo, error) {
	cronjobs, err := c.clientset.BatchV1().CronJobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var images []models.ImageInfo
	for _, cronjob := range cronjobs.Items {
		images = append(images, extractImagesFromPodSpec(
			cronjob.Spec.JobTemplate.Spec.Template.Spec,
			"CronJob",
			cronjob.Name,
			cronjob.Namespace,
		)...)
	}

	return images, nil
}

// extractImagesFromPodSpec extracts image information from a pod spec
func extractImagesFromPodSpec(spec corev1.PodSpec, resourceType, resourceName, namespace string) []models.ImageInfo {
	var images []models.ImageInfo
	imageMap := make(map[string]*models.ImageInfo)

	// Process all containers (including init containers)
	allContainers := append(spec.Containers, spec.InitContainers...)

	for _, container := range allContainers {
		name, tag := parseImage(container.Image)
		key := fmt.Sprintf("%s:%s", name, tag)

		if existing, found := imageMap[key]; found {
			// Add container name to existing image entry
			existing.Containers = append(existing.Containers, container.Name)
		} else {
			// Create new image entry
			imageMap[key] = &models.ImageInfo{
				Name:         name,
				Tag:          tag,
				ResourceType: resourceType,
				ResourceName: resourceName,
				Namespace:    namespace,
				Containers:   []string{container.Name},
			}
		}
	}

	// Convert map to slice
	for _, img := range imageMap {
		images = append(images, *img)
	}

	return images
}

// parseImage splits image string into name and tag
func parseImage(image string) (name, tag string) {
	parts := strings.Split(image, ":")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return image, "latest"
}
