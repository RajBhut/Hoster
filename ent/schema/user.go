package schema

import (
	"regexp"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type User struct {
	ent.Schema
}

// func validate_emial(s string) error {
// 	at := strings.Count(s, "@")
// 	dot := strings.Count(s, ".")
// 	if at != 1 && dot != 1 {
// 		return errors.New("invalid email")
// 	}
// 	return nil
// }

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("username").Unique().NotEmpty().MaxLen(50),
		field.String("email").Unique().Match(regexp.MustCompile("^[a-zA-Z0-9+_.-]+@[a-zA-Z0-9.-]+$")),
		field.String("password").MinLen(6).MaxLen(10)	,
	}
}

func (User) Edges() []ent.Edge {
	return nil
}
