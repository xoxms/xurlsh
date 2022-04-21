package main

import (
  "crypto/tls"
  "fmt"
  "log"
  "math/rand"
  "net/http"
  "os"
  "regexp"
  "time"

  "github.com/gin-gonic/gin"
  "github.com/go-pg/pg"
  "github.com/joho/godotenv"
)

func obtainRandomString(n int) string {
  const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"
  b := make([]byte, n)
  for i := range b {
    b[i] = letterBytes[rand.Intn(len(letterBytes))]
  }
  return string(b)
}

func main() {
  gin.SetMode(gin.ReleaseMode)
  rand.Seed(time.Now().UnixNano())

  err := godotenv.Load()
  if err != nil {
    fmt.Println("Could not load env files")
  }

  USERNAME := os.Getenv("DB_USERNAME")
  PASSWORD := os.Getenv("DB_PASSWORD")
  DATABASE := os.Getenv("DB_DATABASE")
  HOST := os.Getenv("DB_HOST")
  PORT := os.Getenv("DB_PORT")

  db := pg.Connect(&pg.Options{
    User:     USERNAME,
    Password: PASSWORD,
    Database: DATABASE,
    Addr:     HOST + ":" + PORT,
    TLSConfig: &tls.Config{
      InsecureSkipVerify: true,
    },
  })

  r := gin.Default()

  r.StaticFile("/", "./templates/index.html")
  r.Static("/public", "./public")

  r.POST("/create/url", func(c *gin.Context) {
    type Url struct {
      Url string `json:"url"`
    }

    var url Url
    err := c.BindJSON(&url)
    if err != nil {
      c.JSON(http.StatusBadRequest, gin.H{
        "error": err.Error(),
      })
      return
    }

    if url.Url == "" {
      c.JSON(http.StatusBadRequest, gin.H{
        "error": "url is required",
      })
      return
    }

    matched, err := regexp.MatchString("https?://(www\\.)?[-a-zA-Z0-9@:%._+~#=]{1,256}\\.[a-zA-Z0-9()]{1,6}\\b([-a-zA-Z0-9()@:%_+.~#?&/=]*)", url.Url)
    if err != nil {
      c.JSON(http.StatusBadRequest, gin.H{
        "error": err.Error(),
      })
    }

    if !matched {
      c.JSON(http.StatusBadRequest, gin.H{
        "error": "url is invalid",
      })
      return
    }

    rs := obtainRandomString(10)

    res, err := db.Exec("SELECT * FROM urls WHERE short_url = ?", rs)

    if err != nil {
      c.JSON(http.StatusInternalServerError, gin.H{
        "error": "internal server error",
      })
      return
    }

    if res.RowsReturned() > 0 {
      rs = obtainRandomString(10)
    }

    _, err = db.Exec("INSERT INTO urls (short_url, url) VALUES (?, ?)", rs, url.Url)

    if err != nil {
      c.JSON(http.StatusInternalServerError, gin.H{
        "error": "internal server error",
      })
      return
    }

    c.JSON(http.StatusOK, gin.H{
      "short_url": "https://x.vvx.bar/" + rs,
    })
  })

  r.GET("/:short_url", func(c *gin.Context) {
    shortUrl := c.Param("short_url")

    if shortUrl == "" {
      c.JSON(http.StatusBadRequest, gin.H{
        "error": "short_url is required",
      })
      return
    }

    var url string
    _, err := db.QueryOne(pg.Scan(&url), "SELECT url FROM urls WHERE short_url = ?", shortUrl)

    if err != nil {
      c.JSON(http.StatusInternalServerError, gin.H{
        "error": "internal server error",
      })
      return
    }

    c.Redirect(http.StatusPermanentRedirect, url)
  })

  isProduction := os.Getenv("RAILWAY_ENVIRONMENT") == "production"

  if isProduction {
    port := os.Getenv("PORT")
    log.Fatal(r.RunTLS(":"+port, "./ssl/cert.pem", "./ssl/key.pem"))
  } else {
    log.Fatal(r.Run(":8080"))
  }
}
