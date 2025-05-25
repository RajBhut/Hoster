package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

// GitHub OAuth handlers
func githubLogin(c *gin.Context) {
	url := githubOauthConfig.AuthCodeURL(state)
	c.Redirect(http.StatusFound, url)
}

func githubCallback(c *gin.Context) {
	code := c.Query("code")
	gotState := c.Query("state")

	if gotState != state {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid state"})
		return
	}

	token, err := githubOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token exchange failed"})
		return
	}

	ghClient := github.NewClient(githubOauthConfig.Client(context.Background(), token))
	user, _, err := ghClient.Users.Get(context.Background(), "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fetch user failed"})
		return
	}

	c.SetCookie("access_token", token.AccessToken, 3600*24, "/", "localhost", false, true)
	c.SetCookie("github_user", user.GetLogin(), 3600*24, "/", "localhost", false, false)

	c.Redirect(http.StatusFound, "http://localhost:5173/dashboard")
}

// JWT handlers and utilities
func refreshHandler(c *gin.Context) {
	rt, err := c.Cookie("refresh_token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no refresh token"})
		return
	}
	userID, err := ParseToken(rt)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}
	newAccess, _ := GenerateJWT(userID)
	c.JSON(http.StatusOK, gin.H{"access_token": newAccess})
}

func logoutHandler(c *gin.Context) {
	c.SetCookie("refresh_token", "", -1, "/", "localhost", true, true)
	c.SetCookie("access_token", "", -1, "/", "localhost", false, true)
	c.SetCookie("github_user", "", -1, "/", "localhost", false, false)
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

func userinfo(c *gin.Context) {
	tokenCookie, err := c.Cookie("access_token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	token := &oauth2.Token{AccessToken: tokenCookie}
	client := github.NewClient(githubOauthConfig.Client(context.Background(), token))

	user, _, err := client.Users.Get(context.Background(), "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch user info"})
		return
	}

	repos, _, err := client.Repositories.List(context.Background(), "", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch repos"})
		return
	}

	var repoNames []string
	for _, r := range repos {
		repoNames = append(repoNames, r.GetName())
	}

	c.JSON(http.StatusOK, gin.H{
		"username": user.GetLogin(),
		"avatar":   user.GetAvatarURL(),
		"repos":    repoNames,
	})
}

// JWT utilities
func GenerateJWT(userID string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

func GenerateRefreshToken(userID string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(30 * 24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

func ParseToken(tokenStr string) (string, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})
	if err != nil || !token.Valid {
		return "", err
	}
	claims := token.Claims.(jwt.MapClaims)
	return claims["user_id"].(string), nil
}

// Authentication middleware
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := c.GetHeader("Authorization")
		if len(tokenStr) < 7 || tokenStr[:7] != "Bearer " {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing bearer token"})
			c.Abort()
			return
		}
		tokenStr = tokenStr[7:]
		_, err := ParseToken(tokenStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// Repository details handler
func getRepoDetailsHandler(c *gin.Context) {
	tokenCookie, err := c.Cookie("access_token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	repoName := c.Query("repo")
	if repoName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "repo parameter is required"})
		return
	}

	token := &oauth2.Token{AccessToken: tokenCookie}
	client := github.NewClient(githubOauthConfig.Client(context.Background(), token))

	user, _, err := client.Users.Get(context.Background(), "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch user info"})
		return
	}

	repo, _, err := client.Repositories.Get(context.Background(), user.GetLogin(), repoName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch repo details"})
		return
	}

	languages, _, err := client.Repositories.ListLanguages(context.Background(), user.GetLogin(), repoName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch repo languages"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":           repo.GetName(),
		"description":    repo.GetDescription(),
		"language":       repo.GetLanguage(),
		"languages":      languages,
		"stars":          repo.GetStargazersCount(),
		"forks":          repo.GetForksCount(),
		"default_branch": repo.GetDefaultBranch(),
		"created_at":     repo.GetCreatedAt(),
		"updated_at":     repo.GetUpdatedAt(),
	})
}

// Deployment handlers
func selectRepoHandler(c *gin.Context) {
	tokenCookie, err := c.Cookie("access_token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var requestBody struct {
		RepoName string `json:"repo_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Setup GitHub client with user token
	token := &oauth2.Token{AccessToken: tokenCookie}
	client := github.NewClient(githubOauthConfig.Client(context.Background(), token))

	// Get user info to determine repo owner
	user, _, err := client.Users.Get(context.Background(), "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch user info"})
		return
	}

	// Clone and deploy
	deploymentURL, err := cloneAndDeployRepo(user.GetLogin(), requestBody.RepoName, tokenCookie)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Repository deployed successfully",
		"deploy_url": deploymentURL,
	})
}

// Clones and deploys a repository
func cloneAndDeployRepo(owner, repoName, accessToken string) (string, error) {
	// Create a unique deployment ID
	deploymentID := fmt.Sprintf("%s-%d", repoName, time.Now().Unix())

	// Create directories
	baseDir := filepath.Join(deploymentRootDir, deploymentID)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create deployment directory: %v", err)
	}

	// Clone the repository using Git CLI
	repoURL := fmt.Sprintf("https://%s@github.com/%s/%s.git", accessToken, owner, repoName)
	cloneCmd := exec.Command("git", "clone", repoURL, baseDir)
	fmt.Printf("Cloning repository: %s to %s\n", repoName, baseDir)

	if err := cloneCmd.Run(); err != nil {
		return "", fmt.Errorf("failed to clone repository: %v", err)
	}

	fmt.Printf("Repository cloned successfully to: %s\n", baseDir)

	deployURL, err := deployBasedOnType(baseDir, deploymentID)
	if err != nil {
		return "", fmt.Errorf("deployment failed: %v", err)
	}

	return deployURL, nil
}

// Identifies repository type and runs appropriate deployment
func deployBasedOnType(repoDir, deploymentID string) (string, error) {
	// First check at the root level and search for nested projects
	projectDir, projectType := findProjectRoot(repoDir)

	fmt.Println("Project directory found:", projectDir)
	fmt.Println("Project type:", projectType)

	switch projectType {
	case "node":
		return deployNodeApp(projectDir, deploymentID)
	case "go":
		return deployGoApp(projectDir, deploymentID)
	case "python":
		return deployPythonApp(projectDir, deploymentID)
	case "static":
		return deployStaticSite(projectDir, deploymentID)
	default:
		return "", fmt.Errorf("unsupported repository type")
	}
}

// findProjectRoot searches for project files in the repository
// and returns the project directory and type
func findProjectRoot(rootDir string) (string, string) {
	// First check the root directory
	if isNodeProject(rootDir) {
		return rootDir, "node"
	}
	if isGoProject(rootDir) {
		return rootDir, "go"
	}
	if isPythonProject(rootDir) {
		return rootDir, "python"
	}
	if isStaticSite(rootDir) {
		return rootDir, "static"
	}

	// If not found at root, search one level deeper
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return rootDir, ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subDir := filepath.Join(rootDir, entry.Name())

			// Skip common non-project directories
			if entry.Name() == "node_modules" || entry.Name() == ".git" ||
				entry.Name() == "venv" || entry.Name() == ".github" {
				continue
			}

			if isNodeProject(subDir) {
				return subDir, "node"
			}
			if isGoProject(subDir) {
				return subDir, "go"
			}
			if isPythonProject(subDir) {
				return subDir, "python"
			}
			if isStaticSite(subDir) {
				return subDir, "static"
			}
		}
	}

	// If still not found, return the original directory
	return rootDir, ""
}

// Helper functions to identify project types
func isNodeProject(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "package.json"))
	return err == nil
}

func isGoProject(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err == nil
}

func isPythonProject(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "requirements.txt"))
	return err == nil
}

func isStaticSite(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "index.html"))
	return err == nil
}

// Helper function to copy directories
func copyDirectory(source, target string) error {
	// Create the target directory if it doesn't exist
	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}

	// Read the directory
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		sourcePath := filepath.Join(source, entry.Name())
		targetPath := filepath.Join(target, entry.Name())

		if entry.IsDir() {
			// Skip node_modules to avoid large copies
			if entry.Name() == "node_modules" || entry.Name() == ".git" {
				continue
			}
			// Recursively copy subdirectories
			if err := copyDirectory(sourcePath, targetPath); err != nil {
				return err
			}
		} else {
			// Copy files
			content, err := os.ReadFile(sourcePath)
			if err != nil {
				return err
			}

			if err := os.WriteFile(targetPath, content, 0644); err != nil {
				return err
			}
		}
	}

	return nil
}

// Move project to permanent storage
func add_to_deployed_folder(sourceDir, projectName string) error {
	cleanProjectName := strings.Split(projectName, "-")[0]

	deployedDir := "Deployed"
	if err := os.MkdirAll(deployedDir, 0755); err != nil {
		return fmt.Errorf("failed to create Deployed directory: %v", err)
	}

	projectDir := filepath.Join(deployedDir, cleanProjectName)

	if _, err := os.Stat(projectDir); err == nil {
		if err := os.RemoveAll(projectDir); err != nil {
			return fmt.Errorf("failed to clean existing project directory: %v", err)
		}
	}

	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %v", err)
	}

	// Check common build directories
	buildDirs := []string{"dist", "build", "public", "out", "_site"}
	var buildDir string
	var sourceBuildDir string

	for _, dir := range buildDirs {
		path := filepath.Join(sourceDir, dir)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			buildDir = dir
			sourceBuildDir = path
			fmt.Printf("Found build directory: %s\n", buildDir)
			break
		}
	}

	if buildDir == "" {
		fmt.Println("No specific build directory found, copying entire source directory")
		if err := copyDirectory(sourceDir, projectDir); err != nil {
			return fmt.Errorf("failed to copy source directory: %v", err)
		}
	} else {
		// Create the build directory in the project directory
		targetDir := filepath.Join(projectDir, buildDir)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return fmt.Errorf("failed to create build directory: %v", err)
		}

		// Copy build files
		if err := copyDirectory(sourceBuildDir, targetDir); err != nil {
			return fmt.Errorf("failed to copy build directory: %v", err)
		}
	}

	fmt.Printf("Successfully copied from %s to %s\n", sourceDir, projectDir)

	// Remove the original deployment directory after a delay
	// to ensure ongoing requests can complete
	go func() {
		time.Sleep(10 * time.Second)
		fmt.Printf("Removing temporary deployment directory: %s\n", sourceDir)
		if err := os.RemoveAll(sourceDir); err != nil {
			fmt.Printf("Warning: Failed to clean up deployment directory: %v\n", err)
		} else {
			fmt.Printf("Successfully removed temporary deployment directory\n")
		}
	}()

	return nil
}

// Function to update package.json homepage for React/Vite projects
func updateHomepage(projectPath, projectName string) error {
	pkgFile := filepath.Join(projectPath, "package.json")
	data, err := os.ReadFile(pkgFile)
	if err != nil {
		return err
	}

	var pkg map[string]interface{}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return err
	}

	// Set homepage to match our serving path
	cleanProjectName := strings.Split(projectName, "-")[0]
	pkg["homepage"] = "/projects/" + cleanProjectName

	// Update Vite configuration if needed
	if deps, ok := pkg["dependencies"].(map[string]interface{}); ok {
		if _, hasVite := deps["vite"]; hasVite {
			// Create or update vite.config.js
			viteConfigPath := filepath.Join(projectPath, "vite.config.js")
			viteConfig := fmt.Sprintf(`
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  base: '/projects/%s/',
})
`, cleanProjectName)

			if err := os.WriteFile(viteConfigPath, []byte(viteConfig), 0644); err != nil {
				fmt.Printf("Warning: Failed to create Vite config: %v\n", err)
			} else {
				fmt.Println("Updated Vite configuration")
			}
		}
	}

	updated, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(pkgFile, updated, 0644)
}

// Deployment function for Node.js apps (React/Vite/Next.js)
func deployNodeApp(repoDir, deploymentID string) (string, error) {
	// Extract the project name from the deployment ID
	projectName := filepath.Base(repoDir)
	cleanProjectName := strings.Split(projectName, "-")[0]

	// Update package.json with homepage
	if err := updateHomepage(repoDir, deploymentID); err != nil {
		fmt.Printf("Warning: Could not update homepage in package.json: %v\n", err)
	} else {
		fmt.Println("Updated homepage in package.json")
	}

	// Install dependencies
	installCmd := exec.Command("npm", "install")
	installCmd.Dir = repoDir
	fmt.Println("Installing npm dependencies...")
	if err := installCmd.Run(); err != nil {
		return "", fmt.Errorf("npm install failed: %v", err)
	}

	// Build the project
	buildCmd := exec.Command("npm", "run", "build")
	buildCmd.Dir = repoDir
	fmt.Println("Building project...")
	buildOutput, _ := buildCmd.CombinedOutput()
	fmt.Println("Build output:", string(buildOutput))

	// Find build directory
	buildDirs := []string{"dist", "build", "public", "out", "_site"}
	var buildDir string
	for _, dir := range buildDirs {
		path := filepath.Join(repoDir, dir)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			buildDir = dir
			fmt.Println("Found build directory:", buildDir)
			break
		}
	}

	if buildDir == "" {
		fmt.Println("Warning: No build directory found")
	}

	// Move files to Deployed folder and clean up
	err := add_to_deployed_folder(repoDir, deploymentID)
	if err != nil {
		fmt.Printf("Warning: Failed to move to Deployed folder: %v\n", err)
	}

	// Return the URL where the project will be accessible
	return fmt.Sprintf("http://localhost:8000/projects/%s", cleanProjectName), nil
}

// Deployment function for Go apps
func deployGoApp(repoDir, deploymentID string) (string, error) {
	buildCmd := exec.Command("go", "build", "-o", deploymentID)
	buildCmd.Dir = repoDir
	if err := buildCmd.Run(); err != nil {
		return "", err
	}

	port := 8000 + (time.Now().Unix() % 1000)
	runCmd := exec.Command(filepath.Join(repoDir, deploymentID))
	runCmd.Dir = repoDir
	runCmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port))
	if err := runCmd.Start(); err != nil {
		return "", err
	}

	return fmt.Sprintf("http://localhost:%d", port), nil
}

// Deployment function for Python apps
func deployPythonApp(repoDir, deploymentID string) (string, error) {
	venvCmd := exec.Command("python", "-m", "venv", "venv")
	venvCmd.Dir = repoDir
	if err := venvCmd.Run(); err != nil {
		return "", err
	}

	var pipCmd *exec.Cmd
	if runtime.GOOS == "windows" {
		pipCmd = exec.Command("venv\\Scripts\\pip", "install", "-r", "requirements.txt")
	} else {
		pipCmd = exec.Command("venv/bin/pip", "install", "-r", "requirements.txt")
	}
	pipCmd.Dir = repoDir
	if err := pipCmd.Run(); err != nil {
		return "", err
	}

	// Run app (assuming a Flask or Django app)
	port := 5000 + (time.Now().Unix() % 1000)
	var runCmd *exec.Cmd
	if runtime.GOOS == "windows" {
		runCmd = exec.Command("venv\\Scripts\\python", "app.py")
	} else {
		runCmd = exec.Command("venv/bin/python", "app.py")
	}
	runCmd.Dir = repoDir
	runCmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port))
	if err := runCmd.Start(); err != nil {
		return "", err
	}

	return fmt.Sprintf("http://localhost:%d", port), nil
}

// Deployment function for static sites
func deployStaticSite(repoDir, deploymentID string) (string, error) {
	// Copy to Deployed folder
	cleanProjectName := strings.Split(filepath.Base(repoDir), "-")[0]
	projectDir := filepath.Join(deployedDir, cleanProjectName)

	if _, err := os.Stat(projectDir); err == nil {
		if err := os.RemoveAll(projectDir); err != nil {
			return "", fmt.Errorf("failed to clean existing deployment: %v", err)
		}
	}

	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create deployment directory: %v", err)
	}

	if err := copyDirectory(repoDir, projectDir); err != nil {
		return "", fmt.Errorf("failed to copy files: %v", err)
	}

	return fmt.Sprintf("http://localhost:8000/projects/%s", cleanProjectName), nil
}
