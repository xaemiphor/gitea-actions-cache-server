package main

import (
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

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
	myfile, e := os.Create("/data/" + target)
	myfile.Close()
	return (e == nil), e
}

// Credit to https://stackoverflow.com/a/53626880
func addToFile(filepath string, startByte int, data []byte) {
	f, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		panic("File not found")
	}
	whence := io.SeekStart
	_, err = f.Seek(int64(startByte), whence)
	f.Write(data)
	f.Sync() //flush to disk
	f.Close()
}

func doesFileExist(filepath string) (bool, os.FileInfo, error) {
	fileInfo, err := os.Stat(filepath)
	if err != nil {
		return false, fileInfo, err
	}
	fileSize := int(fileInfo.Size())
	fileDate := fileInfo.ModTime()
	fileAge := time.Since(fileDate)
	if fileSize == 0 || fileAge.Hours() > (24*7) {
		if err := os.Remove(filepath); err != nil {
			log.Fatal(err)
		}
		return false, nil, err
	} else {
		return true, fileInfo, err
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func middlewareLogPayload(c *gin.Context) {
	var jsonData map[string]interface{}
	// Extract request information
	requestURI := c.Request.RequestURI
	requestMethod := c.Request.Method
	headerData := c.Request.Header
	queryParams := c.Request.URL.Query()
	if requestMethod == "POST" {
		if err := c.ShouldBindBodyWith(&jsonData, binding.JSON); err != nil {
			fmt.Println("middleware - couldn't bind with payload")
		}
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
		key := c.Query("keys") // TODO This is a list, not handled great here
		version := c.Query("version")
		cacheFile := encodePayloadId(key, version)
		fileExist, _, err := doesFileExist("/data/" + cacheFile)
		if fileExist {
			// File exists
			c.JSON(200, gin.H{
				"cacheKey":        cacheFile,
				"archiveLocation": scheme + "://" + origin + "/download/" + cacheFile,
				"result":          "hit",
			})
			return
		} else if errors.Is(err, os.ErrNotExist) {
			// File doesn't exist and returned an error
			c.Writer.WriteHeader(400)
			return
		} else {
			// File doesn't exist, no error
			c.Writer.WriteHeader(204)
			return
		}
	})

	// Reserves the payload path
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
		fileExist, _, _ := doesFileExist("/data/" + cacheFile)
		if fileExist {
			// File exists
			c.Writer.WriteHeader(400)
			return
		}
		success, err := createEmptyFile(cacheFile + ".inprogress")
		if err != nil {
			log.Fatal("Error creating file:", err)
			c.Writer.WriteHeader(400)
			return
		} else if success {
			c.JSON(200, gin.H{
				"cacheId": cacheFile,
			})
			return
		} else {
			c.Writer.WriteHeader(400)
			return
		}
	})

	// Receive and write payload
	r.PATCH("/_apis/artifactcache/caches/:cacheId", func(c *gin.Context) {
		cacheId := c.Param("cacheId")
		//key, version := decodePayloadId(cacheId)
		contentRange := c.GetHeader("Content-Range")
		// TODO There has to be a better way to parse the contentRange
		rangeSplit := strings.Split(string(contentRange), "/")
		rangeSplit = strings.Split(string(rangeSplit[0]), " ")
		rangeSplit = strings.Split(string(rangeSplit[1]), "-")
		startByte, _ := strconv.Atoi(rangeSplit[0])
		if c.GetHeader("Content-Type") != "application/octet-stream" {
			err := fmt.Errorf("required octet-stream")
			c.AbortWithStatusJSON(400, map[string]string{"message": err.Error()})
			return
		}

		body, _ := c.GetRawData()

		addToFile("/data/"+cacheId+".inprogress", startByte, body)
		c.Writer.WriteHeader(200)
	})

	// Upload complete
	r.POST("/_apis/artifactcache/caches/:cacheId", func(c *gin.Context) {
		cacheFile := c.Param("cacheId")
		var jsonData map[string]interface{}
		if err := c.ShouldBindBodyWith(&jsonData, binding.JSON); err != nil {
			fmt.Println("r.POST - couldn't bind with payload")
		}
		payloadSize := int(jsonData["size"].(float64))
		fileInfo, err := os.Stat("/data/" + cacheFile + ".inprogress")
		if err != nil {
			fmt.Println("File not found " + cacheFile + ".inprogress")
			c.Writer.WriteHeader(400)
			return
		}
		fileSize := int(fileInfo.Size())

		if fileSize == payloadSize {
			e := os.Rename("/data/"+cacheFile+".inprogress", "/data/"+cacheFile)
			if e != nil {
				log.Fatal(e)
				c.Writer.WriteHeader(500)
				return
			}
			c.Writer.WriteHeader(200)
			return
		}
		c.Writer.WriteHeader(400)
		return

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
