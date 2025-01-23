package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
    "github.com/stretchr/testify/assert"
)

var testCollection *mongo.Collection

func validateProduct(p Product) error {
	if p.Name == "" {
		return errors.New("product name cannot be empty")
	}
	if p.Price <= 0 {
		return errors.New("product price must be greater than zero")
	}
	if p.Category == "" {
		return errors.New("product category cannot be empty")
	}
	return nil
}

func TestValidateProduct(t *testing.T) {
	tests := []struct {
		name    string
		product Product
		wantErr bool
	}{
		{
			name: "Valid product",
			product: Product{
				Name:     "Mozzarella",
				Price:    4.50,
				Category: "Soft Cheese",
			},
			wantErr: false,
		},
		{
			name: "Empty name",
			product: Product{
				Name:     "",
				Price:    4.50,
				Category: "Soft Cheese",
			},
			wantErr: true,
		},
		{
			name: "Negative price",
			product: Product{
				Name:     "Brie",
				Price:    -2.00,
				Category: "Soft Cheese",
			},
			wantErr: true,
		},
		{
			name: "Empty category",
			product: Product{
				Name:     "Cheddar",
				Price:    5.00,
				Category: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProduct(tt.product)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateProduct() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}


func generateError(message string) error {
	return errors.New(message)
}
func TestErrors(t *testing.T) {
	err := generateError("Product not found")
	if err.Error() != "Product not found" {
		t.Errorf("generateError() = %v, want %v", err.Error(), "Product not found")
	}

	err = generateError("Invalid product ID")
	if err.Error() != "Invalid product ID" {
		t.Errorf("generateError() = %v, want %v", err.Error(), "Invalid product ID")
	}
}

var client *mongo.Client
var db *mongo.Database

func setup() {
	var err error
	client, err = mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatal(err)
	}
	db = client.Database("cheeseMarket")
	collection = db.Collection("test")

	_, err = collection.DeleteMany(context.Background(), bson.D{})
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
		insertResult, err := collection.InsertOne(context.Background(), item)
		assert.NoError(t, err)
		assert.NotNil(t, insertResult.InsertedID)

		var result bson.D
		err = collection.FindOne(context.Background(), bson.D{{"name", "item1"}}).Decode(&result)
		assert.NoError(t, err)
		assert.Equal(t, "item1", result.Map()["name"])
	})

	t.Run("GET /item", func(t *testing.T) {
		var result bson.D
		err := collection.FindOne(context.Background(), bson.D{{"name", "item1"}}).Decode(&result)
		assert.NoError(t, err)
		assert.Equal(t, "item1", result.Map()["name"])
	})

	t.Run("PUT /item", func(t *testing.T) {
		update := bson.D{
			{"$set", bson.D{{"price", 20.0}}},
		}
		_, err := collection.UpdateOne(context.Background(), bson.D{{"name", "item1"}}, update)
		assert.NoError(t, err)

		var result bson.D
		err = collection.FindOne(context.Background(), bson.D{{"name", "item1"}}).Decode(&result)
		assert.NoError(t, err)
		assert.Equal(t, 20.0, result.Map()["price"])
	})

	t.Run("DELETE /item", func(t *testing.T) {
		_, err := collection.DeleteOne(context.Background(), bson.D{{"name", "item1"}})
		assert.NoError(t, err)

		var result bson.D
		err = collection.FindOne(context.Background(), bson.D{{"name", "item1"}}).Decode(&result)
		assert.Error(t, err) 
	})
}

func TestSendEmail(t *testing.T) {
 	err := sendEmail("kamil.akbarov.95@gmail.com", "Test Email", "This is a test email.", []byte{}, "plain/text")

	assert.NoError(t, err, "Email should be sent successfully")
}

func TestEndToEnd(t *testing.T) {
	go func() {
		if err := http.ListenAndServe(":8081", nil); err != nil {
			t.Fatalf("could not start server: %v", err)
		}
	}()
	time.Sleep(2 * time.Second)

	resp, err := http.Get("http://localhost:8080/products")
	if err != nil {
		t.Fatalf("could not send GET request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK; got %v", resp.Status)
	}
}
