package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/gin-gonic/gin"
	openai "github.com/sashabaranov/go-openai"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type User struct {
	ID    uint `gorm:"primaryKey"`
	Name  string
	Email string
}

type DBConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"username"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
}

var db *gorm.DB

func main() {
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

	r := gin.Default()

	r.GET("/greet/:id", func(c *gin.Context) {
		var user User
		id := c.Param("id")

		if err := db.First(&user, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "User not found"})
			return
		}

		apiKey := ""
		client := openai.NewClient(apiKey)
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

	r.Run(":8080")
}
