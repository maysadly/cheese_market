package auth

import (
    "context"
    "github.com/dgrijalva/jwt-go"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
    "golang.org/x/crypto/bcrypt"
    "golang.org/x/time/rate"
    "html/template"
    "log"
    "net/http"
    "os"
    "time"
)

var tpl = template.Must(template.ParseFiles("templates/login.html", "templates/register.html"))
var secretKey = []byte("mysecretkey")
var collection *mongo.Collection
var limiter = rate.NewLimiter(1, 3) // Rate limit of 1 request per second with a burst of 3 requests

type User struct {
    Email    string `bson:"email"`
    Username string `bson:"username"`
    Password string `bson:"password"`
    Role     string `bson:"role"`
}

func init() {
    // Set up logging to a file
    file, err := os.OpenFile("server.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
    if err != nil {
        log.Fatalf("Failed to open log file: %v", err)
    }
    log.SetOutput(file)
    log.Println("Logging to file server.log")

    clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
    client, err := mongo.Connect(context.TODO(), clientOptions)
    if err != nil {
        log.Fatalf("Failed to connect to MongoDB: %v", err)
    }

    err = client.Ping(context.TODO(), nil)
    if err != nil {
        log.Fatalf("MongoDB connection failed: %v", err)
    }

    collection = client.Database("cheeseMarket").Collection("users")
    log.Println("Connected to MongoDB!")
}

func rateLimitMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !limiter.Allow() {
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }
        next.ServeHTTP(w, r)
    })
}

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodPost {
        r.ParseForm()
        email := r.FormValue("email")
        username := r.FormValue("username")
        password := r.FormValue("password")
        
        log.Printf("Attempt to register user: %s", username)

        var existingUser User
        err := collection.FindOne(context.TODO(), bson.M{"username": username}).Decode(&existingUser)
        if err == nil {
            http.Error(w, "User with such login already exists", http.StatusBadRequest)
            log.Printf("Registration failed: User %s already exists", username)
            return
        } else if err != mongo.ErrNoDocuments {
            http.Error(w, "Error checking user presence", http.StatusInternalServerError)
            log.Printf("Error checking user presence for %s: %v", username, err)
            return
        }

        hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
        if err != nil {
            http.Error(w, "Hashing password error", http.StatusInternalServerError)
            log.Printf("Hashing password error for %s: %v", username, err)
            return
        }

        role := "user"
        count, _ := collection.CountDocuments(context.TODO(), bson.M{})
        if count == 0 {
            role = "admin"
        }

        user := User{
            Email:    email,
            Username: username,
            Password: string(hashedPassword),
            Role:     role,
        }

        _, err = collection.InsertOne(context.TODO(), user)
        if err != nil {
            http.Error(w, "Registration error", http.StatusInternalServerError)
            log.Printf("Registration error for user %s: %v", username, err)
            return
        }

        if role == "admin" {
            http.Redirect(w, r, "/admin", http.StatusSeeOther)
        } else {
            http.Redirect(w, r, "/user", http.StatusSeeOther)
        }
        log.Printf("User %s registered successfully with role %s", username, role)
        return
    }
    tpl.ExecuteTemplate(w, "register.html", nil)
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodPost {
        r.ParseForm()
        username := r.FormValue("username")
        password := r.FormValue("password")
        
        log.Printf("Attempt to login user: %s", username)

        var user User
        err := collection.FindOne(context.TODO(), bson.M{"username": username}).Decode(&user)
        if err != nil {
            http.Error(w, "Incorrect login or password", http.StatusUnauthorized)
            log.Printf("Login failed for user %s: incorrect login or password", username)
            return
        }

        err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
        if err != nil {
            http.Error(w, "Incorrect login or password", http.StatusUnauthorized)
            log.Printf("Login failed for user %s: incorrect login or password", username)
            return
        }

        token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
            "username": username,
            "role":     user.Role,
            "exp":      time.Now().Add(time.Hour * 1).Unix(),
        })

        tokenString, err := token.SignedString(secretKey)
        if err != nil {
            http.Error(w, "Generation token error", http.StatusInternalServerError)
            log.Printf("Token generation error for user %s: %v", username, err)
            return
        }

        http.SetCookie(w, &http.Cookie{
            Name:  "token",
            Value: tokenString,
            Path:  "/",
        })
        http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
        log.Printf("User %s logged in successfully", username)
        return
    }
    tpl.ExecuteTemplate(w, "login.html", nil)
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
    http.SetCookie(w, &http.Cookie{
        Name:   "token",
        Value:  "",
        Path:   "/",
        MaxAge: -1,
    })
    http.Redirect(w, r, "/login", http.StatusSeeOther)
    log.Println("User logged out successfully")
}

func DashboardHandler(w http.ResponseWriter, r *http.Request) {
    tokenCookie, err := r.Cookie("token")
    if err != nil {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        log.Println("Unauthorized access attempt to dashboard")
        return
    }

    tokenString := tokenCookie.Value
    claims := jwt.MapClaims{}

    token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
        return secretKey, nil
    })

    if err != nil || !token.Valid {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        log.Println("Unauthorized access attempt to dashboard with invalid token")
        return
    }

    role := claims["role"].(string)

    if role == "admin" {
        http.Redirect(w, r, "/admin", http.StatusSeeOther)
        log.Println("Admin user redirected to admin dashboard")
    } else {
        http.Redirect(w, r, "/user", http.StatusSeeOther)
        log.Println("Regular user redirected to user dashboard")
    }
}
