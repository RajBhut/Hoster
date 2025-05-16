package main

import (
	"context"
	"log"

	"github.com/RajBhut/go-basics/ent"
	_ "github.com/lib/pq"
)

// func get_by_name(c *gin.Context) {
// 	user := c.Query("name")
// 	var dumy player
// 	for _, value := range players {
// 		if value.Username == user {
// 			dumy = value

// 			fmt.Println(dumy)
// 			c.JSON(200, dumy)
// 			break
// 		}

//		}
//		c.String(http.StatusUnauthorized, "Not valid user")
//	}
func initiate_db() (*ent.Client, context.Context) {
	client, err := ent.Open("postgres", "host=localhost port=5432 user=postgres password=root dbname=gotask sslmode=disable")
	if err != nil {
		log.Fatalf("failed opening connection to postgres: %v", err)
	}

	ctx := context.Background()

	if err := client.Schema.Create(ctx); err != nil {
		log.Fatalf("failed creating schema resources: %v", err)
	}
	return client, ctx

	// task, err := client.Task.
	// 	Create().
	// 	SetContent("Learn Ent").
	// 	SetUser("john").
	// 	Save(ctx)

	// if err != nil {
	// 	log.Fatalf("failed creating task: %v", err)
	// }

	// fmt.Println("Task created:", task)
}

//package main

// import (
//     "context"
//     "fmt"
//     "log"
//     "go-ent-app/ent"

//     _ "github.com/lib/pq"
// )

// func main() {
//     client, err := ent.Open("postgres", "host=localhost port=5432 user=postgres password=yourpass dbname=gotasks sslmode=disable")
//     if err != nil {
//         log.Fatalf("failed opening connection to postgres: %v", err)
//     }
//     defer client.Close()

//     ctx := context.Background()

//     // Run the auto migration tool
//     if err := client.Schema.Create(ctx); err != nil {
//         log.Fatalf("failed creating schema resources: %v", err)
//     }

//     // Create a task
//     task, err := client.Task.
//         Create().
//         SetContent("Learn Ent").
//         SetUser("john").
//         Save(ctx)

//     if err != nil {
//         log.Fatalf("failed creating task: %v", err)
//     }

//     fmt.Println("Task created:", task)
// }
