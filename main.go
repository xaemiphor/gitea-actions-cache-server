package main

import (
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
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

func decodePayloadId(source string) (string, string) {
	data, err := base32.StdEncoding.DecodeString(source)
	if err != nil {
		fmt.Println("error:", err)
	}
	payload := strings.Split(string(data), "|")
	return payload[0], payload[1]
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
		fmt.Println("middlewareLogPayload - Couldn't bind to payload")
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

	// Checks if payload exists, return download URL if avail
	r.GET("/_apis/artifactcache/cache", func(c *gin.Context) {
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		origin := c.Request.Host + c.Request.URL.Host
		key := c.Query("keys")
		version := c.Query("version")
		cacheFile := encodePayloadId(key, version)
		_, err := os.Stat("/data/" + cacheFile) // TODO Use the returned fileInfo to determine if cache should be cleaned, etc
		if err == nil {
			// Found
			c.JSON(200, gin.H{
				"archiveLocation": scheme + "://" + origin + "/download/" + cacheFile,
				"cacheKey":        cacheFile,
			})
		} else if errors.Is(err, os.ErrNotExist) {
			c.Writer.WriteHeader(204) // Not found
		} else {
			c.Writer.WriteHeader(400) // Neither found nor not found
		}
	})

	// Reserves the payload
	r.POST("/_apis/artifactcache/caches", func(c *gin.Context) {
		var jsonData map[string]interface{}
		if err := c.ShouldBindBodyWith(&jsonData, binding.JSON); err != nil {
			fmt.Println("r.POST - couldn't bind with payload")
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
		if err != nil {
			log.Fatal("Error creating file:", err)
			c.Writer.WriteHeader(400)
		} else if success {
			c.JSON(200, gin.H{
				"cacheId": cacheFile,
			})
		} else {
			c.Writer.WriteHeader(400)
		}
	})

	// Provides the payload
	r.Static("/download", "/data") // TODO Expirey of payloads

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
