package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/apex/gateway"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

func setupRouter() *gin.Engine {

	router := gin.Default()

	router.POST("/", runPost)

	return router
}

func runPost(c *gin.Context) {

	defer returnError(c)

	URL := c.Query("url")
	result := Result{}

	// Проверка URL
	if !isValidUrl(URL) {

		result.Status = "Wrong URL format!"
		result.URL = ""

		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"URL":    result.URL,
			"Error":  result.Status,
		})
		return
	} else {
		result.URL = URL
	}

	// Проверка X-RapidAPI-Key
	rawUserKey := c.GetHeader("X-RapidAPI-Key")
	if len(rawUserKey) == 0 {
		panic("Error: invalid key!")
	}
	UserKey := MD5(rawUserKey)

	// Подключение к БД
	db, err := sql.Open("mysql", "admin:zePh8gTsiawj2NWhze6p@tcp(urlshortener.cottl9aqqnvh.us-east-1.rds.amazonaws.com)/UrlShortener")
	if err != nil {
		panic("Error! No connection to DB")
	}
	defer db.Close()

	// Проверка hash
	var url_id int
	result.ShortURL = shortener()
	rowHash := db.QueryRow("SELECT id FROM urls WHERE `hash`=? LIMIT 1", result.ShortURL)
	err = rowHash.Scan(&url_id)
	if err == nil {
		result.ShortURL = shortener()
	}

	// Проверка user
	var user_id int64
	row := db.QueryRow("SELECT id FROM users WHERE `key`=? LIMIT 1", UserKey)
	err = row.Scan(&user_id)
	if err != nil {
		res, err := db.Exec("INSERT INTO users (`key`) VALUES (?)", UserKey)
		if err != nil {
			panic("Error! Failed to write data")
		} else {
			user_id, err = res.LastInsertId()
			if err != nil {
				panic("Error! Failed to write data ID")
			}
		}
	}

	// Запись URL
	_, err = db.Exec("INSERT INTO urls (url, hash, user_id) VALUES (?, ?, ?)", result.URL, result.ShortURL, user_id)
	if err != nil {
		panic("Error! Failed to write URL")
	}

	// Вывод ответа
	domain := "https://mrshort.org/"
	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"URL":      result.URL,
		"ShortURL": domain + result.ShortURL,
	})
}

type User struct {
	Id      string `json:"id"`
	UserKey string `json:"key"`
}

type Result struct {
	URL      string `json:"url"`
	ShortURL string `json:"hash"`
	UserId   string `json:"user_id"`
	Status   string `json:"status"`
}

func MD5(text string) string {
	algorithm := md5.New()
	algorithm.Write([]byte(text))
	return hex.EncodeToString(algorithm.Sum(nil))
}

func isValidUrl(str string) bool {
	_, err := url.ParseRequestURI(str)
	if err != nil {
		return false
	}
	u, err := url.Parse(str)
	if err != nil || u.Host == "" {
		return false
	}
	return true
}

func shortener() string {
	rand.Seed(time.Now().UnixNano())
	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		"0123456789")
	length := 7
	var b strings.Builder
	for i := 0; i < length; i++ {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}
	str := b.String()

	return str
}

func main() {
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
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": err,
		})
	}
}
