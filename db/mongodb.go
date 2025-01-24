package db

import (
	"context"
	"log"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var Collection *mongo.Collection

func ConnectMongoDB() {
	clientOptions := options.Client().ApplyURI("mongodb://mongodb:27017")
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	err = client.Ping(context.TODO(), nil)
	if err != nil {
		log.Fatalf("MongoDB connection failed: %v", err)
	}

	log.Println("Connected to MongoDB!")
	db := client.Database("cheeseMarket")
	Collection = db.Collection("users")
}
