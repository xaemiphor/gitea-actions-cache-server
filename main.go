package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"os"
	"errors"
)

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func cacheFilename(key string, version string) string {
	return key + "-" + version
}

func main() {
	r := gin.Default()

	// Define the /ping route
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	r.GET("/_apis/artifactcache/cache", func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		keys := c.Query("keys")
		version := c.Query("version")
		cacheFile := cacheFilename(keys, version)
		fmt.Println(string(keys))
		//fileInfo, err := os.Stat("/data/" + cacheFile)
		_, err := os.Stat("/data/" + cacheFile)
		if err == nil {
			// Found
			c.JSON(200, gin.H{
				"archiveLocation": origin + "/download/" + cacheFile,
				"cacheKey":        cacheFile,
			})
		} else if errors.Is(err, os.ErrNotExist) {
			c.Writer.WriteHeader(204) // Not found
		} else {
			c.Writer.WriteHeader(400) // Neither found nor not found
		}
	})

	r.NoRoute(func(c *gin.Context) {
		// Extract request information
		requestURI := c.Request.RequestURI
		requestMethod := c.Request.Method
		headerData := c.Request.Header
		queryParams := c.Request.URL.Query()
		postData, _ := c.GetRawData() // Assumes POST data is JSON

		// Construct JSON payload
		payload := gin.H{
			"requestURI":    requestURI,
			"requestMethod": requestMethod,
			"headerData":    headerData,
			"queryParams":   queryParams,
			"postData":      string(postData),
		}
		payloadJson, _ := json.Marshal(payload)
		fmt.Println(string(payloadJson))
		c.JSON(404, payload)
	})

	// Start the server
	r.Run(":" + getEnv("PORT", "8080"))
}
