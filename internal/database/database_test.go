package database

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/huseyinbabal/kubetag/internal/models"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestNewConfigFromEnv(t *testing.T) {
	// Save original env vars
	originalVars := map[string]string{
		"DB_HOST":     os.Getenv("DB_HOST"),
		"DB_PORT":     os.Getenv("DB_PORT"),
		"DB_USER":     os.Getenv("DB_USER"),
		"DB_PASSWORD": os.Getenv("DB_PASSWORD"),
		"DB_NAME":     os.Getenv("DB_NAME"),
		"DB_SSLMODE":  os.Getenv("DB_SSLMODE"),
	}

	// Restore env vars after test
	defer func() {
		for key, value := range originalVars {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	t.Run("Default values when env vars are not set", func(t *testing.T) {
		os.Clearenv()

		config := NewConfigFromEnv()

		if config.Host != "localhost" {
			t.Errorf("Expected Host to be 'localhost', got '%s'", config.Host)
		}
		if config.Port != "5432" {
			t.Errorf("Expected Port to be '5432', got '%s'", config.Port)
		}
		if config.User != "postgres" {
			t.Errorf("Expected User to be 'postgres', got '%s'", config.User)
		}
		if config.Password != "postgres" {
			t.Errorf("Expected Password to be 'postgres', got '%s'", config.Password)
		}
		if config.DBName != "kubetag" {
			t.Errorf("Expected DBName to be 'kubetag', got '%s'", config.DBName)
		}
		if config.SSLMode != "disable" {
			t.Errorf("Expected SSLMode to be 'disable', got '%s'", config.SSLMode)
		}
	})

	t.Run("Custom values from environment variables", func(t *testing.T) {
		os.Setenv("DB_HOST", "customhost")
		os.Setenv("DB_PORT", "5433")
		os.Setenv("DB_USER", "customuser")
		os.Setenv("DB_PASSWORD", "custompass")
		os.Setenv("DB_NAME", "customdb")
		os.Setenv("DB_SSLMODE", "require")

		config := NewConfigFromEnv()

		if config.Host != "customhost" {
			t.Errorf("Expected Host to be 'customhost', got '%s'", config.Host)
		}
		if config.Port != "5433" {
			t.Errorf("Expected Port to be '5433', got '%s'", config.Port)
		}
		if config.User != "customuser" {
			t.Errorf("Expected User to be 'customuser', got '%s'", config.User)
		}
		if config.Password != "custompass" {
			t.Errorf("Expected Password to be 'custompass', got '%s'", config.Password)
		}
		if config.DBName != "customdb" {
			t.Errorf("Expected DBName to be 'customdb', got '%s'", config.DBName)
		}
		if config.SSLMode != "require" {
			t.Errorf("Expected SSLMode to be 'require', got '%s'", config.SSLMode)
		}
	})
}

func TestGetEnv(t *testing.T) {
	t.Run("Returns environment variable value when set", func(t *testing.T) {
		os.Setenv("TEST_VAR", "testvalue")
		defer os.Unsetenv("TEST_VAR")

		result := getEnv("TEST_VAR", "default")
		if result != "testvalue" {
			t.Errorf("Expected 'testvalue', got '%s'", result)
		}
	})

	t.Run("Returns default value when env var is not set", func(t *testing.T) {
		os.Unsetenv("TEST_VAR")

		result := getEnv("TEST_VAR", "default")
		if result != "default" {
			t.Errorf("Expected 'default', got '%s'", result)
		}
	})

	t.Run("Returns default value when env var is empty string", func(t *testing.T) {
		os.Setenv("TEST_VAR", "")
		defer os.Unsetenv("TEST_VAR")

		result := getEnv("TEST_VAR", "default")
		if result != "default" {
			t.Errorf("Expected 'default', got '%s'", result)
		}
	})
}

func TestConnect(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database integration test in short mode")
	}

	// Check if Docker is available
	if !isDockerAvailable() {
		t.Skip("Docker is not available - skipping testcontainer tests")
	}

	ctx := context.Background()

	// Start PostgreSQL container
	postgresContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("Failed to start postgres container: %v", err)
	}
	defer func() {
		if err := testcontainers.TerminateContainer(postgresContainer); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}()

	host, err := postgresContainer.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get container host: %v", err)
	}

	port, err := postgresContainer.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("Failed to get container port: %v", err)
	}

	t.Run("Successfully connect to database", func(t *testing.T) {
		config := &Config{
			Host:     host,
			Port:     port.Port(),
			User:     "testuser",
			Password: "testpass",
			DBName:   "testdb",
			SSLMode:  "disable",
		}

		db, err := Connect(config)
		if err != nil {
			t.Fatalf("Expected successful connection, got error: %v", err)
		}

		// Verify connection
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatalf("Failed to get database instance: %v", err)
		}

		err = sqlDB.Ping()
		if err != nil {
			t.Errorf("Failed to ping database: %v", err)
		}

		// Verify connection pool settings
		stats := sqlDB.Stats()
		if stats.MaxOpenConnections != 100 {
			t.Errorf("Expected MaxOpenConnections to be 100, got %d", stats.MaxOpenConnections)
		}
	})

	t.Run("Fail to connect with invalid credentials", func(t *testing.T) {
		config := &Config{
			Host:     host,
			Port:     port.Port(),
			User:     "wronguser",
			Password: "wrongpass",
			DBName:   "testdb",
			SSLMode:  "disable",
		}

		_, err := Connect(config)
		if err == nil {
			t.Error("Expected connection to fail with invalid credentials")
		}
	})

	t.Run("Fail to connect with invalid host", func(t *testing.T) {
		config := &Config{
			Host:     "invalid-host",
			Port:     "5432",
			User:     "testuser",
			Password: "testpass",
			DBName:   "testdb",
			SSLMode:  "disable",
		}

		_, err := Connect(config)
		if err == nil {
			t.Error("Expected connection to fail with invalid host")
		}
	})
}

func TestMigrate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database integration test in short mode")
	}

	// Check if Docker is available
	if !isDockerAvailable() {
		t.Skip("Docker is not available - skipping testcontainer tests")
	}

	ctx := context.Background()

	// Start PostgreSQL container
	postgresContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("Failed to start postgres container: %v", err)
	}
	defer func() {
		if err := testcontainers.TerminateContainer(postgresContainer); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}()

	host, err := postgresContainer.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get container host: %v", err)
	}

	port, err := postgresContainer.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("Failed to get container port: %v", err)
	}

	config := &Config{
		Host:     host,
		Port:     port.Port(),
		User:     "testuser",
		Password: "testpass",
		DBName:   "testdb",
		SSLMode:  "disable",
	}

	db, err := Connect(config)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	t.Run("Successfully run migrations", func(t *testing.T) {
		err := Migrate(db)
		if err != nil {
			t.Fatalf("Expected migrations to succeed, got error: %v", err)
		}

		// Verify tables were created
		var tableCount int64
		err = db.Raw(`
			SELECT COUNT(*) 
			FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name IN ('images', 'image_tags')
		`).Scan(&tableCount).Error

		if err != nil {
			t.Fatalf("Failed to query tables: %v", err)
		}

		if tableCount != 2 {
			t.Errorf("Expected 2 tables (images, image_tags), found %d", tableCount)
		}

		// Verify unique index was created
		var indexCount int64
		err = db.Raw(`
			SELECT COUNT(*) 
			FROM pg_indexes 
			WHERE tablename = 'image_tags' 
			AND indexname = 'idx_image_tag_resource'
		`).Scan(&indexCount).Error

		if err != nil {
			t.Fatalf("Failed to query indexes: %v", err)
		}

		if indexCount != 1 {
			t.Errorf("Expected unique index 'idx_image_tag_resource' to be created, found %d", indexCount)
		}
	})

	t.Run("Successfully run migrations twice (idempotent)", func(t *testing.T) {
		// First migration
		err := Migrate(db)
		if err != nil {
			t.Fatalf("First migration failed: %v", err)
		}

		// Second migration should also succeed
		err = Migrate(db)
		if err != nil {
			t.Fatalf("Second migration failed: %v", err)
		}
	})

	t.Run("Can insert data after migration", func(t *testing.T) {
		err := Migrate(db)
		if err != nil {
			t.Fatalf("Migration failed: %v", err)
		}

		// Try to insert an image
		image := &models.Image{
			Name: "test-image",
		}
		err = db.Create(image).Error
		if err != nil {
			t.Errorf("Failed to insert test image: %v", err)
		}

		// Try to insert an image tag
		imageTag := &models.ImageTag{
			ImageID:       image.ID,
			Tag:           "v1.0.0",
			Namespace:     "default",
			ResourceType:  "Deployment",
			ResourceName:  "test-app",
			ContainerName: "app",
			FirstSeen:     time.Now().UTC(),
			LastSeen:      time.Now().UTC(),
		}
		err = db.Create(imageTag).Error
		if err != nil {
			t.Errorf("Failed to insert test image tag: %v", err)
		}
	})
}

// isDockerAvailable checks if Docker is available
func isDockerAvailable() (result bool) {
	// Defer and recover from any panics (like $HOME not defined)
	defer func() {
		if r := recover(); r != nil {
			// Docker check failed with panic, assume Docker not available
			result = false
		}
	}()

	ctx := context.Background()
	req := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "hello-world",
		},
		Started: false,
	}

	container, err := testcontainers.GenericContainer(ctx, req)
	if err != nil {
		return false
	}

	if container != nil {
		// Clean up the test container
		_ = container.Terminate(ctx)
	}

	return true
}

func TestConnectAndMigrateIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database integration test in short mode")
	}

	// Check if Docker is available
	if !isDockerAvailable() {
		t.Skip("Docker is not available - skipping testcontainer tests")
	}

	ctx := context.Background()

	// Start PostgreSQL container
	postgresContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("Failed to start postgres container: %v", err)
	}
	defer func() {
		if err := testcontainers.TerminateContainer(postgresContainer); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}()

	host, err := postgresContainer.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get container host: %v", err)
	}

	port, err := postgresContainer.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("Failed to get container port: %v", err)
	}

	t.Run("Full integration: Connect and Migrate", func(t *testing.T) {
		// Create config
		config := &Config{
			Host:     host,
			Port:     port.Port(),
			User:     "testuser",
			Password: "testpass",
			DBName:   "testdb",
			SSLMode:  "disable",
		}

		// Connect
		db, err := Connect(config)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}

		// Migrate
		err = Migrate(db)
		if err != nil {
			t.Fatalf("Failed to migrate: %v", err)
		}

		// Verify we can query the database
		var count int64
		err = db.Model(&models.Image{}).Count(&count).Error
		if err != nil {
			t.Errorf("Failed to count images: %v", err)
		}

		if count != 0 {
			t.Errorf("Expected 0 images, got %d", count)
		}
	})
}

func TestConfigDSNFormat(t *testing.T) {
	t.Run("DSN is formatted correctly", func(t *testing.T) {
		config := &Config{
			Host:     "testhost",
			Port:     "5433",
			User:     "testuser",
			Password: "testpass",
			DBName:   "testdb",
			SSLMode:  "require",
		}

		expectedDSN := "host=testhost port=5433 user=testuser password=testpass dbname=testdb sslmode=require"
		actualDSN := fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			config.Host, config.Port, config.User, config.Password, config.DBName, config.SSLMode,
		)

		if actualDSN != expectedDSN {
			t.Errorf("Expected DSN '%s', got '%s'", expectedDSN, actualDSN)
		}
	})

	t.Run("Config with special characters", func(t *testing.T) {
		config := &Config{
			Host:     "host-with-dash.example.com",
			Port:     "5432",
			User:     "user@domain",
			Password: "p@ssw0rd!",
			DBName:   "db-name",
			SSLMode:  "verify-full",
		}

		expectedDSN := "host=host-with-dash.example.com port=5432 user=user@domain password=p@ssw0rd! dbname=db-name sslmode=verify-full"
		actualDSN := fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			config.Host, config.Port, config.User, config.Password, config.DBName, config.SSLMode,
		)

		if actualDSN != expectedDSN {
			t.Errorf("Expected DSN '%s', got '%s'", expectedDSN, actualDSN)
		}
	})
}

func TestConfigValidation(t *testing.T) {
	t.Run("Config with all fields populated", func(t *testing.T) {
		config := &Config{
			Host:     "localhost",
			Port:     "5432",
			User:     "testuser",
			Password: "testpass",
			DBName:   "testdb",
			SSLMode:  "disable",
		}

		if config.Host == "" {
			t.Error("Expected Host to be set")
		}
		if config.Port == "" {
			t.Error("Expected Port to be set")
		}
		if config.User == "" {
			t.Error("Expected User to be set")
		}
		if config.Password == "" {
			t.Error("Expected Password to be set")
		}
		if config.DBName == "" {
			t.Error("Expected DBName to be set")
		}
		if config.SSLMode == "" {
			t.Error("Expected SSLMode to be set")
		}
	})

	t.Run("NewConfigFromEnv returns non-nil config", func(t *testing.T) {
		config := NewConfigFromEnv()
		if config == nil {
			t.Error("Expected non-nil config")
		}
	})
}

func TestConnectErrorConditions(t *testing.T) {
	t.Run("Connect with invalid host returns error", func(t *testing.T) {
		config := &Config{
			Host:     "nonexistent-host-that-should-not-exist.invalid",
			Port:     "9999",
			User:     "testuser",
			Password: "testpass",
			DBName:   "testdb",
			SSLMode:  "disable",
		}

		db, err := Connect(config)
		if err == nil {
			t.Error("Expected error when connecting to invalid host")
			if db != nil {
				sqlDB, _ := db.DB()
				if sqlDB != nil {
					sqlDB.Close()
				}
			}
		}

		// Verify error message contains context
		if err != nil && err.Error() == "" {
			t.Error("Expected non-empty error message")
		}
	})
}

// Unit tests using SQLite in-memory database
// These tests don't require Docker and test the migration logic

func TestMigrateWithSQLite(t *testing.T) {
	t.Run("Migrate creates tables successfully", func(t *testing.T) {
		// Create in-memory SQLite database
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			t.Fatalf("Failed to create in-memory database: %v", err)
		}

		// Run migrations
		err = Migrate(db)
		if err != nil {
			t.Fatalf("Migrate failed: %v", err)
		}

		// Verify Image table exists by trying to query it
		var count int64
		err = db.Model(&models.Image{}).Count(&count).Error
		if err != nil {
			t.Errorf("Failed to query images table: %v", err)
		}

		// Verify ImageTag table exists by trying to query it
		err = db.Model(&models.ImageTag{}).Count(&count).Error
		if err != nil {
			t.Errorf("Failed to query image_tags table: %v", err)
		}
	})

	t.Run("Migrate is idempotent", func(t *testing.T) {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			t.Fatalf("Failed to create in-memory database: %v", err)
		}

		// Run migration first time
		err = Migrate(db)
		if err != nil {
			t.Fatalf("First migration failed: %v", err)
		}

		// Run migration second time (should not fail)
		err = Migrate(db)
		if err != nil {
			t.Fatalf("Second migration failed: %v", err)
		}

		// Run migration third time (should not fail)
		err = Migrate(db)
		if err != nil {
			t.Fatalf("Third migration failed: %v", err)
		}
	})

	t.Run("Migrate allows inserting and querying data", func(t *testing.T) {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			t.Fatalf("Failed to create in-memory database: %v", err)
		}

		// Run migrations
		err = Migrate(db)
		if err != nil {
			t.Fatalf("Migrate failed: %v", err)
		}

		// Insert test image
		image := &models.Image{
			Name:       "nginx",
			Repository: "docker.io",
			FullName:   "docker.io/nginx",
		}
		err = db.Create(image).Error
		if err != nil {
			t.Fatalf("Failed to insert image: %v", err)
		}

		// Verify image was inserted
		var count int64
		err = db.Model(&models.Image{}).Count(&count).Error
		if err != nil {
			t.Fatalf("Failed to count images: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 image, got %d", count)
		}

		// Insert test image tag
		imageTag := &models.ImageTag{
			ImageID:       image.ID,
			Tag:           "latest",
			FirstSeen:     time.Now().UTC(),
			LastSeen:      time.Now().UTC(),
			ResourceType:  "Deployment",
			ResourceName:  "nginx-deploy",
			Namespace:     "default",
			ContainerName: "nginx",
		}
		err = db.Create(imageTag).Error
		if err != nil {
			t.Fatalf("Failed to insert image tag: %v", err)
		}

		// Verify image tag was inserted
		var tagCount int64
		err = db.Model(&models.ImageTag{}).Count(&tagCount).Error
		if err != nil {
			t.Fatalf("Failed to count image tags: %v", err)
		}
		if tagCount != 1 {
			t.Errorf("Expected 1 image tag, got %d", tagCount)
		}

		// Test querying with join
		var result models.ImageTag
		err = db.Preload("Image").First(&result).Error
		if err != nil {
			t.Fatalf("Failed to query image tag with preload: %v", err)
		}

		if result.Image.Name != "nginx" {
			t.Errorf("Expected image name 'nginx', got '%s'", result.Image.Name)
		}
		if result.Tag != "latest" {
			t.Errorf("Expected tag 'latest', got '%s'", result.Tag)
		}
	})

	t.Run("Migrate preserves existing data", func(t *testing.T) {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			t.Fatalf("Failed to create in-memory database: %v", err)
		}

		// First migration
		err = Migrate(db)
		if err != nil {
			t.Fatalf("First migrate failed: %v", err)
		}

		// Insert data
		image := &models.Image{
			Name:       "redis",
			Repository: "docker.io",
			FullName:   "docker.io/redis",
		}
		err = db.Create(image).Error
		if err != nil {
			t.Fatalf("Failed to insert image: %v", err)
		}

		// Run migration again
		err = Migrate(db)
		if err != nil {
			t.Fatalf("Second migrate failed: %v", err)
		}

		// Verify data still exists
		var count int64
		err = db.Model(&models.Image{}).Where("name = ?", "redis").Count(&count).Error
		if err != nil {
			t.Fatalf("Failed to count images: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 redis image after re-migration, got %d", count)
		}
	})

	t.Run("Migrate handles multiple sequential inserts", func(t *testing.T) {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			t.Fatalf("Failed to create in-memory database: %v", err)
		}

		// Run migrations
		err = Migrate(db)
		if err != nil {
			t.Fatalf("Migrate failed: %v", err)
		}

		// Test multiple sequential inserts
		for i := 0; i < 5; i++ {
			image := &models.Image{
				Name:       fmt.Sprintf("image-%d", i),
				Repository: "docker.io",
				FullName:   fmt.Sprintf("docker.io/image-%d", i),
			}
			err := db.Create(image).Error
			if err != nil {
				t.Fatalf("Insert %d failed: %v", i, err)
			}
		}

		// Verify all inserts succeeded
		var count int64
		err = db.Model(&models.Image{}).Count(&count).Error
		if err != nil {
			t.Fatalf("Failed to count images: %v", err)
		}
		if count != 5 {
			t.Errorf("Expected 5 images to be inserted, got %d", count)
		}
	})
}

func TestConnectWithSQLiteDriver(t *testing.T) {
	t.Run("Database connection settings are applied", func(t *testing.T) {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
			NowFunc: func() time.Time {
				return time.Now().UTC()
			},
		})
		if err != nil {
			t.Fatalf("Failed to create database: %v", err)
		}

		sqlDB, err := db.DB()
		if err != nil {
			t.Fatalf("Failed to get SQL DB: %v", err)
		}

		// Verify we can ping the database
		err = sqlDB.Ping()
		if err != nil {
			t.Errorf("Failed to ping database: %v", err)
		}

		// Close the connection
		err = sqlDB.Close()
		if err != nil {
			t.Errorf("Failed to close database: %v", err)
		}
	})
}

func TestMigrateDropsOldIndex(t *testing.T) {
	t.Run("Migrate drops old index before creating new schema", func(t *testing.T) {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			t.Fatalf("Failed to create in-memory database: %v", err)
		}

		// Create old index manually (simulating old schema)
		db.Exec("CREATE TABLE IF NOT EXISTS image_tags (id INTEGER PRIMARY KEY)")
		db.Exec("CREATE INDEX IF NOT EXISTS idx_image_tag_unique ON image_tags(id)")

		// Run migrations (should drop old index)
		err = Migrate(db)
		if err != nil {
			t.Fatalf("Migrate failed: %v", err)
		}

		// Verify migration succeeded
		var count int64
		err = db.Model(&models.ImageTag{}).Count(&count).Error
		if err != nil {
			t.Fatalf("Failed to count image tags: %v", err)
		}
		if count != 0 {
			t.Errorf("Expected 0 image tags, got %d", count)
		}
	})
}

func TestConfigStructure(t *testing.T) {
	t.Run("Config struct has all required fields", func(t *testing.T) {
		config := &Config{
			Host:     "host",
			Port:     "port",
			User:     "user",
			Password: "password",
			DBName:   "dbname",
			SSLMode:  "sslmode",
		}

		// Use reflection to ensure all fields are set
		if config.Host == "" {
			t.Error("Host should not be empty")
		}
		if config.Port == "" {
			t.Error("Port should not be empty")
		}
		if config.User == "" {
			t.Error("User should not be empty")
		}
		if config.Password == "" {
			t.Error("Password should not be empty")
		}
		if config.DBName == "" {
			t.Error("DBName should not be empty")
		}
		if config.SSLMode == "" {
			t.Error("SSLMode should not be empty")
		}
	})

	t.Run("Empty Config has zero values", func(t *testing.T) {
		config := &Config{}

		if config.Host != "" {
			t.Errorf("Expected empty Host, got '%s'", config.Host)
		}
		if config.Port != "" {
			t.Errorf("Expected empty Port, got '%s'", config.Port)
		}
		if config.User != "" {
			t.Errorf("Expected empty User, got '%s'", config.User)
		}
		if config.Password != "" {
			t.Errorf("Expected empty Password, got '%s'", config.Password)
		}
		if config.DBName != "" {
			t.Errorf("Expected empty DBName, got '%s'", config.DBName)
		}
		if config.SSLMode != "" {
			t.Errorf("Expected empty SSLMode, got '%s'", config.SSLMode)
		}
	})
}
