package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	_ "time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Product struct {
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
		defer func(cursor *mongo.Cursor, ctx context.Context) {
			err := cursor.Close(ctx)
			if err != nil {
				return
			}
		}(cursor, context.TODO())

		var products []Product
		for cursor.Next(context.TODO()) {
			var product Product
			if err := cursor.Decode(&product); err != nil {
				http.Error(w, "Failed to decode data", http.StatusInternalServerError)
				return
			}
			products = append(products, product)
		}
		err = json.NewEncoder(w).Encode(products)
		if err != nil {
			return
		}

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
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func main() {
	connectMongoDB()

	http.HandleFunc("/products", handleProducts)
	http.HandleFunc("/", serveHTML)
	fmt.Println("Server is running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
