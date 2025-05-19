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
	deploymentRootDir = "Deployed"
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

				buildDirs := []string{"dist", "build", "public", "out", "_site"}
				var buildDir string

				for _, dir := range buildDirs {
					path := filepath.Join(projectPath, dir)
					if info, err := os.Stat(path); err == nil && info.IsDir() {
						buildDir = dir
						break
					}
				}

				if buildDir == "" {
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

		cleanProjectName := strings.Split(project, "-")[0]

		if requestedPath == "/" || requestedPath == "" {
			requestedPath = "/index.html"
		}

		fmt.Println("Requested project:", project)
		fmt.Println("Requested path:", requestedPath)

		// Search locations - first look in temporary deployment, then in permanent Deployed folder
		searchLocations := []struct {
			baseDir string
			project string
		}{
			// Temporary deployment
			{"Deployed", cleanProjectName}, // Permanent storage
		}

		fmt.Println("Search locations:", searchLocations)

		for _, loc := range searchLocations {
			rootDir := filepath.Join(loc.baseDir, loc.project)
			distPath := filepath.Join(rootDir, "dist")
			if _, err := os.Stat(distPath); err == nil {

				fmt.Printf("Found dist directory in %s\n", rootDir)
				assetsPath := filepath.Join(distPath, "assets")

				if _, err := os.Stat(assetsPath); err == nil {
					assetFiles, err := os.ReadDir(assetsPath)
					if err == nil {
						fmt.Printf("Assets in %s:\n", assetsPath)
						for _, file := range assetFiles {
							fmt.Println("- ", file.Name())
						}
					}
				}
			}
		}

		if requestedPath == "/index.html" {
			for _, loc := range searchLocations {
				rootDir := filepath.Join(loc.baseDir, loc.project)
				indexPath := findIndexHTML(rootDir)
				if indexPath != "" {
					fmt.Printf("Found index.html at %s\n", indexPath)
					htmlContent, err := os.ReadFile(indexPath)
					if err == nil {
						// Modify HTML to fix asset paths
						htmlString := string(htmlContent)

						// Find assets directory and get JS/CSS files
						var assetsDir string
						var jsFile, cssFile string

						// Check both potential asset locations
						for _, loc := range searchLocations {
							potentialAssetsDir := filepath.Join(loc.baseDir, loc.project, "dist", "assets")
							if _, err := os.Stat(potentialAssetsDir); err == nil {
								assetsDir = potentialAssetsDir
								files, _ := os.ReadDir(assetsDir)
								for _, file := range files {
									if strings.HasSuffix(file.Name(), ".js") {
										jsFile = file.Name()
									}
									if strings.HasSuffix(file.Name(), ".css") {
										cssFile = file.Name()
									}
								}
								// Use the first valid assets directory we find
								if jsFile != "" || cssFile != "" {
									break
								}
							}
						}

						if assetsDir != "" {
							fmt.Printf("Using assets from: %s\n", assetsDir)

							var projectPath string
							if strings.HasPrefix(assetsDir, "Deployed") {
								projectPath = fmt.Sprintf("/projects/%s", cleanProjectName)
							} else {
								projectPath = fmt.Sprintf("/projects/%s", project)
							}

							if jsFile != "" {
								htmlString = strings.Replace(htmlString,
									"/src/main.jsx",
									fmt.Sprintf("%s/dist/assets/%s", projectPath, jsFile),
									-1)

								htmlString = strings.Replace(htmlString,
									"src/main.jsx",
									fmt.Sprintf("%s/dist/assets/%s", projectPath, jsFile),
									-1)
							}

							if cssFile != "" {
								htmlString = strings.Replace(htmlString,
									"/src/index.css",
									fmt.Sprintf("%s/dist/assets/%s", projectPath, cssFile),
									-1)

								htmlString = strings.Replace(htmlString,
									"src/index.css",
									fmt.Sprintf("%s/dist/assets/%s", projectPath, cssFile),
									-1)
							}

							// Generic module source replacement
							htmlString = strings.Replace(htmlString,
								`type="module" src="/src/`,
								fmt.Sprintf(`type="module" src="%s/dist/assets/`, projectPath),
								-1)
						}

						c.Data(http.StatusOK, "text/html", []byte(htmlString))
						return
					}

					c.File(indexPath)
					return
				}
			}
		}

		if strings.HasPrefix(requestedPath, "/src/") {
			if strings.HasSuffix(requestedPath, ".jsx") || strings.HasSuffix(requestedPath, ".tsx") {
				for _, loc := range searchLocations {
					rootDir := filepath.Join(loc.baseDir, loc.project)

					assetsDir := filepath.Join(rootDir, "dist", "assets")

					if _, err := os.Stat(assetsDir); err != nil {
						for _, dir := range []string{"build", "public", "out", "_site"} {
							possibleDir := filepath.Join(rootDir, dir, "assets")
							if _, err := os.Stat(possibleDir); err == nil {
								assetsDir = possibleDir
								break
							}

							possibleDir = filepath.Join(rootDir, dir)
							if _, err := os.Stat(possibleDir); err == nil {
								assetsDir = possibleDir
								break
							}
						}
					}

					// Look for JS files in the assets directory
					files, err := os.ReadDir(assetsDir)
					if err == nil {
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

						// If no specific file found, serve any JS file
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
			}

			// For CSS, SVG and other assets
			if strings.HasSuffix(requestedPath, ".css") || strings.HasSuffix(requestedPath, ".svg") {
				ext := filepath.Ext(requestedPath)

				for _, loc := range searchLocations {
					rootDir := filepath.Join(loc.baseDir, loc.project)
					assetsDir := filepath.Join(rootDir, "dist", "assets")

					if _, err := os.Stat(assetsDir); err == nil {
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
			}

			// Try the source path as a fallback
			for _, loc := range searchLocations {
				srcPath := filepath.Join(loc.baseDir, loc.project, requestedPath[1:])
				if _, err := os.Stat(srcPath); err == nil {
					c.File(srcPath)
					return
				}
			}
		}

		// Check common build directories in all search locations
		buildDirs := []string{"dist", "build", "public", "out", "_site", ""}

		for _, loc := range searchLocations {
			rootDir := filepath.Join(loc.baseDir, loc.project)

			for _, dir := range buildDirs {
				var testPath string
				if dir != "" {
					testPath = filepath.Join(rootDir, dir, requestedPath)
				} else {
					testPath = filepath.Join(rootDir, requestedPath)
				}

				if _, err := os.Stat(testPath); err == nil {
					c.File(testPath)
					return
				}
			}
		}

		// Handle asset files with hashed filenames
		if strings.HasSuffix(requestedPath, ".js") ||
			strings.HasSuffix(requestedPath, ".css") ||
			strings.HasSuffix(requestedPath, ".png") ||
			strings.HasSuffix(requestedPath, ".jpg") ||
			strings.HasSuffix(requestedPath, ".svg") ||
			strings.HasSuffix(requestedPath, ".ico") {

			dir := filepath.Dir(requestedPath)
			base := filepath.Base(requestedPath)
			ext := filepath.Ext(requestedPath)
			prefix := strings.TrimSuffix(base, ext)

			for _, loc := range searchLocations {
				rootDir := filepath.Join(loc.baseDir, loc.project)

				for _, buildDir := range buildDirs {
					var assetDir string
					if buildDir != "" {
						assetDir = filepath.Join(rootDir, buildDir, dir)
					} else {
						assetDir = filepath.Join(rootDir, dir)
					}

					// Try to find files with the same extension and prefix
					files, err := os.ReadDir(assetDir)
					if err == nil {
						for _, file := range files {
							if !file.IsDir() &&
								strings.HasPrefix(file.Name(), prefix) &&
								strings.HasSuffix(file.Name(), ext) {
								c.File(filepath.Join(assetDir, file.Name()))
								return
							}
						}
					}
				}
			}
		}

		// Try specific assets directories for different build tools
		for _, loc := range searchLocations {
			rootDir := filepath.Join(loc.baseDir, loc.project)

			assetDirs := []string{
				filepath.Join(rootDir, "dist", "assets"),
				filepath.Join(rootDir, "build", "static"),
				filepath.Join(rootDir, "out", "_next"),
			}

			for _, assetsDir := range assetDirs {
				if _, err := os.Stat(assetsDir); err == nil {
					files, err := os.ReadDir(assetsDir)
					if err == nil && len(files) > 0 {
						// Match file type with any available asset
						var targetExt string
						if strings.HasSuffix(requestedPath, ".js") ||
							strings.HasSuffix(requestedPath, ".jsx") {
							targetExt = ".js"
						} else if strings.HasSuffix(requestedPath, ".css") {
							targetExt = ".css"
						} else if strings.HasSuffix(requestedPath, ".svg") ||
							strings.HasSuffix(requestedPath, ".png") {
							targetExt = filepath.Ext(requestedPath)
						}

						if targetExt != "" {
							for _, file := range files {
								if strings.HasSuffix(file.Name(), targetExt) {
									c.File(filepath.Join(assetsDir, file.Name()))
									return
								}
							}
						}
					}
				}
			}
		}

		if !strings.Contains(filepath.Base(requestedPath), ".") {
			for _, loc := range searchLocations {
				rootDir := filepath.Join(loc.baseDir, loc.project)
				indexPath := findIndexHTML(rootDir)
				if indexPath != "" {
					fmt.Println("Serving SPA fallback:", indexPath)
					c.File(indexPath)
					return
				}
			}
		}

		c.String(http.StatusNotFound, "File not found: %s", requestedPath)
	})

	// r.GET("/projects/:projectName", func(c *gin.Context) {
	// 	project := c.Param("projectName")
	// 	cleanProjectName := strings.Split(project, "-")[0]

	// 	searchLocations := []struct {
	// 		baseDir string
	// 		project string
	// 	}{

	// 		{"Deployed", cleanProjectName},
	// 	}
	// 	fmt.Println("Search locations:", searchLocations)
	// 	fmt.Println("Project name:", cleanProjectName)
	// 	for _, loc := range searchLocations {
	// 		rootDir := filepath.Join(loc.baseDir, loc.project)
	// 		fmt.Println("Root directory:", rootDir)
	// 		indexPath := findIndexHTML(rootDir)

	// 		fmt.Println("Index path:", indexPath)
	// 		if indexPath != "" {
	// 			c.File(indexPath)
	// 			return
	// 		}
	// 	}

	// 	c.String(http.StatusNotFound, "Project not found")
	// })
	r.GET("/projects/:projectName", func(c *gin.Context) {
		project := c.Param("projectName")
		cleanProjectName := strings.Split(project, "-")[0]

		searchLocations := []struct {
			baseDir string
			project string
		}{
			{"Deployed", cleanProjectName},
		}

		fmt.Println("Search locations:", searchLocations)
		fmt.Println("Project name:", cleanProjectName)

		for _, loc := range searchLocations {
			rootDir := filepath.Join(loc.baseDir, loc.project)
			fmt.Println("Root directory:", rootDir)
			indexPath := findIndexHTML(rootDir)

			fmt.Println("Index path:", indexPath)
			if indexPath != "" {

				htmlContent, err := os.ReadFile(indexPath)
				if err == nil {
					htmlString := string(htmlContent)

					assetsDir := filepath.Join(rootDir, "dist", "assets")
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

						projectPath := fmt.Sprintf("projects/%s", cleanProjectName)

						if jsFile != "" {
							htmlString = strings.Replace(htmlString,
								fmt.Sprintf("assets/%s", jsFile),
								fmt.Sprintf("%s/dist/assets/%s", projectPath, jsFile),
								-1)

						}

						if cssFile != "" {
							htmlString = strings.Replace(htmlString,
								fmt.Sprintf("assets/%s", cssFile),
								fmt.Sprintf("%s/dist/assets/%s", projectPath, cssFile),
								-1)

						}

						htmlString = strings.Replace(htmlString,
							`type="module" src="/src/`,
							fmt.Sprintf(`type="module" src="%s/dist/assets/`, projectPath),
							-1)

					}

					// Log the modified HTML for debugging
					fmt.Println("Modified HTML:", htmlString)

					// Serve the fixed HTML
					c.Data(http.StatusOK, "text/html", []byte(htmlString))
					return
				} else {
					fmt.Println("Error reading index.html:", err)
					c.File(indexPath)
					return
				}
			}
		}

		c.String(http.StatusNotFound, "Project not found")
	})
	r.NoRoute(func(c *gin.Context) {
		c.String(http.StatusNotFound, "404 Not Found: %s", c.Request.URL.Path)
	})

	r.Run(":8000")
}

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
