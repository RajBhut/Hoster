package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	ghoauth "golang.org/x/oauth2/github"
)

var (
	jwtKey            = []byte("supersecretkey")
	githubOauthConfig = &oauth2.Config{
		ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		Scopes:       []string{"repo"},
		Endpoint:     ghoauth.Endpoint,
		RedirectURL:  "http://localhost:8000/github/callback",
	}
	state             = "randomstate"
	deploymentRootDir = "deployments"
	deployedDir       = "Deployed"
)

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	// Create necessary directories
	os.MkdirAll(deploymentRootDir, 0755)
	os.MkdirAll(deployedDir, 0755)

	r := gin.Default()

	// CORS settings
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173"},
		AllowCredentials: true,
		AllowHeaders:     []string{"Content-Type", "Authorization"},
	}))
	// r.Static("/assets", "./Deployed/Paster/dist/assets")
	// r.GET("/project/projectname", serv_react)
	// r.GET("/project/projectname/*any", serv_react)

	r.GET("/github/login", githubLogin)
	r.GET("/github/callback", githubCallback)

	r.POST("/deploy", selectRepoHandler)
	r.GET("/refresh", refreshHandler)
	r.POST("/logout", logoutHandler)
	r.GET("/repo-details", getRepoDetailsHandler)
	r.GET("/github/userinfo", userinfo)

	// Auth routes
	authGroup := r.Group("/auth")
	authGroup.Use(AuthMiddleware())
	authGroup.GET("/secure", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"msg": "You are authenticated!"})
	})

	// Deployed projects routes
	r.GET("/deployed-projects", func(c *gin.Context) {
		var projects []gin.H

		// Check the permanent Deployed directory
		deployedEntries, err := os.ReadDir(deployedDir)
		if err == nil {
			for _, entry := range deployedEntries {
				if entry.IsDir() {
					projectName := entry.Name()
					projectPath := filepath.Join(deployedDir, projectName)

					// Try to find the index.html file
					var indexPath string
					for _, buildDir := range []string{"dist", "build", "public", "out", "_site", ""} {
						if buildDir != "" {
							testPath := filepath.Join(projectPath, buildDir, "index.html")
							if _, err := os.Stat(testPath); err == nil {
								indexPath = testPath
								break
							}
						} else {
							testPath := filepath.Join(projectPath, "index.html")
							if _, err := os.Stat(testPath); err == nil {
								indexPath = testPath
								break
							}
						}
					}

					if indexPath != "" {
						projects = append(projects, gin.H{
							"name":      projectName,
							"url":       fmt.Sprintf("/projects/%s", projectName),
							"timestamp": entry.Name(),
						})
					}
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{"projects": projects})
	})
	r.GET("project/:projectname", serv_react)
	r.GET("project/:projectname/:path", func(c *gin.Context) {

	})
	r.GET("/project/:projectname/assets/*filepath", func(c *gin.Context) {
		project := c.Param("projectname")
		file := c.Param("filepath")
		assetPath := filepath.Join("Deployed", project, "dist", "assets", file)
		if _, err := os.Stat(assetPath); err == nil {
			fmt.Println("got this file", assetPath)
			c.File(assetPath)
			return
		} else {
			c.File(filepath.Join("Deployed", project, "dist", "index.html"))
		}

	})

	r.POST("/register-project", func(c *gin.Context) {
		var req RegisterRequest
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
			return
		}

		// Path where the built React project is stored (adjust as needed)
		targetPath := filepath.Join("Deployed", req.ProjectName, "dist")
		targetPath = filepath.ToSlash(targetPath) // Ensure forward slashes for Caddy

		// Ensure directory exists
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Build not found for this project"})
			return
		}

		// Load existing Caddy config
		configFile := "caddy_config.json"
		data, err := os.ReadFile(configFile)
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to read Caddy config"})
			return
		}

		var config map[string]interface{}
		if err := json.Unmarshal(data, &config); err != nil {
			c.JSON(500, gin.H{"error": "Failed to parse Caddy config"})
			return
		}

		projectHost := req.ProjectName + ".hoster.localhost"

		// THIS IS THE KEY CHANGE - FIXING THE ORDER OF ROUTES:
		// First the file_server, then the rewrite fallback
		newRoute := map[string]interface{}{
			"match": []map[string]interface{}{
				{"host": []string{projectHost}},
			},
			"handle": []map[string]interface{}{
				{
					"handler": "subroute",
					"routes": []map[string]interface{}{
						{
							// FIRST: Try to serve the actual files
							"handle": []map[string]interface{}{
								{
									"handler": "file_server",
									"root":    targetPath,
								},
							},
						},
						{
							// SECOND: Only if file not found, fall back to index.html
							"match": []map[string]interface{}{
								{"not": []map[string]interface{}{
									{"file": map[string]interface{}{"try_files": []string{"{http.request.uri.path}"}}},
								}},
							},
							"handle": []map[string]interface{}{
								{"handler": "rewrite", "uri": "/index.html"},
							},
						},
					},
				},
			},
		}

		// Append new route to config
		servers := config["apps"].(map[string]interface{})["http"].(map[string]interface{})["servers"].(map[string]interface{})
		srv0 := servers["srv0"].(map[string]interface{})
		routes := srv0["routes"].([]interface{})
		srv0["routes"] = append(routes, newRoute)

		// Save updated config
		newConfigBytes, _ := json.MarshalIndent(config, "", "  ")
		if err := os.WriteFile(configFile, newConfigBytes, 0644); err != nil {
			c.JSON(500, gin.H{"error": "Failed to update Caddy config file"})
			return
		}

		resp, err := http.Post("http://localhost:2019/load", "application/json", bytes.NewBuffer(newConfigBytes))
		if err != nil || resp.StatusCode >= 400 {
			c.JSON(500, gin.H{"error": "Caddy reload failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Project registered successfully",
			"url":     fmt.Sprintf("http://%s", projectHost),
		})
	})
	r.Run(":8000")
}

type RegisterRequest struct {
	ProjectName string `json:"projectName"`
}

// Helper function to find index.html in a project directory
func findIndexHTML(dir string) string {
	// Check at root level
	indexPath := filepath.Join(dir, "index.html")
	if _, err := os.Stat(indexPath); err == nil {
		return indexPath
	}

	buildDirs := []string{"dist", "build", "public", "out", "_site"}
	for _, buildDir := range buildDirs {
		indexPath := filepath.Join(dir, buildDir, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			return indexPath
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			if entry.Name() == "node_modules" || entry.Name() == ".git" {
				continue
			}

			nestedDir := filepath.Join(dir, entry.Name())

			// Check at nested root
			indexPath := filepath.Join(nestedDir, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				return indexPath
			}

			// Check in build directories of nested folder
			for _, buildDir := range buildDirs {
				indexPath := filepath.Join(nestedDir, buildDir, "index.html")
				if _, err := os.Stat(indexPath); err == nil {
					return indexPath
				}
			}
		}
	}

	return ""
}

func serv_react(c *gin.Context) {
	project := c.Param("projectname")
	indexpath := filepath.Join("Deployed", project, "dist", "index.html")
	c.File(indexpath)
}
