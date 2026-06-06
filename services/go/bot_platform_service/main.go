package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

var db *sql.DB

func initDB() error {
	var err error
	connStr := "host=localhost port=5432 user=tigerswap password=securepass dbname=tigerswap sslmode=disable"
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		return err
	}
	return db.Ping()
}

func main() {
	r := gin.Default()
	
	if err := initDB(); err != nil {
		fmt.Println("Database connection failed:", err)
	}
	
	fmt.Println("Bot platform service running on :8085")
	r.Run(":8085")
}