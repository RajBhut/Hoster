package main

// import (
// 	"fmt"
// 	"net/http"

// 	"github.com/gin-gonic/gin"
// )

// var players = []player{}
// var client = initiate_db()

// func hellowhandler(w http.ResponseWriter, r *http.Request) {
// 	fmt.Fprintln(w, "Hellow from go")
// }

// type User struct {
// 	Name  string `json:"name"`
// 	Email string `json:"email"`
// }

// func mathod_check(w http.ResponseWriter, r *http.Request, mathod string) bool {
// 	if r.Method != mathod {
// 		http.Error(w, "Invalid mathod", http.StatusBadRequest)
// 		return false
// 	}
// 	return true
// }

// // func createUserHandler(w http.ResponseWriter, r *http.Request) {
// // 	if r.Method != http.MethodPost {
// // 		http.Error(w, "Only post allow", http.StatusMethodNotAllowed)
// // 		return
// // 	}
// // 	var user User
// // 	err := json.NewDecoder(r.Body).Decode(&user)
// // 	if err != nil {
// // 		http.Error(w, "invalid Json", http.StatusBadRequest)
// // 		return
// // 	}
// // 	fmt.Printf("recived user: %+v\n", user)
// // 	w.Header().Set("Content-Type", "application/json")
// // 	json.NewEncoder(w).Encode(map[string]string{
// // 		"message": "User created", "name": user.Name,
// // 	})
// // }

// func createUserHandler(c *gin.Context) {
// 	var user User
// 	err := c.BindJSON(&user)
// 	if err != nil {
// 		c.String(http.StatusBadRequest, "invalid object")
// 	}
// 	fmt.Printf("recived user: %+v\n", user)
// 	c.JSON(http.StatusAccepted, gin.H{
// 		"message": "User created successfully", "name": user.Name,
// 	})
// }

// type player struct {
// 	Username string `json:"username"`
// 	Password string `json:"password"`
// }

// // func getPlayers(w http.ResponseWriter, r *http.Request) {

// //		json.NewEncoder(w).Encode(players)
// //	}
// //
// //	func createRegister(w http.ResponseWriter, r *http.Request) {
// //		if r.Method != http.MethodPost {
// //			http.Error(w, "Mathod not allow", http.StatusBadRequest)
// //			return
// //		}
// //		var p player
// //		err := json.NewDecoder(r.Body).Decode(&p)
// //		if err != nil {
// //			http.Error(w, "Invalid Json", http.StatusBadRequest)
// //			return
// //		}
// //		fmt.Print(p)
// //		if p.Password == "" || p.Username == "" {
// //			http.Error(w, "all fields are neccasary", http.StatusBadRequest)
// //			return
// //		}
// //		players = append(players, player{
// //			p.Username, p.Password,
// //		})
// //		fmt.Println("player saved succesfulyy!!! ", p)
// //		w.Header().Set("Content-type", "application/json")
// //		json.NewEncoder(w).Encode(map[string]string{
// //			"message":  "player saved succesfully",
// //			"username": p.Username,
// //		})
// func getPlayers(c *gin.Context) {
// 	c.JSON(http.StatusAccepted, players)
// }
// func createRegister(c *gin.Context) {
// 	var p player

// 	err := c.BindJSON(&p)

// 	if err != nil {
// 		c.String(http.StatusBadRequest, "Invalid json")
// 		return
// 	}
// 	if p.Username == "" || p.Password == "" {
// 		c.String(400, "fill baoth fields")
// 		return
// 	}
// 	players = append(players, p)
// 	c.JSON(http.StatusAccepted, gin.H{
// 		"message": "player saved successfully",
// 	})

// }

// func login(c *gin.Context) {
// 	var p player
// 	err := c.BindJSON(&p)
// 	if err != nil {
// 		c.String(http.StatusBadRequest, "Invalid json")
// 		return
// 	}
// 	if p.Username == "" || p.Password == "" {
// 		c.String(400, "fill baoth fields")
// 		return
// 	}
// 	for _, value := range players {
// 		if value.Username == p.Username && value.Password == p.Password {
// 			c.JSON(http.StatusAccepted, gin.H{
// 				"message":  "login successfully",
// 				"username": p.Username,
// 			})
// 			return
// 		}
// 	}
// 	c.String(http.StatusUnauthorized, "invalid cradantial")
// 	return

// }

// // }
// // func login(w http.ResponseWriter, r *http.Request) {
// // 	if !mathod_check(w, r, http.MethodPost) {
// // 		return
// // 	}
// // 	var p player
// // 	error := json.NewDecoder(r.Body).Decode(&p)
// // 	if error != nil {
// // 		http.Error(w, "invalid cradantial", http.StatusBadRequest)
// // 		return
// // 	}
// // 	itr := -1
// // 	for val := range players {
// // 		if players[val].Username == p.Username {
// // 			itr = val
// // 			break
// // 		}

// // 	}
// // 	if itr != -1 && players[itr].Password == p.Password {
// // 		w.Header().Set("Content-Type", "application/json")
// // 		json.NewEncoder(w).Encode(map[string]string{
// // 			"message":  "login succesfull",
// // 			"username": p.Username,
// // 		})
// // 	} else {
// // 		http.Error(w, "invalid cradantial", http.StatusUnauthorized)
// // 	}

// // }

// func main() {
// 	r := gin.Default()

// 	r.POST("/", createUserHandler)
// 	r.POST("/reg", createRegister)
// 	r.GET("/users", getPlayers)
// 	r.POST("/login", login)
// 	r.GET("/byid", get_by_name)
// 	r.Run(":8000")

// 	// http.HandleFunc("/", createUserHandler)
// 	// http.HandleFunc("/reg", createRegister)
// 	// http.HandleFunc("/users", getPlayers)
// 	// http.HandleFunc("/login", login)
// 	// http.HandleFunc("/byid", get_by_name)
// 	// fmt.Println("server started at port 8000")
// 	// http.ListenAndServe(":8000", nil)
// }
// package main

// import (
// 	"context"
// 	"log"
// 	"net/http"
// 	"os"

// 	"github.com/gin-gonic/gin"
// 	"github.com/google/go-github/v50/github"
// 	"golang.org/x/oauth2"
// 	ghoauth "golang.org/x/oauth2/github"
// )

// var (
// 	githubOauthConfig = &oauth2.Config{
// 		ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
// 		ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
// 		Scopes:       []string{"repo"},
// 		Endpoint:     ghoauth.Endpoint,
// 	}
// 	state = "random" // you can use crypto/rand for real state
// )

// func main() {
// 	r := gin.Default()

// 	r.GET("/github/login", func(c *gin.Context) {
// 		url := githubOauthConfig.AuthCodeURL(state)
// 		c.Redirect(http.StatusFound, url)
// 	})

// 	r.GET("/github/callback", func(c *gin.Context) {
// 		code := c.Query("code")
// 		gotState := c.Query("state")

// 		if gotState != state {
// 			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid state"})
// 			return
// 		}

// 		token, err := githubOauthConfig.Exchange(context.Background(), code)
// 		if err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": "token exchange failed"})
// 			return
// 		}

// 		client := github.NewClient(githubOauthConfig.Client(context.Background(), token))

// 		// List user's repositories
// 		repos, _, err := client.Repositories.List(context.Background(), "", nil)
// 		if err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": "fetch repos failed"})
// 			return
// 		}

// 		names := []string{}
// 		for _, repo := range repos {
// 			names = append(names, repo.GetFullName())
// 		}

// 		c.JSON(http.StatusOK, gin.H{"repos": names})
// 	})

// 	r.Run(":3000")
// }
