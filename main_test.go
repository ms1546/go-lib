package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/gin-gonic/gin"
	openai "github.com/sashabaranov/go-openai"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestGreetUser(t *testing.T) {
	db, _ = gorm.Open(postgres.New(postgres.Config{
		DSN:                  "user=postgres password=postgres dbname=postgres port=5432 sslmode=disable TimeZone=Asia/Tokyo",
		DriverName:           "postgres",
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})

	db.AutoMigrate(&User{})
	db.Create(&User{Name: "John Doe", Email: "john.doe@example.com"})

	gin.SetMode(gin.TestMode)
	router := gin.Default()

	router.GET("/greet/:id", func(c *gin.Context) {
		var user User
		id := c.Param("id")

		if err := db.First(&user, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "User not found"})
			return
		}

		client := openai.NewClient("your-openai-api-key")
		ctx := context.Background()

		prompt := fmt.Sprintf("Generate a greeting for user: %s", user.Name)
		req := openai.CompletionRequest{
			Model:     "text-davinci-003",
			Prompt:    prompt,
			MaxTokens: 50,
		}

		resp, err := client.CreateCompletion(ctx, req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error generating greeting"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"greeting": resp.Choices[0].Text})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/greet/1", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status code %d but got %d", http.StatusOK, w.Code)
	}

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Unable to parse response body: %v", err)
	}

	if _, exists := response["greeting"]; !exists {
		t.Fatalf("Expected greeting in response but got none")
	}
}

func TestMain(m *testing.M) {
	secretName := ""
	region := "ap-northeast-1"

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	svc := secretsmanager.NewFromConfig(cfg)

	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	}

	result, err := svc.GetSecretValue(context.TODO(), input)
	if err != nil {
		log.Fatalf("unable to retrieve secret, %v", err)
	}

	var dbConfig DBConfig
	if err := json.Unmarshal([]byte(*result.SecretString), &dbConfig); err != nil {
		log.Fatalf("unable to unmarshal secret, %v", err)
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Tokyo",
		dbConfig.Host, dbConfig.User, dbConfig.Password, dbConfig.DBName, dbConfig.Port)

	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database, %v", err)
	}

	db.AutoMigrate(&User{})

	m.Run()
}
