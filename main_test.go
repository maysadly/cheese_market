package main

import (
	"context"
	"os/exec"
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

func TestSendEmail(t *testing.T) {
	err := sendEmail("kamil.akbarov.95@gmail.com", "Test Email", "This is a test email.", []byte{}, "plain/text")
   assert.NoError(t, err, "Email should be sent successfully")
}


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
func startSeleniumServer() (*exec.Cmd, error) {
	cmd := exec.Command("java", "-jar", "selenium-server-4.28.0.jar", "standalone")
	err := cmd.Start() 
	if err != nil {
		return nil, fmt.Errorf("failed to start Selenium Server: %v", err)
	}

	time.Sleep(5 * time.Second)
	return cmd, nil
}
func TestRegisterForm(t *testing.T) {
	username := generateUniqueUsername()
	seleniumCmd, err := startSeleniumServer()
	if err != nil {
		t.Fatalf("Could not start Selenium Server: %v", err)
	}
	defer seleniumCmd.Process.Kill() 

	caps := selenium.Capabilities{
		"browserName": "chrome",
	}

	driver, err := selenium.NewRemote(caps, "http://localhost:4444/wd/hub")
	if err != nil {
		t.Fatalf("Failed to open session: %v", err)
	}
	defer driver.Quit()

	// Переход на страницу регистрации
	if err := driver.Get("http://localhost:8080/register"); err != nil {
		t.Fatalf("Failed to load page: %v", err)
	}

	// Функция ожидания элемента
	waitForElement := func(selector string) selenium.WebElement {
		var elem selenium.WebElement
		for i := 0; i < 10; i++ { // 10 попыток с интервалом 500 мс
			elem, err = driver.FindElement(selenium.ByCSSSelector, selector)
			if err == nil {
				return elem
			}
			time.Sleep(500 * time.Millisecond)
		}
		t.Fatalf("Element not found: %s", selector)
		return nil
	}

	// Ожидание и ввод данных
	emailField := waitForElement("#email")
	usernameField := waitForElement("#username")
	passwordField := waitForElement("#password")

	if err := emailField.SendKeys("test@gmail.com"); err != nil {
		t.Fatalf("Failed to input email: %v", err)
	}
	if err := usernameField.SendKeys(username); err != nil {
		t.Fatalf("Failed to input username: %v", err)
	}
	if err := passwordField.SendKeys("TestPassword123"); err != nil {
		t.Fatalf("Failed to input password: %v", err)
	}

	// Ожидание и нажатие кнопки отправки формы
	submitButton := waitForElement(".main__form-submit")
	if err := submitButton.Click(); err != nil {
		t.Fatalf("Failed to click submit button: %v", err)
	}

	// Ожидание редиректа
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(30 * time.Second)

	for {
		select {
		case <-timeout:
			pageSource, _ := driver.PageSource()
			t.Fatalf("Timed out waiting for page redirect. Page source:\n%s", pageSource)
		case <-ticker.C:
			currentURL, err := driver.CurrentURL()
			if err != nil {
				t.Fatalf("Failed to get current URL: %v", err)
			}

			fmt.Println("Current URL:", currentURL)

			if currentURL == "http://localhost:8080/verify" {
				fmt.Println("Successfully redirected to verification page!")
				return
			}
		}
	}
}