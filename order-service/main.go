package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/streadway/amqp"
)

// Order represents the structure of an order
type Order struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

type User struct {
	ID string `json:"id"`
	// other user properties
}

type Product struct {
	ID string `json:"id"`
	// other product properties
}

// DB stores the database connection pool
var DB *sql.DB

func main() {
	initLogFile()

	// Connect to the PostgreSQL database
	err := connectDB()
	if err != nil {
		log.Fatal("Failed to connect to the database:", err)
	}
	defer DB.Close()

	// Create the Gin router
	router := gin.Default()

	// Routes
	router.GET("/orders", getOrdersHandler)
	router.GET("/orders/:id", getOrderByIdHandler)
	router.POST("/orders", createOrderHandler)
	router.PUT("/orders/:id", updateOrderHandler)
	router.DELETE("/orders/:id", deleteOrderHandler)

	// Run the server
	err = router.Run(":8087")
	if err != nil {
		log.Fatal("Failed to start the server:", err)
	}
}

func initLogFile() {
	file, err := os.Create("logfile.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	log.SetOutput(file)
}

// connectDB establishes a connection to the PostgreSQL database
func connectDB() error {
	pgURI := os.Getenv("POSTGRES_URI")
	db, err := sql.Open("postgres", pgURI)
	if err != nil {
		return err
	}

	err = db.Ping()
	if err != nil {
		return err
	}

	fmt.Println("Connected to the database!")

	DB = db

	// Initialize the orders table if it does not exist
	err = initializeTable()
	if err != nil {
		return err
	}

	return nil
}

// initializeTable creates the orders table if it does not exist
func initializeTable() error {
	_, err := DB.Exec(`
		CREATE TABLE IF NOT EXISTS orders (
			id        SERIAL PRIMARY KEY,
			user_id   VARCHAR(255),
			product_id VARCHAR(255),
			quantity INTEGER
		)
	`)

	//update table, add one more colunm
	// _, err := DB.Exec(`
	// 	ALTER TABLE orders
	// 	ADD COLUMN IF NOT EXISTS quantity INTEGER
	// `)
	if err != nil {
		return err
	}

	return nil
}

// getOrdersHandler retrieves all orders from the database
func getOrdersHandler(c *gin.Context) {
	rows, err := DB.Query("SELECT id, user_id, product_id, quantity FROM orders")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to execute the query"})
		return
	}
	defer rows.Close()

	orders := []Order{}
	for rows.Next() {
		order := Order{}
		err := rows.Scan(&order.ID, &order.UserID, &order.ProductID, &order.Quantity)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan row"})
			return
		}
		orders = append(orders, order)
	}

	c.JSON(http.StatusOK, gin.H{"orders": orders})
}

// getOrderByIdHandler retrieves a specific order by its ID
func getOrderByIdHandler(c *gin.Context) {
	orderID := c.Param("id")

	row := DB.QueryRow("SELECT id, user_id, product_id, quantity FROM orders WHERE id = $1", orderID)
	order := Order{}
	err := row.Scan(&order.ID, &order.UserID, &order.ProductID, &order.Quantity)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to execute the query"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"order": order})
}

// createOrderHandler creates a new order in the database
func createOrderHandler(c *gin.Context) {
	var order Order
	if err := c.ShouldBindJSON(&order); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order data"})
		return
	}

	// Check if the user exists
	userExists, err := checkUserExists(order.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check user existence"})
		return
	}
	if !userExists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User does not exist"})
		return
	}

	// Check if the product exists
	productExists, err := checkProductExists(order.ProductID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check product existence"})
		return
	}
	if !productExists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product does not exist"})
		return
	}

	// Insert the order into the database
	_, insertErr := DB.Exec("INSERT INTO orders (id, user_id, product_id, quantity) VALUES ($1, $2, $3, $4)",
		order.ID, order.UserID, order.ProductID, order.Quantity)

	if insertErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to insert order"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"order": order})

	publishMsg()
}

// updateOrderHandler updates an existing order in the database
func updateOrderHandler(c *gin.Context) {
	orderID := c.Param("id")

	var order Order
	if err := c.ShouldBindJSON(&order); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order data"})
		return
	}

	// Update the order in the database
	_, err := DB.Exec("UPDATE orders SET user_id = $1, product_id = $2, quantity = $3 WHERE id = $4",
		order.UserID, order.ProductID, order.Quantity, orderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"order": order})
}

// deleteOrderHandler deletes an existing order from the database
func deleteOrderHandler(c *gin.Context) {
	orderID := c.Param("id")

	// Delete the order from the database
	_, err := DB.Exec("DELETE FROM orders WHERE id = $1", orderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete order"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Order deleted successfully"})
}

func checkUserExists(userID string) (bool, error) {
	// Make a request to the user web service to check if the user exists
	userURL := fmt.Sprintf("http://%s:%s/users/%s", os.Getenv("USER_SERVICE_HOST"), os.Getenv("USER_SERVICE_PORT"), userID)

	resp, err := http.Get(userURL)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}

	return false, nil
}

func checkProductExists(productID string) (bool, error) {
	// Make a request to the product web service to check if the product exists
	productURL := fmt.Sprintf("http://%s:%s/products/%s", os.Getenv("PRODUCT_SERVICE_HOST"), os.Getenv("PRODUCT_SERVICE_PORT"), productID)

	resp, err := http.Get(productURL)
	fmt.Print(err)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}

	return false, nil
}

func publishMsg() {

	// Connect to RabbitMQ
	conn, err := amqp.Dial(fmt.Sprintf("amqp://guest:guest@%s:%s/", os.Getenv("RABBITMQ_HOST"), os.Getenv("RABBITMQ_PORT")))
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	// Create a channel
	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel: %v", err)
	}
	defer ch.Close()

	// Declare the exchange
	err = ch.ExchangeDeclare(
		"order_exchange", // exchange name
		"fanout",         // exchange type
		true,             // durable
		false,            // auto-deleted
		false,            // internal
		false,            // no-wait
		nil,              // arguments
	)
	if err != nil {
		log.Fatalf("Failed to declare the exchange: %v", err)
	}

	// Define the message
	message := "New order created!"

	// Publish the message to the exchange
	err = ch.Publish(
		"order_exchange", // exchange name
		"",               // routing key
		false,            // mandatory
		false,            // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(message),
		},
	)
	if err != nil {
		log.Fatalf("Failed to publish a message: %v", err)
	}

	log.Println("Message sent to RabbitMQ")
	fmt.Println("Message sent to RabbitMQ")
}
