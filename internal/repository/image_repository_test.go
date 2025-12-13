package repository

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/huseyinbabal/kubetag/internal/models"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	postgresDriver "gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// isDockerAvailable checks if Docker is running
func isDockerAvailable() bool {
	cmd := exec.Command("docker", "info")
	err := cmd.Run()
	return err == nil
}

// setupTestDB creates a PostgreSQL testcontainer for testing
func setupTestDB(t *testing.T) (*gorm.DB, func()) {
	// Skip if Docker is not available
	if testing.Short() {
		t.Skip("Skipping testcontainer tests in short mode")
	}

	if !isDockerAvailable() {
		t.Skip("Docker is not available - skipping testcontainer tests")
	}

	ctx := context.Background()

	// Create PostgreSQL container
	postgresContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("Failed to start postgres container: %v", err)
	}

	// Get connection string
	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to get connection string: %v", err)
	}

	// Connect to database
	db, err := gorm.Open(postgresDriver.Open(connStr), &gorm.Config{})
	if err != nil {
		postgresContainer.Terminate(ctx)
		t.Fatalf("Failed to connect to database: %v", err)
	}

	// Run migrations
	err = db.AutoMigrate(&models.Image{}, &models.ImageTag{})
	if err != nil {
		postgresContainer.Terminate(ctx)
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Return cleanup function
	cleanup := func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}

	return db, cleanup
}

func TestNewImageRepository(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewImageRepository(db)

	if repo == nil {
		t.Fatal("Expected non-nil repository")
	}
	if repo.db != db {
		t.Error("Repository db should match provided db")
	}
}

func TestUpsertImageTag(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewImageRepository(db)

	t.Run("Create new image and tag", func(t *testing.T) {
		err := repo.UpsertImageTag(
			"nginx",
			"docker.io",
			"1.19",
			"Deployment",
			"web-server",
			"default",
			"nginx",
		)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify image was created
		var image models.Image
		err = db.Where("name = ?", "nginx").First(&image).Error
		if err != nil {
			t.Fatalf("Failed to find image: %v", err)
		}

		if image.Name != "nginx" {
			t.Errorf("Expected image name 'nginx', got '%s'", image.Name)
		}
		if image.Repository != "docker.io" {
			t.Errorf("Expected repository 'docker.io', got '%s'", image.Repository)
		}
		if image.FullName != "docker.io/nginx" {
			t.Errorf("Expected full name 'docker.io/nginx', got '%s'", image.FullName)
		}

		// Verify image tag was created
		var imageTag models.ImageTag
		err = db.Where("image_id = ? AND tag = ?", image.ID, "1.19").First(&imageTag).Error
		if err != nil {
			t.Fatalf("Failed to find image tag: %v", err)
		}

		if imageTag.Tag != "1.19" {
			t.Errorf("Expected tag '1.19', got '%s'", imageTag.Tag)
		}
		if imageTag.ResourceType != "Deployment" {
			t.Errorf("Expected resource type 'Deployment', got '%s'", imageTag.ResourceType)
		}
		if imageTag.ResourceName != "web-server" {
			t.Errorf("Expected resource name 'web-server', got '%s'", imageTag.ResourceName)
		}
		if imageTag.Namespace != "default" {
			t.Errorf("Expected namespace 'default', got '%s'", imageTag.Namespace)
		}
		if imageTag.ContainerName != "nginx" {
			t.Errorf("Expected container name 'nginx', got '%s'", imageTag.ContainerName)
		}
	})

	t.Run("Update existing tag", func(t *testing.T) {
		// First insert
		err := repo.UpsertImageTag(
			"redis",
			"docker.io",
			"6.0",
			"Deployment",
			"cache",
			"default",
			"redis",
		)
		if err != nil {
			t.Fatalf("Failed to create initial tag: %v", err)
		}

		// Get first seen time
		var imageTag models.ImageTag
		db.Joins("JOIN images ON images.id = image_tags.image_id").
			Where("images.name = ? AND image_tags.tag = ?", "redis", "6.0").
			First(&imageTag)
		firstSeenTime := imageTag.FirstSeen

		// Wait a bit and upsert again
		time.Sleep(100 * time.Millisecond)

		err = repo.UpsertImageTag(
			"redis",
			"docker.io",
			"6.0",
			"Deployment",
			"cache",
			"default",
			"redis",
		)
		if err != nil {
			t.Fatalf("Failed to update tag: %v", err)
		}

		// Verify FirstSeen didn't change but LastSeen did
		var updatedTag models.ImageTag
		db.Joins("JOIN images ON images.id = image_tags.image_id").
			Where("images.name = ? AND image_tags.tag = ?", "redis", "6.0").
			First(&updatedTag)

		if !updatedTag.FirstSeen.Equal(firstSeenTime) {
			t.Error("FirstSeen should not change on update")
		}
		if !updatedTag.LastSeen.After(firstSeenTime) {
			t.Error("LastSeen should be updated on upsert")
		}
	})

	t.Run("Same image in different resources", func(t *testing.T) {
		err := repo.UpsertImageTag(
			"busybox",
			"docker.io",
			"latest",
			"Deployment",
			"app1",
			"default",
			"init",
		)
		if err != nil {
			t.Fatalf("Failed to create first tag: %v", err)
		}

		err = repo.UpsertImageTag(
			"busybox",
			"docker.io",
			"latest",
			"DaemonSet",
			"app2",
			"default",
			"init",
		)
		if err != nil {
			t.Fatalf("Failed to create second tag: %v", err)
		}

		// Should have 2 distinct image tags
		var count int64
		db.Model(&models.ImageTag{}).Where("tag = ?", "latest").Count(&count)
		if count != 2 {
			t.Errorf("Expected 2 image tags, got %d", count)
		}
	})
}

func TestDeleteImageTag(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewImageRepository(db)

	// Create some test data
	repo.UpsertImageTag("nginx", "docker.io", "1.19", "Deployment", "web", "default", "nginx")
	repo.UpsertImageTag("nginx", "docker.io", "1.20", "Deployment", "web", "default", "nginx")
	repo.UpsertImageTag("redis", "docker.io", "6.0", "Deployment", "cache", "default", "redis")

	t.Run("Delete specific resource tags", func(t *testing.T) {
		err := repo.DeleteImageTag("Deployment", "web", "default")
		if err != nil {
			t.Fatalf("Failed to delete tags: %v", err)
		}

		// Should soft delete nginx tags
		var count int64
		db.Model(&models.ImageTag{}).
			Where("resource_type = ? AND resource_name = ? AND namespace = ?", "Deployment", "web", "default").
			Count(&count)
		if count != 0 {
			t.Errorf("Expected 0 tags after deletion, got %d", count)
		}

		// Redis tag should still exist
		db.Model(&models.ImageTag{}).
			Where("resource_type = ? AND resource_name = ?", "Deployment", "cache").
			Count(&count)
		if count != 1 {
			t.Errorf("Expected redis tag to still exist, got count %d", count)
		}
	})

	t.Run("Delete non-existent resource", func(t *testing.T) {
		err := repo.DeleteImageTag("Deployment", "nonexistent", "default")
		if err != nil {
			t.Errorf("Deleting non-existent resource should not error, got %v", err)
		}
	})
}

func TestGetAllImages(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewImageRepository(db)

	// Create test data
	repo.UpsertImageTag("nginx", "docker.io", "1.19", "Deployment", "web", "default", "nginx")
	repo.UpsertImageTag("nginx", "docker.io", "1.19", "Deployment", "web", "default", "sidecar")
	repo.UpsertImageTag("redis", "docker.io", "6.0", "DaemonSet", "cache", "production", "redis")

	t.Run("Get all images without namespace filter", func(t *testing.T) {
		images, err := repo.GetAllImages("")
		if err != nil {
			t.Fatalf("Failed to get images: %v", err)
		}

		if len(images) != 2 {
			t.Errorf("Expected 2 images, got %d", len(images))
			for i, img := range images {
				t.Logf("Image %d: %+v", i, img)
			}
		}

		// Find nginx image
		var nginxImg *models.ImageInfo
		for i := range images {
			if images[i].Name == "nginx" {
				nginxImg = &images[i]
				break
			}
		}

		if nginxImg == nil {
			t.Fatal("Expected to find nginx image")
		}

		if nginxImg.Name != "nginx" {
			t.Errorf("Expected image name nginx, got %s", nginxImg.Name)
		}
	})

	t.Run("Get images with namespace filter", func(t *testing.T) {
		images, err := repo.GetAllImages("default")
		if err != nil {
			t.Fatalf("Failed to get images: %v", err)
		}

		if len(images) != 1 {
			t.Errorf("Expected 1 image in default namespace, got %d", len(images))
		}

		if images[0].Name != "nginx" {
			t.Errorf("Expected nginx image, got %s", images[0].Name)
		}
	})

	t.Run("Get images excludes deleted tags", func(t *testing.T) {
		// Delete the redis tag
		repo.DeleteImageTag("DaemonSet", "cache", "production")

		images, err := repo.GetAllImages("")
		if err != nil {
			t.Fatalf("Failed to get images: %v", err)
		}

		// Should only have nginx now
		if len(images) != 1 {
			t.Errorf("Expected 1 active image, got %d", len(images))
		}
		if images[0].Name != "nginx" {
			t.Errorf("Expected nginx, got %s", images[0].Name)
		}
	})
}

func TestGetImageTagHistory(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewImageRepository(db)

	// Create test data with version history
	repo.UpsertImageTag("myapp", "gcr.io", "v1.0", "Deployment", "api", "production", "app")
	time.Sleep(50 * time.Millisecond)
	repo.UpsertImageTag("myapp", "gcr.io", "v1.1", "Deployment", "api", "production", "app")
	time.Sleep(50 * time.Millisecond)
	repo.UpsertImageTag("myapp", "gcr.io", "v1.2", "Deployment", "api", "production", "app")

	t.Run("Get history without namespace filter", func(t *testing.T) {
		history, err := repo.GetImageTagHistory("myapp", "")
		if err != nil {
			t.Fatalf("Failed to get history: %v", err)
		}

		if history.ImageName != "myapp" {
			t.Errorf("Expected image name 'myapp', got '%s'", history.ImageName)
		}

		if len(history.Tags) != 3 {
			t.Errorf("Expected 3 tags, got %d", len(history.Tags))
		}

		// Tags should be ordered by FirstSeen DESC
		if history.Tags[0].Tag != "v1.2" {
			t.Errorf("Expected first tag to be 'v1.2', got '%s'", history.Tags[0].Tag)
		}

		// All tags should be active
		for _, tag := range history.Tags {
			if !tag.Active {
				t.Errorf("Expected tag %s to be active", tag.Tag)
			}
		}
	})

	t.Run("Get history with namespace filter", func(t *testing.T) {
		history, err := repo.GetImageTagHistory("myapp", "production")
		if err != nil {
			t.Fatalf("Failed to get history: %v", err)
		}

		if len(history.Tags) != 3 {
			t.Errorf("Expected 3 tags, got %d", len(history.Tags))
		}

		for _, tag := range history.Tags {
			if tag.Namespace != "production" {
				t.Errorf("Expected namespace 'production', got '%s'", tag.Namespace)
			}
		}
	})

	t.Run("Get history includes deleted tags", func(t *testing.T) {
		// Delete v1.0, v1.1, v1.2
		repo.DeleteImageTag("Deployment", "api", "production")

		// Create new version
		repo.UpsertImageTag("myapp", "gcr.io", "v1.3", "Deployment", "api", "production", "app")

		history, err := repo.GetImageTagHistory("myapp", "")
		if err != nil {
			t.Fatalf("Failed to get history: %v", err)
		}

		// Should still see v1.0, v1.1, v1.2 (deleted), v1.3 (active)
		if len(history.Tags) < 3 {
			t.Errorf("Expected at least 3 tags including deleted ones, got %d", len(history.Tags))
		}
	})

	t.Run("Non-existent image returns error", func(t *testing.T) {
		_, err := repo.GetImageTagHistory("nonexistent", "")
		if err == nil {
			t.Error("Expected error for non-existent image")
		}
	})
}

func TestConcurrentUpserts(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewImageRepository(db)

	// Test concurrent upserts to ensure database handles them correctly
	t.Run("Concurrent upserts of same image", func(t *testing.T) {
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func(idx int) {
				err := repo.UpsertImageTag(
					"concurrent-test",
					"docker.io",
					"v1.0",
					"Deployment",
					fmt.Sprintf("deployment-%d", idx),
					"default",
					"app",
				)
				if err != nil {
					t.Errorf("Failed to upsert image tag: %v", err)
				}
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}

		// Should have 10 distinct image tags (different resource names)
		var count int64
		db.Model(&models.ImageTag{}).
			Joins("JOIN images ON images.id = image_tags.image_id").
			Where("images.name = ?", "concurrent-test").
			Count(&count)

		if count != 10 {
			t.Errorf("Expected 10 image tags, got %d", count)
		}
	})
}

func TestPostgreSQLSpecificFeatures(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewImageRepository(db)

	t.Run("Upsert with conflict resolution", func(t *testing.T) {
		// Create initial tag
		err := repo.UpsertImageTag("postgres-test", "docker.io", "v1.0", "Deployment", "test", "default", "app")
		if err != nil {
			t.Fatalf("Failed to create initial tag: %v", err)
		}

		// Get initial LastSeen
		var tag1 models.ImageTag
		db.Joins("JOIN images ON images.id = image_tags.image_id").
			Where("images.name = ? AND image_tags.tag = ?", "postgres-test", "v1.0").
			First(&tag1)
		initialLastSeen := tag1.LastSeen

		time.Sleep(100 * time.Millisecond)

		// Upsert again - should update LastSeen
		err = repo.UpsertImageTag("postgres-test", "docker.io", "v1.0", "Deployment", "test", "default", "app")
		if err != nil {
			t.Fatalf("Failed to upsert tag: %v", err)
		}

		// Verify LastSeen was updated
		var tag2 models.ImageTag
		db.Joins("JOIN images ON images.id = image_tags.image_id").
			Where("images.name = ? AND image_tags.tag = ?", "postgres-test", "v1.0").
			First(&tag2)

		if !tag2.LastSeen.After(initialLastSeen) {
			t.Error("LastSeen should be updated after upsert")
		}
	})
}

// SQLite-based unit tests that don't require Docker

func setupSQLiteDB(t *testing.T) (*gorm.DB, func()) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}

	// Run migrations
	err = db.AutoMigrate(&models.Image{}, &models.ImageTag{})
	if err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	cleanup := func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}

	return db, cleanup
}

func TestNewImageRepositoryUnit(t *testing.T) {
	t.Run("creates repository with database connection", func(t *testing.T) {
		db, cleanup := setupSQLiteDB(t)
		defer cleanup()

		repo := NewImageRepository(db)

		if repo == nil {
			t.Error("Expected non-nil repository")
		}

		if repo.db == nil {
			t.Error("Expected repository to have database connection")
		}
	})
}

func TestUpsertImageTagUnit(t *testing.T) {
	t.Run("successfully creates new image and tag", func(t *testing.T) {
		db, cleanup := setupSQLiteDB(t)
		defer cleanup()

		repo := NewImageRepository(db)

		err := repo.UpsertImageTag("nginx", "docker.io", "latest", "Deployment", "nginx-deploy", "default", "nginx")
		if err != nil {
			t.Fatalf("Failed to upsert: %v", err)
		}

		// Verify image was created
		var image models.Image
		err = db.Where("name = ?", "nginx").First(&image).Error
		if err != nil {
			t.Fatalf("Failed to find image: %v", err)
		}

		if image.Name != "nginx" {
			t.Errorf("Expected image name 'nginx', got '%s'", image.Name)
		}

		if image.Repository != "docker.io" {
			t.Errorf("Expected repository 'docker.io', got '%s'", image.Repository)
		}

		if image.FullName != "docker.io/nginx" {
			t.Errorf("Expected full name 'docker.io/nginx', got '%s'", image.FullName)
		}

		// Verify tag was created
		var tag models.ImageTag
		err = db.Where("image_id = ? AND tag = ?", image.ID, "latest").First(&tag).Error
		if err != nil {
			t.Fatalf("Failed to find tag: %v", err)
		}

		if tag.Tag != "latest" {
			t.Errorf("Expected tag 'latest', got '%s'", tag.Tag)
		}
	})

	t.Run("updates LastSeen when upserting existing tag", func(t *testing.T) {
		db, cleanup := setupSQLiteDB(t)
		defer cleanup()

		repo := NewImageRepository(db)

		// First insert
		err := repo.UpsertImageTag("redis", "docker.io", "7.0", "Deployment", "redis-deploy", "default", "redis")
		if err != nil {
			t.Fatalf("Failed to upsert: %v", err)
		}

		// Get initial LastSeen
		var tag1 models.ImageTag
		db.Joins("JOIN images ON images.id = image_tags.image_id").
			Where("images.name = ? AND image_tags.tag = ?", "redis", "7.0").
			First(&tag1)
		initialLastSeen := tag1.LastSeen

		time.Sleep(10 * time.Millisecond)

		// Second insert (should update)
		err = repo.UpsertImageTag("redis", "docker.io", "7.0", "Deployment", "redis-deploy", "default", "redis")
		if err != nil {
			t.Fatalf("Failed to upsert: %v", err)
		}

		// Verify LastSeen was updated
		var tag2 models.ImageTag
		db.Joins("JOIN images ON images.id = image_tags.image_id").
			Where("images.name = ? AND image_tags.tag = ?", "redis", "7.0").
			First(&tag2)

		if !tag2.LastSeen.After(initialLastSeen) {
			t.Error("LastSeen should be updated after upsert")
		}
	})

	t.Run("handles multiple containers with same image", func(t *testing.T) {
		db, cleanup := setupSQLiteDB(t)
		defer cleanup()

		repo := NewImageRepository(db)

		// Insert same image for different containers
		err := repo.UpsertImageTag("nginx", "docker.io", "latest", "Deployment", "nginx-deploy", "default", "nginx")
		if err != nil {
			t.Fatalf("Failed to upsert: %v", err)
		}

		err = repo.UpsertImageTag("nginx", "docker.io", "latest", "Deployment", "nginx-deploy", "default", "sidecar")
		if err != nil {
			t.Fatalf("Failed to upsert: %v", err)
		}

		// Should have 2 tags (one per container)
		var count int64
		db.Model(&models.ImageTag{}).Count(&count)
		if count != 2 {
			t.Errorf("Expected 2 tags, got %d", count)
		}
	})
}

func TestDeleteImageTagUnit(t *testing.T) {
	t.Run("successfully deletes image tag", func(t *testing.T) {
		db, cleanup := setupSQLiteDB(t)
		defer cleanup()

		repo := NewImageRepository(db)

		// Insert tag
		err := repo.UpsertImageTag("nginx", "docker.io", "latest", "Deployment", "nginx-deploy", "default", "nginx")
		if err != nil {
			t.Fatalf("Failed to upsert: %v", err)
		}

		// Delete tag
		err = repo.DeleteImageTag("Deployment", "nginx-deploy", "default")
		if err != nil {
			t.Fatalf("Failed to delete: %v", err)
		}

		// Verify tag is soft deleted
		var count int64
		db.Model(&models.ImageTag{}).Where("deleted_at IS NULL").Count(&count)
		if count != 0 {
			t.Errorf("Expected 0 active tags, got %d", count)
		}
	})

	t.Run("deletes multiple tags for same resource", func(t *testing.T) {
		db, cleanup := setupSQLiteDB(t)
		defer cleanup()

		repo := NewImageRepository(db)

		// Insert multiple tags for same resource
		repo.UpsertImageTag("nginx", "docker.io", "latest", "Deployment", "nginx-deploy", "default", "nginx")
		repo.UpsertImageTag("nginx", "docker.io", "latest", "Deployment", "nginx-deploy", "default", "sidecar")

		// Delete all tags for resource
		err := repo.DeleteImageTag("Deployment", "nginx-deploy", "default")
		if err != nil {
			t.Fatalf("Failed to delete: %v", err)
		}

		// Verify all tags are deleted
		var count int64
		db.Model(&models.ImageTag{}).Where("deleted_at IS NULL").Count(&count)
		if count != 0 {
			t.Errorf("Expected 0 active tags, got %d", count)
		}
	})
}

func TestGetAllImagesUnit(t *testing.T) {
	t.Run("returns all images", func(t *testing.T) {
		db, cleanup := setupSQLiteDB(t)
		defer cleanup()

		repo := NewImageRepository(db)

		// Insert test data
		repo.UpsertImageTag("nginx", "docker.io", "latest", "Deployment", "nginx-deploy", "default", "nginx")
		repo.UpsertImageTag("redis", "docker.io", "7.0", "Deployment", "redis-deploy", "default", "redis")

		// Get all images
		images, err := repo.GetAllImages("")
		if err != nil {
			t.Fatalf("Failed to get images: %v", err)
		}

		if len(images) != 2 {
			t.Errorf("Expected 2 images, got %d", len(images))
		}
	})

	t.Run("filters by namespace", func(t *testing.T) {
		db, cleanup := setupSQLiteDB(t)
		defer cleanup()

		repo := NewImageRepository(db)

		// Insert test data in different namespaces
		repo.UpsertImageTag("nginx", "docker.io", "latest", "Deployment", "nginx-deploy", "default", "nginx")
		repo.UpsertImageTag("redis", "docker.io", "7.0", "Deployment", "redis-deploy", "production", "redis")

		// Get images for specific namespace
		images, err := repo.GetAllImages("default")
		if err != nil {
			t.Fatalf("Failed to get images: %v", err)
		}

		if len(images) != 1 {
			t.Errorf("Expected 1 image, got %d", len(images))
		}

		if images[0].Name != "nginx" {
			t.Errorf("Expected nginx, got %s", images[0].Name)
		}
	})

	t.Run("returns latest tag per resource", func(t *testing.T) {
		db, cleanup := setupSQLiteDB(t)
		defer cleanup()

		repo := NewImageRepository(db)

		// Insert same image with different tags for same resource
		repo.UpsertImageTag("nginx", "docker.io", "1.20", "Deployment", "nginx-deploy", "default", "nginx")
		time.Sleep(10 * time.Millisecond)
		repo.UpsertImageTag("nginx", "docker.io", "1.21", "Deployment", "nginx-deploy", "default", "nginx")

		// Get all images - should return only the latest tag
		images, err := repo.GetAllImages("")
		if err != nil {
			t.Fatalf("Failed to get images: %v", err)
		}

		if len(images) != 1 {
			t.Errorf("Expected 1 image, got %d", len(images))
		}

		// Should have the latest tag (1.21)
		if images[0].Tag != "1.21" {
			t.Errorf("Expected tag '1.21', got '%s'", images[0].Tag)
		}
	})

	t.Run("handles multiple resources with same image", func(t *testing.T) {
		db, cleanup := setupSQLiteDB(t)
		defer cleanup()

		repo := NewImageRepository(db)

		// Insert same image in different resources
		repo.UpsertImageTag("nginx", "docker.io", "latest", "Deployment", "nginx-deploy-1", "default", "nginx")
		repo.UpsertImageTag("nginx", "docker.io", "latest", "Deployment", "nginx-deploy-2", "default", "nginx")

		// Get all images - should return 2 separate entries (one per resource)
		images, err := repo.GetAllImages("")
		if err != nil {
			t.Fatalf("Failed to get images: %v", err)
		}

		if len(images) != 2 {
			t.Errorf("Expected 2 images (one per resource), got %d", len(images))
		}
	})

	t.Run("returns empty list when no images", func(t *testing.T) {
		db, cleanup := setupSQLiteDB(t)
		defer cleanup()

		repo := NewImageRepository(db)

		images, err := repo.GetAllImages("")
		if err != nil {
			t.Fatalf("Failed to get images: %v", err)
		}

		if len(images) != 0 {
			t.Errorf("Expected 0 images, got %d", len(images))
		}
	})
}

func TestGetImageTagHistoryUnit(t *testing.T) {
	t.Run("returns tag history for image", func(t *testing.T) {
		db, cleanup := setupSQLiteDB(t)
		defer cleanup()

		repo := NewImageRepository(db)

		// Insert multiple tags
		repo.UpsertImageTag("nginx", "docker.io", "1.20", "Deployment", "nginx-v1", "default", "nginx")
		time.Sleep(10 * time.Millisecond)
		repo.UpsertImageTag("nginx", "docker.io", "1.21", "Deployment", "nginx-v2", "default", "nginx")

		// Get history
		history, err := repo.GetImageTagHistory("nginx", "")
		if err != nil {
			t.Fatalf("Failed to get history: %v", err)
		}

		if history.ImageName != "nginx" {
			t.Errorf("Expected image name 'nginx', got '%s'", history.ImageName)
		}

		if len(history.Tags) != 2 {
			t.Errorf("Expected 2 tags, got %d", len(history.Tags))
		}
	})

	t.Run("filters history by namespace", func(t *testing.T) {
		db, cleanup := setupSQLiteDB(t)
		defer cleanup()

		repo := NewImageRepository(db)

		// Insert tags in different namespaces
		repo.UpsertImageTag("nginx", "docker.io", "latest", "Deployment", "nginx-deploy", "default", "nginx")
		repo.UpsertImageTag("nginx", "docker.io", "latest", "Deployment", "nginx-deploy", "production", "nginx")

		// Get history for specific namespace
		history, err := repo.GetImageTagHistory("nginx", "default")
		if err != nil {
			t.Fatalf("Failed to get history: %v", err)
		}

		if len(history.Tags) != 1 {
			t.Errorf("Expected 1 tag, got %d", len(history.Tags))
		}

		if history.Tags[0].Namespace != "default" {
			t.Errorf("Expected namespace 'default', got '%s'", history.Tags[0].Namespace)
		}
	})

	t.Run("marks deleted tags as inactive", func(t *testing.T) {
		db, cleanup := setupSQLiteDB(t)
		defer cleanup()

		repo := NewImageRepository(db)

		// Insert and then delete a tag
		repo.UpsertImageTag("nginx", "docker.io", "old", "Deployment", "nginx-old", "default", "nginx")
		repo.DeleteImageTag("Deployment", "nginx-old", "default")

		// Insert an active tag
		repo.UpsertImageTag("nginx", "docker.io", "latest", "Deployment", "nginx-new", "default", "nginx")

		// Get history
		history, err := repo.GetImageTagHistory("nginx", "")
		if err != nil {
			t.Fatalf("Failed to get history: %v", err)
		}

		// Find the old tag
		var oldTagActive, latestTagActive bool
		for _, tag := range history.Tags {
			if tag.Tag == "old" {
				oldTagActive = tag.Active
			}
			if tag.Tag == "latest" {
				latestTagActive = tag.Active
			}
		}

		if oldTagActive {
			t.Error("Expected old tag to be inactive")
		}

		if !latestTagActive {
			t.Error("Expected latest tag to be active")
		}
	})

	t.Run("returns error for non-existent image", func(t *testing.T) {
		db, cleanup := setupSQLiteDB(t)
		defer cleanup()

		repo := NewImageRepository(db)

		_, err := repo.GetImageTagHistory("nonexistent", "")
		if err == nil {
			t.Error("Expected error for non-existent image")
		}
	})
}
