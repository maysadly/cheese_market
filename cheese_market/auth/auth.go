package auth

import (
	"context"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
	"html/template"
	"net/http"
	"time"
)

var tpl = template.Must(template.ParseFiles("templates/login.html", "templates/register.html"))

var secretKey = []byte("mysecretkey") // Секретный ключ для подписи JWT

var collection *mongo.Collection // Коллекция пользователей

type User struct {
	Username string `bson:"username"`
	Password string `bson:"password"`
	Role     string `bson:"role"`
}

func init() {
	// Инициализация подключения к MongoDB
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017") // Указываем URI MongoDB
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		fmt.Println("Ошибка подключения к MongoDB:", err)
		return
	}

	// Проверка соединения
	err = client.Ping(context.TODO(), nil)
	if err != nil {
		fmt.Println("Не удалось подключиться к MongoDB:", err)
		return
	}

	// Инициализируем коллекцию
	collection = client.Database("cheeseMarket").Collection("users")
	fmt.Println("Подключение к MongoDB установлено!")
}

// RegisterHandler - обработчик для регистрации
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		r.ParseForm()
		username := r.FormValue("username")
		password := r.FormValue("password")

		// Проверяем, существует ли уже пользователь с таким логином
		var existingUser User
		err := collection.FindOne(context.TODO(), bson.M{"username": username}).Decode(&existingUser)
		if err == nil { // Если не ошибка, значит пользователь существует
			http.Error(w, "User with such login already exists", http.StatusBadRequest)
			return
		} else if err != mongo.ErrNoDocuments {
			// Если ошибка другая, обработать её
			http.Error(w, "Error checking user presence", http.StatusInternalServerError)
			return
		}

		// Хешируем пароль перед сохранением
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Hashing password error", http.StatusInternalServerError)
			return
		}

		// Определяем роль (первый пользователь будет админом)
		role := "user"
		count, _ := collection.CountDocuments(context.TODO(), bson.M{})
		if count == 0 {
			role = "admin"
		}

		// Создаем нового пользователя
		user := User{
			Username: username,
			Password: string(hashedPassword),
			Role:     role,
		}

		// Сохраняем пользователя в MongoDB
		_, err = collection.InsertOne(context.TODO(), user)
		if err != nil {
			http.Error(w, "Registration error", http.StatusInternalServerError)
			return
		}

		// Перенаправляем в зависимости от роли
		if role == "admin" {
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/user", http.StatusSeeOther)
		}
		return
	}
	tpl.ExecuteTemplate(w, "register.html", nil)
}

// LoginHandler - обработчик для входа
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		r.ParseForm()
		username := r.FormValue("username")
		password := r.FormValue("password")

		// Проверяем наличие пользователя в базе данных
		var user User
		err := collection.FindOne(context.TODO(), bson.M{"username": username}).Decode(&user)
		if err != nil {
			http.Error(w, "Неверный логин или пароль", http.StatusUnauthorized)
			return
		}

		// Сравниваем пароль с сохраненным хешом
		err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
		if err != nil {
			http.Error(w, "Неверный логин или пароль", http.StatusUnauthorized)
			return
		}

		// Создаём JWT токен
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"username": username,
			"role":     user.Role,
			"exp":      time.Now().Add(time.Hour * 1).Unix(), // Время жизни токена (1 час)
		})

		// Подписываем токен
		tokenString, err := token.SignedString(secretKey)
		if err != nil {
			http.Error(w, "Ошибка генерации токена", http.StatusInternalServerError)
			return
		}

		// Отправляем токен клиенту (например, в cookie или в теле ответа)
		http.SetCookie(w, &http.Cookie{
			Name:  "token",
			Value: tokenString,
			Path:  "/",
		})
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}
	tpl.ExecuteTemplate(w, "login.html", nil)
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	// Удаляем токен из cookies
	http.SetCookie(w, &http.Cookie{
		Name:   "token",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func DashboardHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем наличие и валидность токена
	tokenCookie, err := r.Cookie("token")
	if err != nil {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	tokenString := tokenCookie.Value
	claims := jwt.MapClaims{}

	// Проверяем и парсим токен
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return secretKey, nil
	})

	if err != nil || !token.Valid {
		http.Error(w, "Не авторизован", http.StatusUnauthorized)
		return
	}

	// Получаем имя пользователя из токена
	role := claims["role"].(string)

	// Перенаправление в зависимости от роли
	if role == "admin" {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/user", http.StatusSeeOther)
	}
}
