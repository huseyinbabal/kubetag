package k8s

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetAllImagesWithFakeClient(t *testing.T) {
	t.Run("get images from deployments only", func(t *testing.T) {
		// Create fake clientset
		fakeClient := fake.NewSimpleClientset()

		// Create test deployment
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nginx-deployment",
				Namespace: "default",
			},
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "nginx",
								Image: "nginx:1.19",
							},
							{
								Name:  "sidecar",
								Image: "busybox:latest",
							},
						},
					},
				},
			},
		}

		_, err := fakeClient.AppsV1().Deployments("default").Create(
			context.Background(), deployment, metav1.CreateOptions{},
		)
		if err != nil {
			t.Fatalf("Failed to create deployment: %v", err)
		}

		// Create client with fake clientset
		client := &Client{clientset: fakeClient}

		// Test GetAllImages
		images, err := client.GetAllImages(context.Background(), "default")
		if err != nil {
			t.Fatalf("GetAllImages failed: %v", err)
		}

		// Verify results
		if len(images) != 2 {
			t.Errorf("Expected 2 images, got %d", len(images))
		}

		// Verify nginx image
		foundNginx := false
		foundBusybox := false
		for _, img := range images {
			if img.Name == "nginx" && img.Tag == "1.19" {
				foundNginx = true
				if img.ResourceType != "Deployment" {
					t.Errorf("Expected ResourceType 'Deployment', got '%s'", img.ResourceType)
				}
				if img.ResourceName != "nginx-deployment" {
					t.Errorf("Expected ResourceName 'nginx-deployment', got '%s'", img.ResourceName)
				}
				if img.Namespace != "default" {
					t.Errorf("Expected Namespace 'default', got '%s'", img.Namespace)
				}
				if len(img.Containers) != 1 || img.Containers[0] != "nginx" {
					t.Errorf("Expected containers [nginx], got %v", img.Containers)
				}
			}
			if img.Name == "busybox" && img.Tag == "latest" {
				foundBusybox = true
				if len(img.Containers) != 1 || img.Containers[0] != "sidecar" {
					t.Errorf("Expected containers [sidecar], got %v", img.Containers)
				}
			}
		}

		if !foundNginx {
			t.Error("nginx:1.19 image not found in results")
		}
		if !foundBusybox {
			t.Error("busybox:latest image not found in results")
		}
	})

	t.Run("get images from daemonsets only", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()

		// Create test daemonset
		daemonset := &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "logging-agent",
				Namespace: "kube-system",
			},
			Spec: appsv1.DaemonSetSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "fluentd",
								Image: "fluentd:v1.14",
							},
						},
					},
				},
			},
		}

		_, err := fakeClient.AppsV1().DaemonSets("kube-system").Create(
			context.Background(), daemonset, metav1.CreateOptions{},
		)
		if err != nil {
			t.Fatalf("Failed to create daemonset: %v", err)
		}

		client := &Client{clientset: fakeClient}
		images, err := client.GetAllImages(context.Background(), "kube-system")
		if err != nil {
			t.Fatalf("GetAllImages failed: %v", err)
		}

		if len(images) != 1 {
			t.Errorf("Expected 1 image, got %d", len(images))
		}

		img := images[0]
		if img.Name != "fluentd" {
			t.Errorf("Expected name 'fluentd', got '%s'", img.Name)
		}
		if img.Tag != "v1.14" {
			t.Errorf("Expected tag 'v1.14', got '%s'", img.Tag)
		}
		if img.ResourceType != "DaemonSet" {
			t.Errorf("Expected ResourceType 'DaemonSet', got '%s'", img.ResourceType)
		}
		if img.ResourceName != "logging-agent" {
			t.Errorf("Expected ResourceName 'logging-agent', got '%s'", img.ResourceName)
		}
	})

	t.Run("get images from cronjobs only", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()

		// Create test cronjob
		cronjob := &batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "backup-job",
				Namespace: "production",
			},
			Spec: batchv1.CronJobSpec{
				Schedule: "0 0 * * *",
				JobTemplate: batchv1.JobTemplateSpec{
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "backup",
										Image: "backup-tool:v2.0",
									},
								},
								InitContainers: []corev1.Container{
									{
										Name:  "init-backup",
										Image: "busybox:1.35",
									},
								},
							},
						},
					},
				},
			},
		}

		_, err := fakeClient.BatchV1().CronJobs("production").Create(
			context.Background(), cronjob, metav1.CreateOptions{},
		)
		if err != nil {
			t.Fatalf("Failed to create cronjob: %v", err)
		}

		client := &Client{clientset: fakeClient}
		images, err := client.GetAllImages(context.Background(), "production")
		if err != nil {
			t.Fatalf("GetAllImages failed: %v", err)
		}

		if len(images) != 2 {
			t.Errorf("Expected 2 images (main + init container), got %d", len(images))
		}

		foundBackup := false
		foundInit := false
		for _, img := range images {
			if img.Name == "backup-tool" && img.Tag == "v2.0" {
				foundBackup = true
				if img.ResourceType != "CronJob" {
					t.Errorf("Expected ResourceType 'CronJob', got '%s'", img.ResourceType)
				}
				if img.ResourceName != "backup-job" {
					t.Errorf("Expected ResourceName 'backup-job', got '%s'", img.ResourceName)
				}
			}
			if img.Name == "busybox" && img.Tag == "1.35" {
				foundInit = true
				if len(img.Containers) != 1 || img.Containers[0] != "init-backup" {
					t.Errorf("Expected containers [init-backup], got %v", img.Containers)
				}
			}
		}

		if !foundBackup {
			t.Error("backup-tool:v2.0 image not found")
		}
		if !foundInit {
			t.Error("busybox:1.35 init container image not found")
		}
	})

	t.Run("get images from all resource types", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()

		// Create deployment
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "web-app",
				Namespace: "default",
			},
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "app", Image: "myapp:v1.0"},
						},
					},
				},
			},
		}

		// Create daemonset
		daemonset := &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "monitor",
				Namespace: "default",
			},
			Spec: appsv1.DaemonSetSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "prometheus", Image: "prometheus:latest"},
						},
					},
				},
			},
		}

		// Create cronjob
		cronjob := &batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cleanup",
				Namespace: "default",
			},
			Spec: batchv1.CronJobSpec{
				Schedule: "0 0 * * *",
				JobTemplate: batchv1.JobTemplateSpec{
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{Name: "cleaner", Image: "alpine:3.18"},
								},
							},
						},
					},
				},
			},
		}

		fakeClient.AppsV1().Deployments("default").Create(context.Background(), deployment, metav1.CreateOptions{})
		fakeClient.AppsV1().DaemonSets("default").Create(context.Background(), daemonset, metav1.CreateOptions{})
		fakeClient.BatchV1().CronJobs("default").Create(context.Background(), cronjob, metav1.CreateOptions{})

		client := &Client{clientset: fakeClient}
		images, err := client.GetAllImages(context.Background(), "default")
		if err != nil {
			t.Fatalf("GetAllImages failed: %v", err)
		}

		if len(images) != 3 {
			t.Errorf("Expected 3 images (one from each resource type), got %d", len(images))
			for i, img := range images {
				t.Logf("Image %d: %s:%s (%s)", i, img.Name, img.Tag, img.ResourceType)
			}
		}

		// Verify we have one of each resource type
		resourceTypes := make(map[string]bool)
		for _, img := range images {
			resourceTypes[img.ResourceType] = true
		}

		if !resourceTypes["Deployment"] {
			t.Error("Missing Deployment image")
		}
		if !resourceTypes["DaemonSet"] {
			t.Error("Missing DaemonSet image")
		}
		if !resourceTypes["CronJob"] {
			t.Error("Missing CronJob image")
		}
	})

	t.Run("get images with namespace filter", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()

		// Create deployment in default namespace
		deployment1 := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "app1",
				Namespace: "default",
			},
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "app1", Image: "app1:v1"},
						},
					},
				},
			},
		}

		// Create deployment in production namespace
		deployment2 := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "app2",
				Namespace: "production",
			},
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "app2", Image: "app2:v1"},
						},
					},
				},
			},
		}

		fakeClient.AppsV1().Deployments("default").Create(context.Background(), deployment1, metav1.CreateOptions{})
		fakeClient.AppsV1().Deployments("production").Create(context.Background(), deployment2, metav1.CreateOptions{})

		client := &Client{clientset: fakeClient}

		// Get images from default namespace only
		images, err := client.GetAllImages(context.Background(), "default")
		if err != nil {
			t.Fatalf("GetAllImages failed: %v", err)
		}

		if len(images) != 1 {
			t.Errorf("Expected 1 image from default namespace, got %d", len(images))
		}

		if images[0].Namespace != "default" {
			t.Errorf("Expected namespace 'default', got '%s'", images[0].Namespace)
		}
		if images[0].Name != "app1" {
			t.Errorf("Expected image name 'app1', got '%s'", images[0].Name)
		}
	})

	t.Run("get all images across all namespaces", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()

		// Create resources in multiple namespaces
		namespaces := []string{"default", "production", "staging"}
		for i, ns := range namespaces {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app",
					Namespace: ns,
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "app", Image: "myapp:v" + string(rune(i+1))},
							},
						},
					},
				},
			}
			fakeClient.AppsV1().Deployments(ns).Create(context.Background(), deployment, metav1.CreateOptions{})
		}

		client := &Client{clientset: fakeClient}

		// Get images from all namespaces (empty string)
		images, err := client.GetAllImages(context.Background(), "")
		if err != nil {
			t.Fatalf("GetAllImages failed: %v", err)
		}

		if len(images) != 3 {
			t.Errorf("Expected 3 images from all namespaces, got %d", len(images))
		}

		// Verify we have images from all namespaces
		foundNamespaces := make(map[string]bool)
		for _, img := range images {
			foundNamespaces[img.Namespace] = true
		}

		for _, ns := range namespaces {
			if !foundNamespaces[ns] {
				t.Errorf("Missing image from namespace '%s'", ns)
			}
		}
	})

	t.Run("handle empty cluster", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()
		client := &Client{clientset: fakeClient}

		images, err := client.GetAllImages(context.Background(), "default")
		if err != nil {
			t.Fatalf("GetAllImages failed: %v", err)
		}

		if len(images) != 0 {
			t.Errorf("Expected 0 images from empty cluster, got %d", len(images))
		}
	})

	t.Run("handle multiple containers with same image", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()

		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "multi-container",
				Namespace: "default",
			},
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "container1", Image: "nginx:1.19"},
							{Name: "container2", Image: "nginx:1.19"},
							{Name: "container3", Image: "nginx:1.19"},
						},
					},
				},
			},
		}

		fakeClient.AppsV1().Deployments("default").Create(context.Background(), deployment, metav1.CreateOptions{})

		client := &Client{clientset: fakeClient}
		images, err := client.GetAllImages(context.Background(), "default")
		if err != nil {
			t.Fatalf("GetAllImages failed: %v", err)
		}

		// Should have only 1 image entry (deduplication)
		if len(images) != 1 {
			t.Errorf("Expected 1 image (deduplicated), got %d", len(images))
		}

		// But it should list all 3 containers
		if len(images[0].Containers) != 3 {
			t.Errorf("Expected 3 containers, got %d", len(images[0].Containers))
		}

		// Verify all container names are present
		expectedContainers := map[string]bool{
			"container1": false,
			"container2": false,
			"container3": false,
		}
		for _, containerName := range images[0].Containers {
			if _, exists := expectedContainers[containerName]; exists {
				expectedContainers[containerName] = true
			}
		}
		for name, found := range expectedContainers {
			if !found {
				t.Errorf("Container '%s' not found in image containers list", name)
			}
		}
	})

	t.Run("handle images with registry paths", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()

		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "private-registry",
				Namespace: "default",
			},
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "gcr", Image: "gcr.io/project/myapp:v1.0"},
							{Name: "ecr", Image: "123456.dkr.ecr.us-east-1.amazonaws.com/myapp:latest"},
							{Name: "dockerhub", Image: "library/nginx:1.19"},
						},
					},
				},
			},
		}

		fakeClient.AppsV1().Deployments("default").Create(context.Background(), deployment, metav1.CreateOptions{})

		client := &Client{clientset: fakeClient}
		images, err := client.GetAllImages(context.Background(), "default")
		if err != nil {
			t.Fatalf("GetAllImages failed: %v", err)
		}

		if len(images) != 3 {
			t.Errorf("Expected 3 images with different registries, got %d", len(images))
		}

		// Verify image names are parsed correctly
		imageNames := make(map[string]bool)
		for _, img := range images {
			imageNames[img.Name] = true
		}

		expectedNames := []string{"gcr.io/project/myapp", "123456.dkr.ecr.us-east-1.amazonaws.com/myapp", "library/nginx"}
		for _, expectedName := range expectedNames {
			if !imageNames[expectedName] {
				t.Errorf("Expected image name '%s' not found", expectedName)
			}
		}
	})
}

func TestGetClientset(t *testing.T) {
	t.Run("returns the underlying clientset", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()
		client := &Client{clientset: fakeClient}

		clientset := client.GetClientset()
		if clientset != fakeClient {
			t.Error("GetClientset should return the underlying clientset")
		}
	})
}
