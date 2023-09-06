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
	"path"
	"strings"
	"time"

	"github.com/apex/gateway"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

func setupRouter() *gin.Engine {

	router := gin.Default()

	router.POST("/", runPost)

	router.GET("/", runGet)

	return router
}

var result = Result{}

var user = User{}

const keyDefault = "default" // default == "c21f969b5f03d33d43e04f8f136e7682"

func runPost(c *gin.Context) {

	defer returnError(c)

	URL := c.Query("url")
	secretKey := c.Query("secretKey")

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

	// Проверка Key
	if len(secretKey) == 0 {
		secretKey = keyDefault //c21f969b5f03d33d43e04f8f136e7682
	}

	// Создание ключа/хеша
	user.UserKey = MD5(secretKey)

	// Подключение к БД
	db, err := sql.Open("mysql", os.Getenv("CONNECTION_STRING")) // "user_name:password@tcp(host_name:3306)/db_name"
	if err != nil {
		panic(err)
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
	row := db.QueryRow("SELECT id FROM users WHERE `key`=? LIMIT 1", user.UserKey)
	err = row.Scan(&user_id)
	if err != nil {
		res, err := db.Exec("INSERT INTO users (`key`) VALUES (?)", user.UserKey)
		if err != nil {
			panic(err)
		} else {
			user_id, err = res.LastInsertId()
			if err != nil {
				panic(err)
			}
		}
	}

	// Запись URL
	_, err = db.Exec("INSERT INTO urls (url, hash, user_id) VALUES (?, ?, ?)", result.URL, result.ShortURL, user_id)
	if err != nil {
		panic(err)
	}

	// Вывод ответа
	domain := "https://mrshort.org/"
	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"URL":      result.URL,
		"ShortURL": domain + result.ShortURL,
	})
}

func runGet(c *gin.Context) {

	defer returnError(c)

	getUrl := c.Query("shortUrl")

	secretKey := c.Query("secretKey")

	if !isValidUrl(getUrl) {

		result.Status = "Wrong URL format!"
		getUrl = ""

		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"URL":    getUrl,
			"Error":  result.Status,
		})
		return
	}

	// Подключение к БД
	db, err := sql.Open("mysql", os.Getenv("CONNECTION_STRING"))
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Проверка введенного пароля user

	UserKey := MD5(secretKey)

	if len(secretKey) == 0 {
		UserKey = MD5(keyDefault)
	}

	var url_id int
	var user_id int
	var visits int
	var origUrl string

	// Забираем Path из URL
	u, _ := url.Parse(getUrl)
	hash := path.Base(u.Path)

	// Проверка на совпадение пароля в базе
	row := db.QueryRow("SELECT id FROM users WHERE `key`=? LIMIT 1", UserKey)
	err = row.Scan(&user_id)
	if err != nil {
		panic(err)
	} else if len(secretKey) == 0 {

		// Если пароль = "default" отдать только исходный URL
		rows := db.QueryRow("SELECT id, url FROM urls WHERE `hash`=? AND `user_id`=? LIMIT 1", hash, user_id)
		err = rows.Scan(&url_id, &origUrl)
		if err != nil {
			panic(err)
		}
		// StatusBadRequest
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"Error":  "Password not entered. Statistics not available.",
			"URL":    origUrl,
		})

	} else {

		// Если пароль был введен, то отдать исх.URL и кол-во визитов
		rowv := db.QueryRow("SELECT id, url FROM urls WHERE `hash`=? AND `user_id`=? LIMIT 1", hash, user_id)
		err = rowv.Scan(&url_id, &origUrl)
		if err != nil {
			panic(err)
		} else {
			rowv = db.QueryRow("SELECT visits FROM statistics WHERE `url_id`=?", url_id)
			err = rowv.Scan(&visits)
			if err != nil && err != sql.ErrNoRows {
				panic(err)
			}

			c.JSON(http.StatusOK, gin.H{
				"status": "success",
				"URL":    origUrl,
				"Visits": visits,
			})
		}
	}
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
