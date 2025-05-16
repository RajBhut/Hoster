package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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
)

func main() {
	err := godotenv.Load(".env")

	if err != nil {
		log.Fatalf("Error loading .env file")
	}
	r := gin.Default()

	// CORS settings
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173"},
		AllowCredentials: true,
		AllowHeaders:     []string{"Content-Type", "Authorization"},
	}))

	// OAuth routes
	r.GET("/github/login", githubLogin)
	r.GET("/github/callback", githubCallback)

	// API routes
	r.POST("/deploy", selectRepoHandler)
	r.GET("/refresh", refreshHandler)
	r.POST("/logout", logoutHandler)
	r.GET("/repo-details", getRepoDetailsHandler)
	r.GET("/github/userinfo", userinfo)

	// Authenticated test route
	authGroup := r.Group("/auth")
	authGroup.Use(AuthMiddleware())
	authGroup.GET("/secure", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"msg": "You are authenticated!"})
	})

	// List all deployed frontend projects
	r.GET("/deployed-projects", func(c *gin.Context) {
		entries, err := os.ReadDir(deploymentRootDir)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read deployment directory"})
			return
		}
		fmt.Println("entries: ", entries)
		var projects []gin.H

		for _, entry := range entries {
			if entry.IsDir() {
				projectName := entry.Name()
				projectPath := filepath.Join(deploymentRootDir, projectName)

				// Check for common build output directories
				buildDirs := []string{"dist", "build", "public", "out", "_site"}
				var buildDir string

				for _, dir := range buildDirs {
					path := filepath.Join(projectPath, dir)
					if info, err := os.Stat(path); err == nil && info.IsDir() {
						buildDir = dir
						break
					}
				}

				// If none of the standard directories exist, use the project directory itself
				if buildDir == "" {
					// At least check if there's an index.html file somewhere
					indexPath := findIndexHTML(projectPath)
					if indexPath != "" {
						buildDir = filepath.Dir(indexPath[len(projectPath)+1:])
						if buildDir == "." {
							buildDir = ""
						}
					}
				}

				if buildDir != "" || findIndexHTML(projectPath) != "" {
					projects = append(projects, gin.H{
						"id":       projectName,
						"name":     cleanProjectName(projectName),
						"path":     fmt.Sprintf("/projects/%s", projectName),
						"buildDir": buildDir,
					})
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{"projects": projects})
	})

	// Serve deployed frontend projects
	r.GET("/projects/:projectName/*filepath", func(c *gin.Context) {
		project := c.Param("projectName")
		requestedPath := c.Param("filepath")

		if requestedPath == "/" || requestedPath == "" {
			requestedPath = "/index.html"
		}

		// Check common build directories
		buildDirs := []string{"dist", "build", "public", "out", "_site", ""}
		var fullPath string
		var found bool

		for _, dir := range buildDirs {
			if dir != "" {
				fullPath = filepath.Join(deploymentRootDir, project, dir, requestedPath)
			} else {
				fullPath = filepath.Join(deploymentRootDir, project, requestedPath)
			}

			if _, err := os.Stat(fullPath); err == nil {
				found = true
				break
			}
		}

		if !found {
			// Try to find and serve index.html (SPA support)
			indexPath := findIndexHTML(filepath.Join(deploymentRootDir, project))
			if indexPath != "" {
				c.File(indexPath)
				return
			}

			c.String(http.StatusNotFound, "File not found: %s", requestedPath)
			return
		}

		c.File(fullPath)
	})

	// Optional: redirect to index.html if someone visits "/projects/:projectName"
	r.GET("/projects/:projectName", func(c *gin.Context) {
		project := c.Param("projectName")
		projectRoot := filepath.Join(deploymentRootDir, project)

		// Look for index.html in various directories
		indexPath := findIndexHTML(projectRoot)
		if indexPath != "" {
			c.File(indexPath)
			return
		}

		c.String(http.StatusNotFound, "Project not found")
	})

	// Catch all unmatched routes
	r.NoRoute(func(c *gin.Context) {
		c.String(http.StatusNotFound, "404 Not Found: %s", c.Request.URL.Path)
	})

	r.Run(":8000")
}

// Helper function to find index.html in a directory
//
//	func findIndexHTML(dir string) string {
//		// Check at root level
//		indexPath := filepath.Join(dir, "index.html")
//		if _, err := os.Stat(indexPath); err == nil {
//			return indexPath
//		}
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

			// Check index.html directly
			indexPath := filepath.Join(nestedDir, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				return indexPath
			}

			// Check common build directories in the nested directory
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

// 	// Check common build directories
// 	buildDirs := []string{"dist", "build", "public", "out", "_site"}
// 	for _, buildDir := range buildDirs {
// 		indexPath := filepath.Join(dir, buildDir, "index.html")
// 		if _, err := os.Stat(indexPath); err == nil {
// 			return indexPath
// 		}
// 	}

// 	// Look one level deeper in case the project is nested
// 	entries, err := os.ReadDir(dir)
// 	if err != nil {
// 		return ""
// 	}

// 	for _, entry := range entries {
// 		if entry.IsDir() {
// 			// Skip common non-project directories
// 			if entry.Name() == "node_modules" || entry.Name() == ".git" {
// 				continue
// 			}

// 			nestedDir := filepath.Join(dir, entry.Name())

// 			// Check index.html directly
// 			indexPath := filepath.Join(nestedDir, "index.html")
// 			if _, err := os.Stat(indexPath); err == nil {
// 				return indexPath
// 			}

// 			// Check common build directories in the nested directory
// 			for _, buildDir := range buildDirs {
// 				indexPath := filepath.Join(nestedDir, buildDir, "index.html")
// 				if _, err := os.Stat(indexPath); err == nil {
// 					return indexPath
// 				}
// 			}
// 		}
// 	}

// 	return ""
// }

// Helper function to clean up project names for display
func cleanProjectName(name string) string {
	// Strip timestamp from project name if present (e.g., "project-1234567890" -> "project")
	parts := strings.Split(name, "-")
	if len(parts) > 1 {
		// Check if the last part looks like a timestamp (all digits)
		lastPart := parts[len(parts)-1]
		if _, err := strconv.ParseInt(lastPart, 10, 64); err == nil {
			name = strings.Join(parts[:len(parts)-1], "-")
		}
	}

	// Replace underscores with spaces
	name = strings.ReplaceAll(name, "_", " ")

	return name
}
