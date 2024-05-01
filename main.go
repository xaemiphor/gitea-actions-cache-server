package main

import (
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"log"
	"net/http"
	"os"
)

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func encodePayloadId(key string, version string) string {
	payload := []byte(key + "|" + version)
	return base32.StdEncoding.EncodeToString(payload)
}

func decodePayloadId(source string) string {
	data, err := base32.StdEncoding.DecodeString(source)
	if err != nil {
		fmt.Println("error:", err)
	}
	return string(data)
}

func createEmptyFile(target string) (bool, error) {
	fmt.Println("Attempting /data/" + target)
	myfile, e := os.Create("/data/" + target)
	myfile.Close()
	return (e == nil), e
}

func middlewareLogPayload(c *gin.Context) {
	var jsonData map[string]interface{}
	// Extract request information
	requestURI := c.Request.RequestURI
	requestMethod := c.Request.Method
	headerData := c.Request.Header
	queryParams := c.Request.URL.Query()
	if err := c.ShouldBindBodyWith(&jsonData, binding.JSON); err != nil {
		fmt.Println("?")
	}

	// Construct JSON payload
	payload := gin.H{
		"requestURI":    requestURI,
		"requestMethod": requestMethod,
		"headerData":    headerData,
		"queryParams":   queryParams,
		"jsonData":      jsonData,
	}
	payloadJson, _ := json.Marshal(payload)
	fmt.Println(string(payloadJson))

	c.Next()
}

func main() {
	r := gin.Default()

	r.Use(middlewareLogPayload)

	// Define the /ping route
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	r.GET("/_apis/artifactcache/cache", func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		key := c.Query("keys")
		version := c.Query("version")
		cacheFile := encodePayloadId(key, version)
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

	r.POST("/_apis/artifactcache/caches", func(c *gin.Context) {
		var jsonData map[string]interface{}
		if err := c.ShouldBindBodyWith(&jsonData, binding.JSON); err != nil {
			fmt.Println("?")
		}

		key, ok := jsonData["key"].(string)
		if !ok {
			fmt.Println(http.StatusBadRequest, gin.H{"error": "Invalid key value"})
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid key value"})
			return
		}
		version, ok := jsonData["version"].(string)
		if !ok {
			fmt.Println(http.StatusBadRequest, gin.H{"error": "Invalid version value"})
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid version value"})
			return
		}
		cacheFile := encodePayloadId(key, version)
		success, err := createEmptyFile(cacheFile)
		fmt.Println(err)
		fmt.Println(success)
		if err != nil {
			log.Fatal("Error creating file:", err)
			c.Writer.WriteHeader(400)
		} else if success {
			c.Writer.WriteHeader(200)
		} else {
			c.Writer.WriteHeader(400)
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
		c.JSON(404, payload)
	})

	// Start the server
	r.Run(":" + getEnv("PORT", "8080"))
}
