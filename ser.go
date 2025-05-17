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
	r.Use(Handlerefrer())

	r.GET("/github/login", githubLogin)
	r.GET("/github/callback", githubCallback)

	// API routes
	r.POST("/deploy", selectRepoHandler)
	r.GET("/refresh", refreshHandler)
	r.POST("/logout", logoutHandler)
	r.GET("/repo-details", getRepoDetailsHandler)
	r.GET("/github/userinfo", userinfo)

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

	r.GET("/projects/:projectName/*filepath", func(c *gin.Context) {
		project := c.Param("projectName")
		requestedPath := c.Param("filepath")

		if requestedPath == "/" || requestedPath == "" {
			requestedPath = "/index.html"
		}

		fmt.Println("Requested project:", project)
		fmt.Println("Requested path:", requestedPath)

		projectRoot := filepath.Join(deploymentRootDir, project)
		distPath := filepath.Join(projectRoot, "dist")
		if _, err := os.Stat(distPath); err == nil {
			fmt.Println("Dist directory exists")
			// List contents of assets if it exists
			assetsPath := filepath.Join(distPath, "assets")
			if _, err := os.Stat(assetsPath); err == nil {
				assetFiles, err := os.ReadDir(assetsPath)
				if err == nil {
					fmt.Println("Assets directory contents:")
					for _, file := range assetFiles {
						fmt.Println("- ", file.Name())
					}
				}
			}
		}

		if requestedPath == "/index.html" {
			indexPath := findIndexHTML(projectRoot)
			if indexPath != "" {
				htmlContent, err := os.ReadFile(indexPath)
				if err == nil {
					htmlString := string(htmlContent)

					assetsDir := filepath.Join(projectRoot, "dist/assets")
					if _, err := os.Stat(assetsDir); err == nil {
						files, _ := os.ReadDir(assetsDir)
						var jsFile, cssFile string

						for _, file := range files {
							if strings.HasSuffix(file.Name(), ".js") {
								jsFile = file.Name()
							}
							if strings.HasSuffix(file.Name(), ".css") {
								cssFile = file.Name()
							}
						}

						if jsFile != "" {
							htmlString = strings.Replace(htmlString,
								"/src/main.jsx",
								fmt.Sprintf("/projects/%s/dist/assets/%s", project, jsFile),
								-1)

							htmlString = strings.Replace(htmlString,
								"src/main.jsx",
								fmt.Sprintf("/projects/%s/dist/assets/%s", project, jsFile),
								-1)
						}

						if cssFile != "" {
							htmlString = strings.Replace(htmlString,
								"/src/index.css",
								fmt.Sprintf("/projects/%s/dist/assets/%s", project, cssFile),
								-1)

							htmlString = strings.Replace(htmlString,
								"src/index.css",
								fmt.Sprintf("/projects/%s/dist/assets/%s", project, cssFile),
								-1)
						}
					}

					htmlString = strings.Replace(htmlString,
						`type="module" src="/src/`,
						fmt.Sprintf(`type="module" src="/projects/%s/dist/assets/`, project),
						-1)

					c.Data(http.StatusOK, "text/html", []byte(htmlString))
					return
				}

				c.File(indexPath)
				return
			}
		}

		if strings.HasPrefix(requestedPath, "/src/") {
			// For .jsx files, we need to serve the main JS bundle
			if strings.HasSuffix(requestedPath, ".jsx") || strings.HasSuffix(requestedPath, ".tsx") {
				// Find and serve the main JS bundle from dist/assets
				assetsDir := filepath.Join(deploymentRootDir, project, "dist", "assets")

				// If that specific directory doesn't exist, try to find any assets directory
				if _, err := os.Stat(assetsDir); err != nil {
					// Try other common build directories
					for _, dir := range []string{"build", "public", "out", "_site"} {
						possibleDir := filepath.Join(deploymentRootDir, project, dir, "assets")
						if _, err := os.Stat(possibleDir); err == nil {
							assetsDir = possibleDir
							break
						}

						// Also check for just the build dir with no assets subdirectory
						possibleDir = filepath.Join(deploymentRootDir, project, dir)
						if _, err := os.Stat(possibleDir); err == nil {
							assetsDir = possibleDir
							break
						}
					}
				}

				// Now that we have an assets directory (hopefully), look for JS files
				files, err := os.ReadDir(assetsDir)
				if err == nil {
					// Look for index or main JS files first
					for _, file := range files {
						fileName := file.Name()
						if strings.HasSuffix(fileName, ".js") &&
							(strings.HasPrefix(fileName, "index-") || strings.HasPrefix(fileName, "main-")) {
							fullPath := filepath.Join(assetsDir, fileName)
							fmt.Println("Serving compiled JS bundle:", fullPath)
							c.File(fullPath)
							return
						}
					}

					// If no index/main file found, serve any JS file
					for _, file := range files {
						if strings.HasSuffix(file.Name(), ".js") {
							fullPath := filepath.Join(assetsDir, file.Name())
							fmt.Println("Serving fallback JS file:", fullPath)
							c.File(fullPath)
							return
						}
					}
				}
			}

			// For CSS, SVG and other assets
			if strings.HasSuffix(requestedPath, ".css") ||
				strings.HasSuffix(requestedPath, ".svg") {

				assetsDir := filepath.Join(deploymentRootDir, project, "dist", "assets")
				if _, err := os.Stat(assetsDir); err == nil {
					ext := filepath.Ext(requestedPath)
					files, err := os.ReadDir(assetsDir)
					if err == nil {
						for _, file := range files {
							if strings.HasSuffix(file.Name(), ext) {
								fullPath := filepath.Join(assetsDir, file.Name())
								fmt.Println("Serving asset:", fullPath)
								c.File(fullPath)
								return
							}
						}
					}
				}
			}

			// Try the source path as a fallback
			srcPath := filepath.Join(deploymentRootDir, project, requestedPath[1:])
			if _, err := os.Stat(srcPath); err == nil {
				c.File(srcPath)
				return
			}
		}

		// Check common build directories first
		buildDirs := []string{"dist", "build", "public", "out", "_site", ""}
		var fullPath string
		var found bool

		for _, dir := range buildDirs {
			var testPath string
			if dir != "" {
				testPath = filepath.Join(deploymentRootDir, project, dir, requestedPath)
			} else {
				testPath = filepath.Join(deploymentRootDir, project, requestedPath)
			}

			if _, err := os.Stat(testPath); err == nil {
				fullPath = testPath
				found = true
				break
			}
		}

		if !found && (strings.HasSuffix(requestedPath, ".js") || strings.HasSuffix(requestedPath, ".css") ||
			strings.HasSuffix(requestedPath, ".png") || strings.HasSuffix(requestedPath, ".jpg") ||
			strings.HasSuffix(requestedPath, ".svg") || strings.HasSuffix(requestedPath, ".ico")) {

			// For asset files that aren't found, check if they have a hash in the filename
			dir := filepath.Dir(requestedPath)
			base := filepath.Base(requestedPath)
			ext := filepath.Ext(requestedPath)
			prefix := strings.TrimSuffix(base, ext)

			// Look in each build directory
			for _, buildDir := range buildDirs {
				var assetDir string
				if buildDir != "" {
					assetDir = filepath.Join(deploymentRootDir, project, buildDir, dir)
				} else {
					assetDir = filepath.Join(deploymentRootDir, project, dir)
				}

				// Try to find files with the same extension and prefix
				files, err := os.ReadDir(assetDir)
				if err == nil {
					for _, file := range files {
						if !file.IsDir() && strings.HasPrefix(file.Name(), prefix) && strings.HasSuffix(file.Name(), ext) {
							fullPath = filepath.Join(assetDir, file.Name())
							found = true
							break
						}
					}
				}
				if found {
					break
				}
			}
		}

		if !found {
			// Try specific assets directories for different build tools
			assetDirs := []string{
				filepath.Join(deploymentRootDir, project, "dist", "assets"),
				filepath.Join(deploymentRootDir, project, "build", "static"),
				filepath.Join(deploymentRootDir, project, "out", "_next"),
			}

			for _, assetsDir := range assetDirs {
				if _, err := os.Stat(assetsDir); err == nil {
					files, err := os.ReadDir(assetsDir)
					if err == nil && len(files) > 0 {
						// If we're looking for a JS file
						if strings.HasSuffix(requestedPath, ".js") || strings.HasSuffix(requestedPath, ".jsx") {
							for _, file := range files {
								if strings.HasSuffix(file.Name(), ".js") {
									c.File(filepath.Join(assetsDir, file.Name()))
									return
								}
							}
						}

						// If we're looking for a CSS file
						if strings.HasSuffix(requestedPath, ".css") {
							for _, file := range files {
								if strings.HasSuffix(file.Name(), ".css") {
									c.File(filepath.Join(assetsDir, file.Name()))
									return
								}
							}
						}

						if strings.HasSuffix(requestedPath, ".svg") || strings.HasSuffix(requestedPath, ".png") {
							for _, file := range files {
								if strings.HasSuffix(file.Name(), filepath.Ext(requestedPath)) {
									c.File(filepath.Join(assetsDir, file.Name()))
									return
								}
							}
						}
					}
				}
			}

			// SPA routing support - serve index.html for paths that don't match files
			if !strings.Contains(filepath.Base(requestedPath), ".") {
				indexPath := findIndexHTML(filepath.Join(deploymentRootDir, project))
				if indexPath != "" {
					fmt.Println("Serving SPA fallback:", indexPath)
					c.File(indexPath)
					return
				}
			}

			c.String(http.StatusNotFound, "File not found: %s", requestedPath)
			return
		}

		c.File(fullPath)
	})
	r.GET("/projects/:projectName", func(c *gin.Context) {
		project := c.Param("projectName")
		projectRoot := filepath.Join(deploymentRootDir, project)

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

			//indexPath := filepath.Join(nestedDir, "index.html")
			// if _, err := os.Stat(indexPath); err == nil {
			// 	return indexPath
			// }

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
