package main

import ( // Импортируем пакет auth
	"cheese_market/auth"
	"context"
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"net/http"
	"strconv"
)

var collection *mongo.Collection

// Product структура продукта
type Product struct {
	ID    string  `json:"id" bson:"_id,omitempty"`
	Name  string  `json:"name" bson:"name"`
	Price float64 `json:"price" bson:"price"`
}

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

	db := client.Database("cheeseMarket")
	collection = db.Collection("products")
}

func serveHTML(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/admin.html")
}
func serveUser(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/user.html")
}
func serveLogin(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/login.html")
}
func serveRegistration(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/register.html")
}

func handleProducts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		sortBy := r.URL.Query().Get("sortBy")
		order := r.URL.Query().Get("order")
		page := r.URL.Query().Get("page")
		pageSize := r.URL.Query().Get("pageSize")

		if id != "" {
			objectID, err := primitive.ObjectIDFromHex(id)
			if err != nil {
				http.Error(w, "Invalid ID format", http.StatusBadRequest)
				return
			}

			var product Product
			filter := bson.M{"_id": objectID}
			err = collection.FindOne(context.TODO(), filter).Decode(&product)
			if err == mongo.ErrNoDocuments {
				http.Error(w, "Product not found", http.StatusNotFound)
				return
			} else if err != nil {
				http.Error(w, "Failed to fetch product", http.StatusInternalServerError)
				return
			}

			json.NewEncoder(w).Encode(product)
			return
		}

		// Default filter for fetching all products
		filter := bson.M{}
		findOptions := options.Find()

		// Sorting
		if sortBy != "" {
			sortOrder := 1
			if order == "desc" {
				sortOrder = -1
			}
			findOptions.SetSort(bson.D{{Key: sortBy, Value: sortOrder}})
		}

		// Pagination
		p, _ := strconv.Atoi(page)
		ps, _ := strconv.Atoi(pageSize)
		if p < 1 {
			p = 1
		}
		if ps < 1 {
			ps = 10 // Default page size
		}
		skip := (p - 1) * ps
		findOptions.SetSkip(int64(skip))
		findOptions.SetLimit(int64(ps))

		// Fetch products
		var cursor, err = collection.Find(context.TODO(), filter, findOptions)
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

		if err := cursor.Err(); err != nil {
			http.Error(w, "Error during cursor iteration", http.StatusInternalServerError)
			return
		}

		// Get total count of products for pagination metadata
		totalCount, err := collection.CountDocuments(context.TODO(), filter)
		if err != nil {
			http.Error(w, "Failed to fetch total count", http.StatusInternalServerError)
			return
		}

		// Response with products and pagination metadata
		response := map[string]interface{}{
			"products":   products,
			"total":      totalCount,
			"page":       p,
			"pageSize":   ps,
			"totalPages": (totalCount + int64(ps) - 1) / int64(ps), // Calculate total pages
		}

		json.NewEncoder(w).Encode(response)

	case http.MethodPost:
		var product Product
		err := json.NewDecoder(r.Body).Decode(&product)
		if err != nil {
			http.Error(w, "Failed to decode request body", http.StatusBadRequest)
			return
		}
		_, err = collection.InsertOne(context.TODO(), product)
		if err != nil {
			http.Error(w, "Failed to insert data", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"message": "Product added successfully!"})

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

		json.NewEncoder(w).Encode(map[string]string{"message": "Product updated successfully!"})

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

	// Обработка статики
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Обработка маршрутов для продуктов
	http.HandleFunc("/products", handleProducts)
	http.HandleFunc("/", serveHTML)                      // admin.html page
	http.HandleFunc("/user", serveUser)                  //user.html page
	http.HandleFunc("/login", auth.LoginHandler)         // login page
	http.HandleFunc("/register", auth.RegisterHandler)   // registration page
	http.HandleFunc("/logout", auth.LogoutHandler)       // logout
	http.HandleFunc("/dashboard", auth.DashboardHandler) // after login

	// Старт сервера
	fmt.Println("Server is running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
