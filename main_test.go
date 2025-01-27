package main

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/tebeka/selenium"
)

const (
	seleniumPath     = "selenium/selenium-server-4.28.0.jar"          // Укажите путь к Selenium Server
	chromeDriverPath = "selenium/chromedriver-win64/chromedriver.exe" // Укажите путь к chromedriver
	seleniumPort     = 4444                                           // Порт для Selenium WebDriver
)

var (
	client          *mongo.Client
	db              *mongo.Database
	test_collection *mongo.Collection
)

func setup() {
	var err error
	client, err = mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatal(err)
	}
	db = client.Database("cheeseMarket")
	test_collection = db.Collection("test")

	_, err = test_collection.DeleteMany(context.Background(), bson.D{})
	if err != nil {
		log.Fatal(err)
	}
}

func teardown() {
	if err := client.Disconnect(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func TestCRUDOperations(t *testing.T) {
	setup()
	defer teardown()

	t.Run("POST /item", func(t *testing.T) {
		item := bson.D{
			{"name", "item1"},
			{"price", 10.0},
		}
		insertResult, err := test_collection.InsertOne(context.Background(), item)
		assert.NoError(t, err)
		assert.NotNil(t, insertResult.InsertedID)

		var result bson.D
		err = test_collection.FindOne(context.Background(), bson.D{{"name", "item1"}}).Decode(&result)
		assert.NoError(t, err)
		assert.Equal(t, "item1", result.Map()["name"])
	})

	t.Run("GET /item", func(t *testing.T) {
		var result bson.D
		err := test_collection.FindOne(context.Background(), bson.D{{"name", "item1"}}).Decode(&result)
		assert.NoError(t, err)
		assert.Equal(t, "item1", result.Map()["name"])
	})

	t.Run("PUT /item", func(t *testing.T) {
		update := bson.D{
			{"$set", bson.D{{"price", 20.0}}},
		}
		_, err := test_collection.UpdateOne(context.Background(), bson.D{{"name", "item1"}}, update)
		assert.NoError(t, err)

		var result bson.D
		err = test_collection.FindOne(context.Background(), bson.D{{"name", "item1"}}).Decode(&result)
		assert.NoError(t, err)
		assert.Equal(t, 20.0, result.Map()["price"])
	})

	t.Run("DELETE /item", func(t *testing.T) {
		_, err := test_collection.DeleteOne(context.Background(), bson.D{{"name", "item1"}})
		assert.NoError(t, err)

		var result bson.D
		err = test_collection.FindOne(context.Background(), bson.D{{"name", "item1"}}).Decode(&result)
		assert.Error(t, err)
	})
}

func TestEndToEnd(t *testing.T) {
	geckoDriverPath := "selenium/geckodriver.exe" // Укажите путь к GeckoDriver

	// Запуск службы GeckoDriver
	fmt.Println("Attempting to start GeckoDriver service...")
	service, err := selenium.NewGeckoDriverService(geckoDriverPath, 4444, nil)
	if err != nil {
		t.Fatalf("Failed to start GeckoDriver service: %v", err)
	} else {
		fmt.Println("GeckoDriver service started successfully")
	}

	fmt.Println("GeckoDriver service started successfully")

	// Настройки для запуска Firefox в headless-режиме
	caps := selenium.Capabilities{
		"browserName": "firefox",
		"moz:firefoxOptions": map[string]interface{}{
			"args": []string{
				"--headless",   // Безголовый режим
				"--width=1920", // Размер окна
				"--height=1080",
			},
		},
	}

	// Подключение к Selenium WebDriver
	driver, err := selenium.NewRemote(caps, "http://localhost:4444/wd/hub")
	if err != nil {
		t.Fatalf("Error connecting to WebDriver: %v", err)
	}
	defer driver.Quit()

	// Запуск тестового HTTP-сервера
	go func() {
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()
	time.Sleep(2 * time.Second) // Даем серверу время на запуск

	// Открытие страницы
	err = driver.Get("http://localhost:8080/products")
	if err != nil {
		t.Fatalf("Error navigating to the page: %v", err)
	}

	// Проверяем заголовок страницы
	header, err := driver.FindElement(selenium.ByTagName, "h1")
	if err != nil {
		t.Fatalf("Error finding header element: %v", err)
	}
	text, err := header.Text()
	if err != nil || text != "Products" {
		t.Errorf("Expected header to be 'Products', got: %v", text)
	}
}
