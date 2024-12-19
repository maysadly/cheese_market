package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	_ "time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Product struct {
	ID    string  `json:"id" bson:"_id,omitempty"`
	Name  string  `json:"name" bson:"name"`
	Price float64 `json:"price" bson:"price"`
}

var collection *mongo.Collection

func connectMongoDB() {
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")

	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	err = client.Ping(context.TODO(), nil)
	if err != nil {
		log.Fatalf("MongoDB connection failed: %v", err)
	}

	fmt.Println("Connected to MongoDB!")
	collection = client.Database("cheeseMarket").Collection("products")
}

func serveHTML(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "index.html")
}

func handleProducts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		cursor, err := collection.Find(context.TODO(), bson.M{})
		if err != nil {
			http.Error(w, "Failed to fetch data", http.StatusInternalServerError)
			return
		}
		defer cursor.Close(context.TODO())

		var products []Product
		for cursor.Next(context.TODO()) {
			var product Product
			if err := cursor.Decode(&product); err != nil {
				http.Error(w, "Failed to decode data", http.StatusInternalServerError)
				return
			}
			products = append(products, product)
		}
		json.NewEncoder(w).Encode(products)

	case http.MethodPost:
		var product Product
		err := json.NewDecoder(r.Body).Decode(&product)
		if err != nil {
			return
		}
		_, err = collection.InsertOne(context.TODO(), product)
		if err != nil {
			http.Error(w, "Failed to insert data", http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(w).Encode(map[string]string{"message": "Product added successfully!"})
		if err != nil {
			return
		}

	case http.MethodPut:
		var payload struct {
			ID    string  `json:"id"`
			Name  string  `json:"name"`
			Price float64 `json:"price"`
		}

		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Failed to decode request body", http.StatusBadRequest)
			return
		}

		objectID, err := primitive.ObjectIDFromHex(payload.ID)
		if err != nil {
			http.Error(w, "Invalid ID format", http.StatusBadRequest)
			return
		}

		filter := bson.M{"_id": objectID}
		update := bson.M{
			"$set": bson.M{
				"name":  payload.Name,
				"price": payload.Price,
			},
		}

		result, err := collection.UpdateOne(context.TODO(), filter, update)
		if err != nil {
			http.Error(w, "Failed to update product", http.StatusInternalServerError)
			return
		}

		if result.MatchedCount == 0 {
			http.Error(w, "Product not found", http.StatusNotFound)
			return
		}

		err = json.NewEncoder(w).Encode(map[string]string{"message": "Product updated successfully!"})
		if err != nil {
			http.Error(w, "Failed to send response", http.StatusInternalServerError)
			return
		}

	case http.MethodDelete:
		var payload struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Failed to decode request body", http.StatusBadRequest)
			return
		}

		objectID, err := primitive.ObjectIDFromHex(payload.ID)
		if err != nil {
			http.Error(w, "Invalid ID format", http.StatusBadRequest)
			return
		}

		filter := bson.M{"_id": objectID}
		_, err = collection.DeleteOne(context.TODO(), filter)
		if err != nil {
			http.Error(w, "Failed to delete product", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"message": "Product deleted successfully!"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func main() {
	connectMongoDB()

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/products", handleProducts)
	http.HandleFunc("/", serveHTML)
	fmt.Println("Server is running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
