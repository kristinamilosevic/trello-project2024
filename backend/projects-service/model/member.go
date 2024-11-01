package model

import "go.mongodb.org/mongo-driver/bson/primitive"

type Member struct {
	ID   primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name string             `bson:"name" json:"name"`
	Role string             `bson:"role" json:"role"`
}
