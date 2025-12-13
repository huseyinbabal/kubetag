package models

import (
	"encoding/json"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestImageStruct(t *testing.T) {
	t.Run("Image struct has correct fields", func(t *testing.T) {
		now := time.Now()
		img := Image{
			ID:         1,
			CreatedAt:  now,
			UpdatedAt:  now,
			Name:       "nginx",
			Repository: "docker.io",
			FullName:   "docker.io/nginx",
		}

		if img.ID != 1 {
			t.Errorf("Expected ID 1, got %d", img.ID)
		}
		if img.Name != "nginx" {
			t.Errorf("Expected name 'nginx', got '%s'", img.Name)
		}
		if img.Repository != "docker.io" {
			t.Errorf("Expected repository 'docker.io', got '%s'", img.Repository)
		}
		if img.FullName != "docker.io/nginx" {
			t.Errorf("Expected full name 'docker.io/nginx', got '%s'", img.FullName)
		}
	})

	t.Run("Image struct can be JSON serialized", func(t *testing.T) {
		img := Image{
			ID:         1,
			Name:       "nginx",
			Repository: "docker.io",
			FullName:   "docker.io/nginx",
		}

		data, err := json.Marshal(img)
		if err != nil {
			t.Fatalf("Failed to marshal image: %v", err)
		}

		if len(data) == 0 {
			t.Error("Expected non-empty JSON data")
		}

		// Verify it can be unmarshaled
		var decoded Image
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Fatalf("Failed to unmarshal image: %v", err)
		}

		if decoded.Name != img.Name {
			t.Errorf("Expected name '%s', got '%s'", img.Name, decoded.Name)
		}
	})

	t.Run("Image struct works with GORM", func(t *testing.T) {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			t.Fatalf("Failed to open database: %v", err)
		}

		err = db.AutoMigrate(&Image{})
		if err != nil {
			t.Fatalf("Failed to migrate: %v", err)
		}

		img := Image{
			Name:       "redis",
			Repository: "docker.io",
			FullName:   "docker.io/redis",
		}

		err = db.Create(&img).Error
		if err != nil {
			t.Fatalf("Failed to create image: %v", err)
		}

		if img.ID == 0 {
			t.Error("Expected auto-generated ID")
		}
	})
}

func TestImageTagStruct(t *testing.T) {
	t.Run("ImageTag struct has correct fields", func(t *testing.T) {
		now := time.Now()
		tag := ImageTag{
			ID:            1,
			ImageID:       1,
			Tag:           "latest",
			FirstSeen:     now,
			LastSeen:      now,
			ResourceType:  "Deployment",
			ResourceName:  "nginx-deploy",
			Namespace:     "default",
			ContainerName: "nginx",
		}

		if tag.ID != 1 {
			t.Errorf("Expected ID 1, got %d", tag.ID)
		}
		if tag.Tag != "latest" {
			t.Errorf("Expected tag 'latest', got '%s'", tag.Tag)
		}
		if tag.ResourceType != "Deployment" {
			t.Errorf("Expected resource type 'Deployment', got '%s'", tag.ResourceType)
		}
	})

	t.Run("ImageTag TableName returns correct name", func(t *testing.T) {
		tag := ImageTag{}
		tableName := tag.TableName()

		if tableName != "image_tags" {
			t.Errorf("Expected table name 'image_tags', got '%s'", tableName)
		}
	})

	t.Run("ImageTag struct can be JSON serialized", func(t *testing.T) {
		now := time.Now()
		tag := ImageTag{
			ID:            1,
			ImageID:       1,
			Tag:           "v1.0",
			FirstSeen:     now,
			LastSeen:      now,
			ResourceType:  "Deployment",
			ResourceName:  "app",
			Namespace:     "default",
			ContainerName: "main",
		}

		data, err := json.Marshal(tag)
		if err != nil {
			t.Fatalf("Failed to marshal tag: %v", err)
		}

		if len(data) == 0 {
			t.Error("Expected non-empty JSON data")
		}

		// Verify it can be unmarshaled
		var decoded ImageTag
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Fatalf("Failed to unmarshal tag: %v", err)
		}

		if decoded.Tag != tag.Tag {
			t.Errorf("Expected tag '%s', got '%s'", tag.Tag, decoded.Tag)
		}
	})

	t.Run("ImageTag struct works with GORM", func(t *testing.T) {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			t.Fatalf("Failed to open database: %v", err)
		}

		err = db.AutoMigrate(&Image{}, &ImageTag{})
		if err != nil {
			t.Fatalf("Failed to migrate: %v", err)
		}

		// Create image first
		img := Image{
			Name:       "postgres",
			Repository: "docker.io",
			FullName:   "docker.io/postgres",
		}
		db.Create(&img)

		// Create tag
		tag := ImageTag{
			ImageID:       img.ID,
			Tag:           "14",
			FirstSeen:     time.Now(),
			LastSeen:      time.Now(),
			ResourceType:  "StatefulSet",
			ResourceName:  "postgres",
			Namespace:     "database",
			ContainerName: "postgres",
		}

		err = db.Create(&tag).Error
		if err != nil {
			t.Fatalf("Failed to create tag: %v", err)
		}

		if tag.ID == 0 {
			t.Error("Expected auto-generated ID")
		}
	})
}

func TestImageInfoStruct(t *testing.T) {
	t.Run("ImageInfo struct has correct fields", func(t *testing.T) {
		info := ImageInfo{
			Name:         "nginx",
			Tag:          "latest",
			ResourceType: "Deployment",
			ResourceName: "nginx-deploy",
			Namespace:    "default",
			Containers:   []string{"nginx", "sidecar"},
			FirstSeen:    "2024-01-01T00:00:00Z",
			LastSeen:     "2024-01-02T00:00:00Z",
		}

		if info.Name != "nginx" {
			t.Errorf("Expected name 'nginx', got '%s'", info.Name)
		}
		if len(info.Containers) != 2 {
			t.Errorf("Expected 2 containers, got %d", len(info.Containers))
		}
	})

	t.Run("ImageInfo struct can be JSON serialized", func(t *testing.T) {
		info := ImageInfo{
			Name:         "redis",
			Tag:          "7.0",
			ResourceType: "Deployment",
			ResourceName: "redis-deploy",
			Namespace:    "cache",
			Containers:   []string{"redis"},
			FirstSeen:    "2024-01-01T00:00:00Z",
			LastSeen:     "2024-01-02T00:00:00Z",
		}

		data, err := json.Marshal(info)
		if err != nil {
			t.Fatalf("Failed to marshal info: %v", err)
		}

		if len(data) == 0 {
			t.Error("Expected non-empty JSON data")
		}

		// Verify it can be unmarshaled
		var decoded ImageInfo
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Fatalf("Failed to unmarshal info: %v", err)
		}

		if decoded.Name != info.Name {
			t.Errorf("Expected name '%s', got '%s'", info.Name, decoded.Name)
		}
	})

	t.Run("ImageInfo handles empty containers list", func(t *testing.T) {
		info := ImageInfo{
			Name:       "nginx",
			Tag:        "latest",
			Containers: []string{},
		}

		if info.Containers == nil {
			t.Error("Expected non-nil containers slice")
		}

		if len(info.Containers) != 0 {
			t.Errorf("Expected 0 containers, got %d", len(info.Containers))
		}
	})
}

func TestImagesResponseStruct(t *testing.T) {
	t.Run("ImagesResponse struct has correct fields", func(t *testing.T) {
		resp := ImagesResponse{
			Images: []ImageInfo{
				{Name: "nginx", Tag: "latest"},
				{Name: "redis", Tag: "7.0"},
			},
			Total: 2,
		}

		if len(resp.Images) != 2 {
			t.Errorf("Expected 2 images, got %d", len(resp.Images))
		}
		if resp.Total != 2 {
			t.Errorf("Expected total 2, got %d", resp.Total)
		}
	})

	t.Run("ImagesResponse struct can be JSON serialized", func(t *testing.T) {
		resp := ImagesResponse{
			Images: []ImageInfo{
				{Name: "nginx", Tag: "latest", Namespace: "default"},
			},
			Total: 1,
		}

		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("Failed to marshal response: %v", err)
		}

		if len(data) == 0 {
			t.Error("Expected non-empty JSON data")
		}

		// Verify it can be unmarshaled
		var decoded ImagesResponse
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if decoded.Total != resp.Total {
			t.Errorf("Expected total %d, got %d", resp.Total, decoded.Total)
		}
	})

	t.Run("ImagesResponse handles empty images list", func(t *testing.T) {
		resp := ImagesResponse{
			Images: []ImageInfo{},
			Total:  0,
		}

		if resp.Images == nil {
			t.Error("Expected non-nil images slice")
		}

		if len(resp.Images) != 0 {
			t.Errorf("Expected 0 images, got %d", len(resp.Images))
		}
	})
}

func TestImageTagHistoryStruct(t *testing.T) {
	t.Run("ImageTagHistory struct has correct fields", func(t *testing.T) {
		history := ImageTagHistory{
			ImageName: "nginx",
			Tags: []ImageTagDetails{
				{Tag: "1.20", Active: true},
				{Tag: "1.21", Active: true},
			},
		}

		if history.ImageName != "nginx" {
			t.Errorf("Expected image name 'nginx', got '%s'", history.ImageName)
		}
		if len(history.Tags) != 2 {
			t.Errorf("Expected 2 tags, got %d", len(history.Tags))
		}
	})

	t.Run("ImageTagHistory struct can be JSON serialized", func(t *testing.T) {
		history := ImageTagHistory{
			ImageName: "redis",
			Tags: []ImageTagDetails{
				{Tag: "7.0", Active: true, Namespace: "default"},
			},
		}

		data, err := json.Marshal(history)
		if err != nil {
			t.Fatalf("Failed to marshal history: %v", err)
		}

		if len(data) == 0 {
			t.Error("Expected non-empty JSON data")
		}

		// Verify it can be unmarshaled
		var decoded ImageTagHistory
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Fatalf("Failed to unmarshal history: %v", err)
		}

		if decoded.ImageName != history.ImageName {
			t.Errorf("Expected image name '%s', got '%s'", history.ImageName, decoded.ImageName)
		}
	})
}

func TestImageTagDetailsStruct(t *testing.T) {
	t.Run("ImageTagDetails struct has correct fields", func(t *testing.T) {
		now := time.Now()
		details := ImageTagDetails{
			Tag:          "v1.0",
			FirstSeen:    now,
			LastSeen:     now,
			ResourceType: "Deployment",
			ResourceName: "app",
			Namespace:    "default",
			Container:    "main",
			Active:       true,
		}

		if details.Tag != "v1.0" {
			t.Errorf("Expected tag 'v1.0', got '%s'", details.Tag)
		}
		if !details.Active {
			t.Error("Expected active to be true")
		}
	})

	t.Run("ImageTagDetails struct can be JSON serialized", func(t *testing.T) {
		now := time.Now()
		details := ImageTagDetails{
			Tag:          "latest",
			FirstSeen:    now,
			LastSeen:     now,
			ResourceType: "DaemonSet",
			ResourceName: "logger",
			Namespace:    "kube-system",
			Container:    "fluentd",
			Active:       false,
		}

		data, err := json.Marshal(details)
		if err != nil {
			t.Fatalf("Failed to marshal details: %v", err)
		}

		if len(data) == 0 {
			t.Error("Expected non-empty JSON data")
		}

		// Verify it can be unmarshaled
		var decoded ImageTagDetails
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Fatalf("Failed to unmarshal details: %v", err)
		}

		if decoded.Tag != details.Tag {
			t.Errorf("Expected tag '%s', got '%s'", details.Tag, decoded.Tag)
		}
		if decoded.Active != details.Active {
			t.Errorf("Expected active %v, got %v", details.Active, decoded.Active)
		}
	})
}

func TestImageRelationship(t *testing.T) {
	t.Run("Image can have multiple ImageTags", func(t *testing.T) {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			t.Fatalf("Failed to open database: %v", err)
		}

		err = db.AutoMigrate(&Image{}, &ImageTag{})
		if err != nil {
			t.Fatalf("Failed to migrate: %v", err)
		}

		// Create image
		img := Image{
			Name:       "nginx",
			Repository: "docker.io",
			FullName:   "docker.io/nginx",
		}
		db.Create(&img)

		// Create multiple tags
		tag1 := ImageTag{
			ImageID:       img.ID,
			Tag:           "1.20",
			FirstSeen:     time.Now(),
			LastSeen:      time.Now(),
			ResourceType:  "Deployment",
			ResourceName:  "nginx-v1",
			Namespace:     "default",
			ContainerName: "nginx",
		}
		tag2 := ImageTag{
			ImageID:       img.ID,
			Tag:           "1.21",
			FirstSeen:     time.Now(),
			LastSeen:      time.Now(),
			ResourceType:  "Deployment",
			ResourceName:  "nginx-v2",
			Namespace:     "default",
			ContainerName: "nginx",
		}

		db.Create(&tag1)
		db.Create(&tag2)

		// Load image with tags
		var loadedImage Image
		db.Preload("ImageTags").First(&loadedImage, img.ID)

		if len(loadedImage.ImageTags) != 2 {
			t.Errorf("Expected 2 tags, got %d", len(loadedImage.ImageTags))
		}
	})
}
