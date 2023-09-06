package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/apex/gateway"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

func setupRouter() *gin.Engine {

	router := gin.Default()

	router.GET("/:hash", shortUrlRedirect)

	return router
}

func isValidHash(hash string) bool {
	if len(hash) != 7 {
		return false
	}

	for _, r := range hash {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}

	return true
}

func returnInitUrl(hash string, c *gin.Context) string {

	defer returnError(c)

	if !isValidHash(hash) {
		panic("404 not found")
	}

	db, err := sql.Open("mysql", os.Getenv("CONNECTION_STRING")) 
	if err != nil {
		panic(err)
	}
	defer db.Close()

	var originalURL string
	var url_id int
	rows := db.QueryRow("SELECT id, url FROM urls WHERE `hash`=? LIMIT 1", hash)
	err = rows.Scan(&url_id, &originalURL)
	if err != nil {
		panic(err)
	}

	var visit int
	row := db.QueryRow("SELECT visits FROM statistics WHERE `url_id`=?", url_id)
	err = row.Scan(&visit)
	if err != nil {
		_, err = db.Exec("INSERT INTO statistics (url_id, visits) VALUES (?, ?)", url_id, 1)
		if err != nil {
			panic(err)
		}
	} else {
		_, err = db.Exec("UPDATE statistics SET visits = visits + 1 WHERE `url_id`=?", url_id)
		if err != nil {
			panic(err)
		}
	}

	return originalURL
}

func shortUrlRedirect(c *gin.Context) {
	hash := c.Param("hash")
	URL := returnInitUrl(hash, c)
	c.Redirect(302, URL)
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	
	if inLambda() {
		fmt.Println("running aws lambda in aws")
		log.Fatal(gateway.ListenAndServe(":8080", setupRouter()))
	} else {
		fmt.Println("running aws lambda in local")
		log.Fatal(http.ListenAndServe(":8080", setupRouter()))
	}
}

func inLambda() bool {
	if lambdaTaskRoot := os.Getenv("LAMBDA_TASK_ROOT"); lambdaTaskRoot != "" {
		return true
	}
	return false
}

func returnError(c *gin.Context) {
	if err := recover(); err != nil {
		c.String(http.StatusNotFound, "404 not found")
	}
}
