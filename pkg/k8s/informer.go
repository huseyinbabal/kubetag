package k8s

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// ImageEventType represents the type of image event
type ImageEventType string

const (
	EventTypeAdd    ImageEventType = "ADD"
	EventTypeUpdate ImageEventType = "UPDATE"
	EventTypeDelete ImageEventType = "DELETE"
)

// ImageEvent represents an image change event
type ImageEvent struct {
	Type          ImageEventType
	ResourceType  string // Deployment, DaemonSet, CronJob
	ResourceName  string
	Namespace     string
	ContainerName string
	ImageName     string
	ImageTag      string
	Repository    string
	Timestamp     time.Time
}

// ImageEventHandler is the callback function for image events
type ImageEventHandler func(event ImageEvent)

// InformerManager manages Kubernetes informers for watching resources
type InformerManager struct {
	clientset    *kubernetes.Clientset
	factory      informers.SharedInformerFactory
	stopCh       chan struct{}
	eventHandler ImageEventHandler
	namespaces   []string // List of namespaces to watch, empty means all
}

// NewInformerManager creates a new informer manager
// namespaces: list of namespaces to watch. Pass ["*"] or empty slice to watch all namespaces
func NewInformerManager(clientset *kubernetes.Clientset, eventHandler ImageEventHandler, namespaces []string) *InformerManager {
	// If namespaces contains "*" or is empty, watch all namespaces
	if len(namespaces) == 0 || (len(namespaces) == 1 && namespaces[0] == "*") {
		namespaces = []string{} // Empty means all namespaces
	}

	var factory informers.SharedInformerFactory

	// If watching all namespaces, create factory without namespace filter
	if len(namespaces) == 0 {
		factory = informers.NewSharedInformerFactory(clientset, 30*time.Second)
	} else if len(namespaces) == 1 {
		// If watching a single namespace, use namespace-specific factory
		factory = informers.NewSharedInformerFactoryWithOptions(clientset, 30*time.Second,
			informers.WithNamespace(namespaces[0]))
	} else {
		// For multiple namespaces, we'll still use all namespaces factory but filter in event handlers
		factory = informers.NewSharedInformerFactory(clientset, 30*time.Second)
	}

	return &InformerManager{
		clientset:    clientset,
		factory:      factory,
		stopCh:       make(chan struct{}),
		eventHandler: eventHandler,
		namespaces:   namespaces,
	}
}

// Start begins watching for resource changes
func (im *InformerManager) Start(ctx context.Context) error {
	log.Println("Starting Kubernetes informers...")

	// Setup informers for different resource types
	if err := im.setupDeploymentInformer(); err != nil {
		return fmt.Errorf("failed to setup deployment informer: %w", err)
	}

	if err := im.setupDaemonSetInformer(); err != nil {
		return fmt.Errorf("failed to setup daemonset informer: %w", err)
	}

	if err := im.setupCronJobInformer(); err != nil {
		return fmt.Errorf("failed to setup cronjob informer: %w", err)
	}

	// Start all informers
	im.factory.Start(im.stopCh)

	// Wait for cache sync
	log.Println("Waiting for informer caches to sync...")
	if !cache.WaitForCacheSync(im.stopCh,
		im.factory.Apps().V1().Deployments().Informer().HasSynced,
		im.factory.Apps().V1().DaemonSets().Informer().HasSynced,
		im.factory.Batch().V1().CronJobs().Informer().HasSynced,
	) {
		return fmt.Errorf("failed to sync informer caches")
	}

	log.Println("Informer caches synced successfully")

	// Wait for context cancellation
	go func() {
		<-ctx.Done()
		im.Stop()
	}()

	return nil
}

// Stop stops all informers
func (im *InformerManager) Stop() {
	log.Println("Stopping Kubernetes informers...")
	close(im.stopCh)
}

// setupDeploymentInformer sets up the Deployment informer
func (im *InformerManager) setupDeploymentInformer() error {
	informer := im.factory.Apps().V1().Deployments().Informer()

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			deployment := obj.(*appsv1.Deployment)
			im.handlePodSpecChange(EventTypeAdd, "Deployment", deployment.Name, deployment.Namespace, deployment.Spec.Template.Spec)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldDeployment := oldObj.(*appsv1.Deployment)
			newDeployment := newObj.(*appsv1.Deployment)

			// Check if image has changed
			if im.hasImageChanged(oldDeployment.Spec.Template.Spec, newDeployment.Spec.Template.Spec) {
				im.handlePodSpecChange(EventTypeUpdate, "Deployment", newDeployment.Name, newDeployment.Namespace, newDeployment.Spec.Template.Spec)
			}
		},
		DeleteFunc: func(obj interface{}) {
			deployment := obj.(*appsv1.Deployment)
			im.handlePodSpecChange(EventTypeDelete, "Deployment", deployment.Name, deployment.Namespace, deployment.Spec.Template.Spec)
		},
	})

	return err
}

// setupDaemonSetInformer sets up the DaemonSet informer
func (im *InformerManager) setupDaemonSetInformer() error {
	informer := im.factory.Apps().V1().DaemonSets().Informer()

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			daemonset := obj.(*appsv1.DaemonSet)
			im.handlePodSpecChange(EventTypeAdd, "DaemonSet", daemonset.Name, daemonset.Namespace, daemonset.Spec.Template.Spec)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldDaemonSet := oldObj.(*appsv1.DaemonSet)
			newDaemonSet := newObj.(*appsv1.DaemonSet)

			if im.hasImageChanged(oldDaemonSet.Spec.Template.Spec, newDaemonSet.Spec.Template.Spec) {
				im.handlePodSpecChange(EventTypeUpdate, "DaemonSet", newDaemonSet.Name, newDaemonSet.Namespace, newDaemonSet.Spec.Template.Spec)
			}
		},
		DeleteFunc: func(obj interface{}) {
			daemonset := obj.(*appsv1.DaemonSet)
			im.handlePodSpecChange(EventTypeDelete, "DaemonSet", daemonset.Name, daemonset.Namespace, daemonset.Spec.Template.Spec)
		},
	})

	return err
}

// setupCronJobInformer sets up the CronJob informer
func (im *InformerManager) setupCronJobInformer() error {
	informer := im.factory.Batch().V1().CronJobs().Informer()

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			cronjob := obj.(*batchv1.CronJob)
			im.handlePodSpecChange(EventTypeAdd, "CronJob", cronjob.Name, cronjob.Namespace, cronjob.Spec.JobTemplate.Spec.Template.Spec)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldCronJob := oldObj.(*batchv1.CronJob)
			newCronJob := newObj.(*batchv1.CronJob)

			if im.hasImageChanged(oldCronJob.Spec.JobTemplate.Spec.Template.Spec, newCronJob.Spec.JobTemplate.Spec.Template.Spec) {
				im.handlePodSpecChange(EventTypeUpdate, "CronJob", newCronJob.Name, newCronJob.Namespace, newCronJob.Spec.JobTemplate.Spec.Template.Spec)
			}
		},
		DeleteFunc: func(obj interface{}) {
			cronjob := obj.(*batchv1.CronJob)
			im.handlePodSpecChange(EventTypeDelete, "CronJob", cronjob.Name, cronjob.Namespace, cronjob.Spec.JobTemplate.Spec.Template.Spec)
		},
	})

	return err
}

// shouldWatchNamespace checks if a namespace should be watched based on the filter
func (im *InformerManager) shouldWatchNamespace(namespace string) bool {
	// If namespaces list is empty, watch all namespaces
	if len(im.namespaces) == 0 {
		return true
	}

	// Check if namespace is in the watch list
	for _, ns := range im.namespaces {
		if ns == namespace {
			return true
		}
	}

	return false
}

// handlePodSpecChange processes pod spec changes and extracts image information
func (im *InformerManager) handlePodSpecChange(eventType ImageEventType, resourceType, resourceName, namespace string, spec corev1.PodSpec) {
	// Filter by namespace if configured
	if !im.shouldWatchNamespace(namespace) {
		return
	}

	allContainers := append(spec.Containers, spec.InitContainers...)

	for _, container := range allContainers {
		name, tag, repo := parseImageFull(container.Image)

		event := ImageEvent{
			Type:          eventType,
			ResourceType:  resourceType,
			ResourceName:  resourceName,
			Namespace:     namespace,
			ContainerName: container.Name,
			ImageName:     name,
			ImageTag:      tag,
			Repository:    repo,
			Timestamp:     time.Now().UTC(),
		}

		// Call the event handler
		if im.eventHandler != nil {
			im.eventHandler(event)
		}
	}
}

// hasImageChanged checks if images in pod specs have changed
func (im *InformerManager) hasImageChanged(oldSpec, newSpec corev1.PodSpec) bool {
	oldImages := im.extractImagesFromSpec(oldSpec)
	newImages := im.extractImagesFromSpec(newSpec)

	if len(oldImages) != len(newImages) {
		return true
	}

	for i := range oldImages {
		if oldImages[i] != newImages[i] {
			return true
		}
	}

	return false
}

// extractImagesFromSpec extracts all image strings from a pod spec
func (im *InformerManager) extractImagesFromSpec(spec corev1.PodSpec) []string {
	var images []string
	allContainers := append(spec.Containers, spec.InitContainers...)

	for _, container := range allContainers {
		images = append(images, container.Image)
	}

	return images
}

// parseImageFull splits image string into name, tag, and repository
// Examples:
//   - nginx:latest -> name: nginx, tag: latest, repo: docker.io
//   - gcr.io/my-project/app:v1.0 -> name: app, tag: v1.0, repo: gcr.io/my-project
func parseImageFull(image string) (name, tag, repository string) {
	// Default repository
	repository = "docker.io"
	tag = "latest"

	// Split by tag separator
	parts := strings.Split(image, ":")
	imagePart := parts[0]
	if len(parts) == 2 {
		tag = parts[1]
	}

	// Check if there's a repository prefix (contains /)
	slashParts := strings.Split(imagePart, "/")

	if len(slashParts) == 1 {
		// Just image name, e.g., "nginx"
		name = slashParts[0]
	} else if len(slashParts) == 2 {
		// Could be "library/nginx" or "gcr.io/nginx"
		if strings.Contains(slashParts[0], ".") {
			// It's a registry, e.g., "gcr.io/nginx"
			repository = slashParts[0]
			name = slashParts[1]
		} else {
			// It's a namespace, e.g., "library/nginx"
			repository = "docker.io/" + slashParts[0]
			name = slashParts[1]
		}
	} else {
		// Full path with registry, e.g., "gcr.io/project/app"
		name = slashParts[len(slashParts)-1]
		repository = strings.Join(slashParts[:len(slashParts)-1], "/")
	}

	return name, tag, repository
}
