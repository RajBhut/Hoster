package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

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
	// repos, _, err := ghClient.Repositories.List(context.Background(), "", nil)
	// if err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch repos"})
	// 	return
	// }

	// repoNames := []string{}
	// for _, r := range repos {
	// 	repoNames = append(repoNames, r.GetFullName())
	// }

	// accessToken, _ := GenerateJWT("github_user")
	// refreshToken, _ := GenerateRefreshToken("github_user")

	// c.SetCookie("refresh_token", refreshToken, 60*60*24*30, "/", "localhost", true, true)

	// c.JSON(http.StatusOK, gin.H{
	// 	"message":      "Login successful",
	// 	"access_token": accessToken,
	// 	"repos":        repoNames,
	// })
}

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
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

// JWT logic
func GenerateJWT(userID string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
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

func Handlerefrer() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		refer := ctx.Request.Referer()
		fmt.Println("this is refer : ==================> ", refer)
		currentPath := ctx.Request.URL.String()
		fmt.Println("current path +>>>>>>>>>>>>>>>>>>>>>", currentPath)
		matched, err := regexp.MatchString(`^http://localhost:8000/projects/[^/]+$`, refer)
		if err != nil {
			fmt.Println("regex error ", err)
			ctx.Next()
			return
		}
		tem := strings.HasPrefix(currentPath, "/assets")
		if matched && tem {

			println("matched ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++")
			ctx.Redirect(http.StatusFound, refer+currentPath)
			ctx.Abort()

			return
		}
		// if regexp.MustCompile(`^http://localhost:8000/projects/[^/]+$`).MatchString(refer) {
		// 	fmt.Println("it is for redirection !!!!!!")

		// 	ctx.Redirect(http.StatusOK, tem)
		// }
		ctx.Next()
	}
}

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

// Clones and deploys a repository
func cloneAndDeployRepo(owner, repoName, accessToken string) (string, error) {
	// Create a unique deployment ID
	deploymentID := fmt.Sprintf("%s-%d", repoName, time.Now().Unix())

	// Create directories
	baseDir := filepath.Join("deployments", deploymentID)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create deployment directory: %v", err)
	}

	// Clone the repository using Git CLI
	repoURL := fmt.Sprintf("https://%s@github.com/%s/%s.git", accessToken, owner, repoName)
	cloneCmd := exec.Command("git", "clone", repoURL, baseDir)

	if err := cloneCmd.Run(); err != nil {
		return "", fmt.Errorf("failed to clone repository: %v", err)
	}

	fmt.Print("base dir:", baseDir)
	deployURL, err := deployBasedOnType(baseDir, deploymentID)
	if err != nil {
		return "", fmt.Errorf("deployment failed: %v", err)
	}

	return deployURL, nil
}

// Identifies repository type and runs appropriate deployment
func deployBasedOnType(repoDir, deploymentID string) (string, error) {
	// First check at the root level
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

func deployNodeApp(repoDir, deploymentID string) (string, error) {
	// Install dependencies
	installCmd := exec.Command("npm", "install")
	installCmd.Dir = repoDir
	fmt.Println("Installing npm dependencies...")
	if err := installCmd.Run(); err != nil {
		return "", fmt.Errorf("npm install failed: %v", err)
	}

	// Build the application
	buildCmd := exec.Command("npm", "run", "build")
	buildCmd.Dir = repoDir
	fmt.Println("Building project...")
	buildOutput, _ := buildCmd.CombinedOutput()
	fmt.Println("Build output:", string(buildOutput))

	// Even if build fails, we'll try to find a deployable directory
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

	// If no build directory was found, we'll treat the repo root as the deployable directory
	if buildDir == "" {
		fmt.Println("No build directory found, using repository root")
		// Check if there's an index.html file at the root level
		indexPath := filepath.Join(repoDir, "index.html")
		if _, err := os.Stat(indexPath); err != nil {
			fmt.Println("Warning: No index.html found at root level")
		}
	}

	// Return the URL where the project will be accessible
	return fmt.Sprintf("http://localhost:8000/projects/%s", deploymentID), nil
}

// 	port := 3000 + (time.Now().Unix() % 1000)
// 	buildCmd := exec.Command("npm", "run", "build")
// 	buildCmd.Dir = repoDir
// 	if err := buildCmd.Run(); err != nil {

// 		startCmd := exec.Command("npm", "start")
// 		startCmd.Dir = repoDir
// 		startCmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port))
// 		if err := startCmd.Start(); err != nil {
// 			return "", err
// 		}
// 	} else {
// 		// Serve the built files
// 		serveCmd := exec.Command("npx", "serve", "-s", "build", "-l", fmt.Sprintf("%d", port))
// 		serveCmd.Dir = repoDir
// 		if err := serveCmd.Start(); err != nil {
// 			return "", err
// 		}
// 	}

// 	return fmt.Sprintf("http://localhost:%d", port), nil
// }

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

func deployStaticSite(repoDir, deploymentID string) (string, error) {
	port := 9000 + (time.Now().Unix() % 1000)
	serveCmd := exec.Command("npx", "serve", "-l", fmt.Sprintf("%d", port))
	serveCmd.Dir = repoDir
	if err := serveCmd.Start(); err != nil {
		return "", err
	}

	return fmt.Sprintf("http://localhost:%d", port), nil
}

// Add this near your other handlers

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
