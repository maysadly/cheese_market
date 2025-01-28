package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tebeka/selenium"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
func generateUniqueUsername() string {
	rand.Seed(time.Now().UnixNano()) 
	uniqueSuffix := rand.Intn(1000) 
	return "user" + strconv.Itoa(int(time.Now().Unix())) + strconv.Itoa(uniqueSuffix)
}
func TestRegisterForm(t *testing.T) {
	username := generateUniqueUsername()

	caps := selenium.Capabilities{
		"browserName": "chrome",
	}

	driver, err := selenium.NewRemote(caps, "http://localhost:4444/wd/hub")
	if err != nil {
		log.Fatalf("Failed to open session: %v", err)
	}
	defer driver.Quit()

	if err := driver.Get("http://localhost:8080/register"); err != nil {
		t.Fatalf("Failed to load page: %v", err)
	}

	emailField, err := driver.FindElement(selenium.ByCSSSelector, "#email")
	if err != nil {
		t.Fatalf("Failed to find email field: %v", err)
	}

	usernameField, err := driver.FindElement(selenium.ByCSSSelector, "#username")
	if err != nil {
		t.Fatalf("Failed to find username field: %v", err)
	}

	passwordField, err := driver.FindElement(selenium.ByCSSSelector, "#password")
	if err != nil {
		t.Fatalf("Failed to find password field: %v", err)
	}

	err = emailField.SendKeys("test@gmail.com")
	if err != nil {
		t.Fatalf("Failed to input email: %v", err)
	}

	err = usernameField.SendKeys(username)
	if err != nil {
		t.Fatalf("Failed to input username: %v", err)
	}

	err = passwordField.SendKeys("TestPassword123")
	if err != nil {
		t.Fatalf("Failed to input password: %v", err)
	}

	submitButton, err := driver.FindElement(selenium.ByCSSSelector, ".main__form-submit")
	if err != nil {
		t.Fatalf("Failed to find submit button: %v", err)
	}

	err = submitButton.Click()
	if err != nil {
		t.Fatalf("Failed to click submit button: %v", err)
	}

	timeout := time.After(30 * time.Second)
	tick := time.Tick(500 * time.Millisecond)

	var currentURL string
	for {
		select {
		case <-timeout:
			t.Fatalf("Timed out waiting for page redirect to login")
		case <-tick:
			currentURL, err = driver.CurrentURL()
			if err != nil {
				t.Fatalf("Failed to get current URL: %v", err)
			}

			fmt.Println("Current URL:", currentURL)

			if currentURL == "http://localhost:8080/verify" {
				fmt.Println("Successfully redirected to login page!")
				return
			}
		}
	}
}