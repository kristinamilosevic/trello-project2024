package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Project struct {
	ID              primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Name            string             `json:"name" bson:"name"`
	ExpectedEndDate time.Time          `json:"expectedEndDate" bson:"expected_end_date"`
	MinMembers      int                `json:"minMembers" bson:"min_members"`
	MaxMembers      int                `json:"maxMembers" bson:"max_members"`
	ManagerID       primitive.ObjectID `json:"managerId" bson:"manager_id"`
}
