package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"os"
)

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func main() {
	r := gin.Default()

	// Define the /ping route
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	r.NoRoute(func(c *gin.Context) {
		// Extract request information
		requestURI := c.Request.RequestURI
		requestMethod := c.Request.Method
		queryParams := c.Request.URL.Query()
		postData, _ := c.GetRawData() // Assumes POST data is JSON

		// Construct JSON payload
		payload := gin.H{
			"requestURI":    requestURI,
			"requestMethod": requestMethod,
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
