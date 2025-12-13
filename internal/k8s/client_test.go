package k8s

import (
	"testing"

	"github.com/huseyinbabal/kubetag/internal/models"
	corev1 "k8s.io/api/core/v1"
)

func TestParseImage(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedName string
		expectedTag  string
	}{
		{
			name:         "Image with tag",
			input:        "nginx:1.19",
			expectedName: "nginx",
			expectedTag:  "1.19",
		},
		{
			name:         "Image without tag",
			input:        "nginx",
			expectedName: "nginx",
			expectedTag:  "latest",
		},
		{
			name:         "Image with repository and tag",
			input:        "gcr.io/my-project/app:v1.0",
			expectedName: "gcr.io/my-project/app",
			expectedTag:  "v1.0",
		},
		{
			name:         "Image with repository",
			input:        "gcr.io/project/app",
			expectedName: "gcr.io/project/app",
			expectedTag:  "latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, tag := parseImage(tt.input)
			if name != tt.expectedName {
				t.Errorf("Expected name '%s', got '%s'", tt.expectedName, name)
			}
			if tag != tt.expectedTag {
				t.Errorf("Expected tag '%s', got '%s'", tt.expectedTag, tag)
			}
		})
	}
}

func TestExtractImagesFromPodSpec(t *testing.T) {
	tests := []struct {
		name         string
		spec         corev1.PodSpec
		resourceType string
		resourceName string
		namespace    string
		expected     int // expected number of images
	}{
		{
			name: "Single container",
			spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "nginx",
						Image: "nginx:1.19",
					},
				},
			},
			resourceType: "Deployment",
			resourceName: "test-deployment",
			namespace:    "default",
			expected:     1,
		},
		{
			name: "Multiple containers",
			spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "nginx",
						Image: "nginx:1.19",
					},
					{
						Name:  "redis",
						Image: "redis:6.0",
					},
				},
			},
			resourceType: "Deployment",
			resourceName: "test-deployment",
			namespace:    "default",
			expected:     2,
		},
		{
			name: "Container with init container",
			spec: corev1.PodSpec{
				InitContainers: []corev1.Container{
					{
						Name:  "init",
						Image: "busybox:latest",
					},
				},
				Containers: []corev1.Container{
					{
						Name:  "app",
						Image: "myapp:v1.0",
					},
				},
			},
			resourceType: "Deployment",
			resourceName: "test-deployment",
			namespace:    "default",
			expected:     2,
		},
		{
			name: "Duplicate images in different containers",
			spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "container1",
						Image: "nginx:1.19",
					},
					{
						Name:  "container2",
						Image: "nginx:1.19",
					},
				},
			},
			resourceType: "Deployment",
			resourceName: "test-deployment",
			namespace:    "default",
			expected:     1, // Should be deduplicated
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			images := extractImagesFromPodSpec(tt.spec, tt.resourceType, tt.resourceName, tt.namespace)
			if len(images) != tt.expected {
				t.Errorf("Expected %d images, got %d", tt.expected, len(images))
			}

			// Verify basic properties
			for _, img := range images {
				if img.ResourceType != tt.resourceType {
					t.Errorf("Expected ResourceType '%s', got '%s'", tt.resourceType, img.ResourceType)
				}
				if img.ResourceName != tt.resourceName {
					t.Errorf("Expected ResourceName '%s', got '%s'", tt.resourceName, img.ResourceName)
				}
				if img.Namespace != tt.namespace {
					t.Errorf("Expected Namespace '%s', got '%s'", tt.namespace, img.Namespace)
				}
				if len(img.Containers) == 0 {
					t.Error("Expected at least one container name")
				}
			}
		})
	}
}

func TestExtractImagesFromPodSpecContainerAggregation(t *testing.T) {
	spec := corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "container1",
				Image: "nginx:1.19",
			},
			{
				Name:  "container2",
				Image: "nginx:1.19",
			},
		},
	}

	images := extractImagesFromPodSpec(spec, "Deployment", "test", "default")

	if len(images) != 1 {
		t.Fatalf("Expected 1 image, got %d", len(images))
	}

	if len(images[0].Containers) != 2 {
		t.Errorf("Expected 2 container names, got %d", len(images[0].Containers))
	}

	// Verify both container names are present
	containerNames := images[0].Containers
	foundContainer1 := false
	foundContainer2 := false
	for _, name := range containerNames {
		if name == "container1" {
			foundContainer1 = true
		}
		if name == "container2" {
			foundContainer2 = true
		}
	}

	if !foundContainer1 || !foundContainer2 {
		t.Errorf("Expected both container1 and container2 in containers list, got %v", containerNames)
	}
}

func TestParseImageFull(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedName string
		expectedTag  string
		expectedRepo string
	}{
		{
			name:         "Simple image with tag",
			input:        "nginx:1.19",
			expectedName: "nginx",
			expectedTag:  "1.19",
			expectedRepo: "docker.io",
		},
		{
			name:         "Simple image without tag",
			input:        "nginx",
			expectedName: "nginx",
			expectedTag:  "latest",
			expectedRepo: "docker.io",
		},
		{
			name:         "Docker Hub with namespace",
			input:        "library/nginx:1.19",
			expectedName: "nginx",
			expectedTag:  "1.19",
			expectedRepo: "docker.io/library",
		},
		{
			name:         "GCR image",
			input:        "gcr.io/my-project/app:v1.0",
			expectedName: "app",
			expectedTag:  "v1.0",
			expectedRepo: "gcr.io/my-project",
		},
		{
			name:         "Private registry",
			input:        "registry.example.com/team/app:v2.0",
			expectedName: "app",
			expectedTag:  "v2.0",
			expectedRepo: "registry.example.com/team",
		},
		{
			name:         "ECR image",
			input:        "123456789012.dkr.ecr.us-east-1.amazonaws.com/myapp:latest",
			expectedName: "myapp",
			expectedTag:  "latest",
			expectedRepo: "123456789012.dkr.ecr.us-east-1.amazonaws.com",
		},
		{
			name:         "Deep path repository",
			input:        "gcr.io/project/team/subteam/app:v1.0",
			expectedName: "app",
			expectedTag:  "v1.0",
			expectedRepo: "gcr.io/project/team/subteam",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, tag, repo := parseImageFull(tt.input)
			if name != tt.expectedName {
				t.Errorf("Expected name '%s', got '%s'", tt.expectedName, name)
			}
			if tag != tt.expectedTag {
				t.Errorf("Expected tag '%s', got '%s'", tt.expectedTag, tag)
			}
			if repo != tt.expectedRepo {
				t.Errorf("Expected repo '%s', got '%s'", tt.expectedRepo, repo)
			}
		})
	}
}

func TestImageInfoStructure(t *testing.T) {
	// Test that ImageInfo struct can be properly created
	info := models.ImageInfo{
		Name:         "nginx",
		Tag:          "1.19",
		ResourceType: "Deployment",
		ResourceName: "web-server",
		Namespace:    "default",
		Containers:   []string{"nginx", "sidecar"},
		FirstSeen:    "2023-01-01T00:00:00Z",
		LastSeen:     "2023-01-02T00:00:00Z",
	}

	if info.Name != "nginx" {
		t.Errorf("Expected Name 'nginx', got '%s'", info.Name)
	}
	if len(info.Containers) != 2 {
		t.Errorf("Expected 2 containers, got %d", len(info.Containers))
	}
}
