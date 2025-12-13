package k8s

import (
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
)

func TestShouldWatchNamespace(t *testing.T) {
	tests := []struct {
		name           string
		namespaces     []string
		testNamespace  string
		expectedResult bool
	}{
		{
			name:           "Empty namespace list watches all",
			namespaces:     []string{},
			testNamespace:  "default",
			expectedResult: true,
		},
		{
			name:           "Namespace in watch list",
			namespaces:     []string{"default", "kube-system"},
			testNamespace:  "default",
			expectedResult: true,
		},
		{
			name:           "Namespace not in watch list",
			namespaces:     []string{"default", "kube-system"},
			testNamespace:  "production",
			expectedResult: false,
		},
		{
			name:           "Single namespace match",
			namespaces:     []string{"production"},
			testNamespace:  "production",
			expectedResult: true,
		},
		{
			name:           "Single namespace no match",
			namespaces:     []string{"production"},
			testNamespace:  "staging",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			im := &InformerManager{
				namespaces: tt.namespaces,
			}

			result := im.shouldWatchNamespace(tt.testNamespace)
			if result != tt.expectedResult {
				t.Errorf("Expected %v, got %v", tt.expectedResult, result)
			}
		})
	}
}

func TestExtractImagesFromSpec(t *testing.T) {
	im := &InformerManager{}

	tests := []struct {
		name     string
		spec     corev1.PodSpec
		expected []string
	}{
		{
			name: "Single container",
			spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Image: "nginx:1.19"},
				},
			},
			expected: []string{"nginx:1.19"},
		},
		{
			name: "Multiple containers",
			spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Image: "nginx:1.19"},
					{Image: "redis:6.0"},
				},
			},
			expected: []string{"nginx:1.19", "redis:6.0"},
		},
		{
			name: "Init containers and regular containers",
			spec: corev1.PodSpec{
				InitContainers: []corev1.Container{
					{Image: "busybox:latest"},
				},
				Containers: []corev1.Container{
					{Image: "app:v1.0"},
				},
			},
			expected: []string{"app:v1.0", "busybox:latest"}, // Order may vary due to slice append
		},
		{
			name: "Empty spec",
			spec: corev1.PodSpec{
				Containers: []corev1.Container{},
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			images := im.extractImagesFromSpec(tt.spec)
			if len(images) != len(tt.expected) {
				t.Errorf("Expected %d images, got %d", len(tt.expected), len(images))
				return
			}

			// Check that all expected images are present (order may vary)
			imageMap := make(map[string]bool)
			for _, img := range images {
				imageMap[img] = true
			}
			for _, expected := range tt.expected {
				if !imageMap[expected] {
					t.Errorf("Expected image '%s' not found in result", expected)
				}
			}
		})
	}
}

func TestHasImageChanged(t *testing.T) {
	im := &InformerManager{}

	tests := []struct {
		name     string
		oldSpec  corev1.PodSpec
		newSpec  corev1.PodSpec
		expected bool
	}{
		{
			name: "No change",
			oldSpec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Image: "nginx:1.19"},
				},
			},
			newSpec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Image: "nginx:1.19"},
				},
			},
			expected: false,
		},
		{
			name: "Image tag changed",
			oldSpec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Image: "nginx:1.19"},
				},
			},
			newSpec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Image: "nginx:1.20"},
				},
			},
			expected: true,
		},
		{
			name: "Container added",
			oldSpec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Image: "nginx:1.19"},
				},
			},
			newSpec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Image: "nginx:1.19"},
					{Image: "redis:6.0"},
				},
			},
			expected: true,
		},
		{
			name: "Container removed",
			oldSpec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Image: "nginx:1.19"},
					{Image: "redis:6.0"},
				},
			},
			newSpec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Image: "nginx:1.19"},
				},
			},
			expected: true,
		},
		{
			name: "Init container changed",
			oldSpec: corev1.PodSpec{
				InitContainers: []corev1.Container{
					{Image: "busybox:1.0"},
				},
				Containers: []corev1.Container{
					{Image: "app:v1.0"},
				},
			},
			newSpec: corev1.PodSpec{
				InitContainers: []corev1.Container{
					{Image: "busybox:2.0"},
				},
				Containers: []corev1.Container{
					{Image: "app:v1.0"},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := im.hasImageChanged(tt.oldSpec, tt.newSpec)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestImageEventType(t *testing.T) {
	// Test that event types are properly defined
	if EventTypeAdd != "ADD" {
		t.Errorf("Expected EventTypeAdd to be 'ADD', got '%s'", EventTypeAdd)
	}
	if EventTypeUpdate != "UPDATE" {
		t.Errorf("Expected EventTypeUpdate to be 'UPDATE', got '%s'", EventTypeUpdate)
	}
	if EventTypeDelete != "DELETE" {
		t.Errorf("Expected EventTypeDelete to be 'DELETE', got '%s'", EventTypeDelete)
	}
}

func TestImageEventStructure(t *testing.T) {
	// Test that ImageEvent struct can be properly created
	now := time.Now().UTC()
	event := ImageEvent{
		Type:          EventTypeAdd,
		ResourceType:  "Deployment",
		ResourceName:  "test-deployment",
		Namespace:     "default",
		ContainerName: "nginx",
		ImageName:     "nginx",
		ImageTag:      "1.19",
		Repository:    "docker.io",
		Timestamp:     now,
	}

	if event.Type != EventTypeAdd {
		t.Errorf("Expected Type '%s', got '%s'", EventTypeAdd, event.Type)
	}
	if event.ResourceType != "Deployment" {
		t.Errorf("Expected ResourceType 'Deployment', got '%s'", event.ResourceType)
	}
	if event.Namespace != "default" {
		t.Errorf("Expected Namespace 'default', got '%s'", event.Namespace)
	}
	if !event.Timestamp.Equal(now) {
		t.Errorf("Expected Timestamp to match")
	}
}

func TestHandlePodSpecChange(t *testing.T) {
	t.Run("handles pod spec change with single container", func(t *testing.T) {
		var capturedEvents []ImageEvent
		var mu sync.Mutex

		handler := func(event ImageEvent) {
			mu.Lock()
			capturedEvents = append(capturedEvents, event)
			mu.Unlock()
		}

		im := &InformerManager{
			eventHandler: handler,
			namespaces:   []string{},
		}

		spec := corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "nginx", Image: "nginx:latest"},
			},
		}

		im.handlePodSpecChange(EventTypeAdd, "Deployment", "test-deploy", "default", spec)

		if len(capturedEvents) != 1 {
			t.Errorf("Expected 1 event, got %d", len(capturedEvents))
		}

		if len(capturedEvents) > 0 {
			event := capturedEvents[0]
			if event.Type != EventTypeAdd {
				t.Errorf("Expected EventTypeAdd, got %s", event.Type)
			}
			if event.ResourceType != "Deployment" {
				t.Errorf("Expected ResourceType 'Deployment', got '%s'", event.ResourceType)
			}
			if event.ImageName != "nginx" {
				t.Errorf("Expected ImageName 'nginx', got '%s'", event.ImageName)
			}
			if event.ImageTag != "latest" {
				t.Errorf("Expected ImageTag 'latest', got '%s'", event.ImageTag)
			}
		}
	})

	t.Run("handles pod spec change with multiple containers", func(t *testing.T) {
		var capturedEvents []ImageEvent
		var mu sync.Mutex

		handler := func(event ImageEvent) {
			mu.Lock()
			capturedEvents = append(capturedEvents, event)
			mu.Unlock()
		}

		im := &InformerManager{
			eventHandler: handler,
			namespaces:   []string{},
		}

		spec := corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "nginx", Image: "nginx:latest"},
				{Name: "redis", Image: "redis:7.0"},
			},
		}

		im.handlePodSpecChange(EventTypeUpdate, "Deployment", "test-deploy", "default", spec)

		if len(capturedEvents) != 2 {
			t.Errorf("Expected 2 events, got %d", len(capturedEvents))
		}
	})

	t.Run("filters events by namespace", func(t *testing.T) {
		var capturedEvents []ImageEvent
		var mu sync.Mutex

		handler := func(event ImageEvent) {
			mu.Lock()
			capturedEvents = append(capturedEvents, event)
			mu.Unlock()
		}

		im := &InformerManager{
			eventHandler: handler,
			namespaces:   []string{"default"},
		}

		spec := corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "nginx", Image: "nginx:latest"},
			},
		}

		// This should be captured
		im.handlePodSpecChange(EventTypeAdd, "Deployment", "test-deploy", "default", spec)

		// This should be filtered out
		im.handlePodSpecChange(EventTypeAdd, "Deployment", "other-deploy", "production", spec)

		if len(capturedEvents) != 1 {
			t.Errorf("Expected 1 event (filtered), got %d", len(capturedEvents))
		}

		if len(capturedEvents) > 0 && capturedEvents[0].Namespace != "default" {
			t.Errorf("Expected namespace 'default', got '%s'", capturedEvents[0].Namespace)
		}
	})

	t.Run("handles init containers", func(t *testing.T) {
		var capturedEvents []ImageEvent
		var mu sync.Mutex

		handler := func(event ImageEvent) {
			mu.Lock()
			capturedEvents = append(capturedEvents, event)
			mu.Unlock()
		}

		im := &InformerManager{
			eventHandler: handler,
			namespaces:   []string{},
		}

		spec := corev1.PodSpec{
			InitContainers: []corev1.Container{
				{Name: "init", Image: "busybox:latest"},
			},
			Containers: []corev1.Container{
				{Name: "app", Image: "nginx:latest"},
			},
		}

		im.handlePodSpecChange(EventTypeAdd, "Deployment", "test-deploy", "default", spec)

		if len(capturedEvents) != 2 {
			t.Errorf("Expected 2 events (init + regular), got %d", len(capturedEvents))
		}
	})

	t.Run("handles nil event handler gracefully", func(t *testing.T) {
		im := &InformerManager{
			eventHandler: nil,
			namespaces:   []string{},
		}

		spec := corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "nginx", Image: "nginx:latest"},
			},
		}

		// Should not panic
		im.handlePodSpecChange(EventTypeAdd, "Deployment", "test-deploy", "default", spec)
	})

	t.Run("handles DELETE event type", func(t *testing.T) {
		var capturedEvents []ImageEvent
		var mu sync.Mutex

		handler := func(event ImageEvent) {
			mu.Lock()
			capturedEvents = append(capturedEvents, event)
			mu.Unlock()
		}

		im := &InformerManager{
			eventHandler: handler,
			namespaces:   []string{},
		}

		spec := corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "nginx", Image: "nginx:latest"},
			},
		}

		im.handlePodSpecChange(EventTypeDelete, "CronJob", "test-cronjob", "default", spec)

		if len(capturedEvents) != 1 {
			t.Errorf("Expected 1 event, got %d", len(capturedEvents))
		}

		if len(capturedEvents) > 0 && capturedEvents[0].Type != EventTypeDelete {
			t.Errorf("Expected EventTypeDelete, got %s", capturedEvents[0].Type)
		}
	})
}
