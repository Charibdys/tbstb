package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type connection struct {
	Client *mongo.Client
}

func Init() *connection {
	client, err := mongo.NewClient(options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}

	db := connection{
		Client: client,
	}

	return &db
}

// List the names of databases contained in the MongoDB database
func (db *connection) ListDatabases() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	databases, err := db.Client.ListDatabaseNames(ctx, bson.M{})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(databases)
}

// Check if the required collections exist in the database.
// If all or only some collections do not exit, create them.
// Otherwise, ensure that the collections have the latest validation schema.
func (db *connection) CheckCollections() {
	TBSTBDatabase := db.Client.Database("tbstb")

	currentCollections, listCollErr := TBSTBDatabase.ListCollectionNames(context.Background(), bson.D{})
	if listCollErr != nil {
		log.Fatal(listCollErr)
	}

	check := 0

	for _, collName := range currentCollections {
		switch collName {
		case "roles", "config", "tickets", "users":
			check++
		}
	}

	if check == 4 {
		db.ValidateSchema(false, TBSTBDatabase)
	} else {
		db.ValidateSchema(true, TBSTBDatabase)
	}
}

// Creates the required collections if they do not exist
// Otheriwse, updates the validation schema on the existing collections
func (db *connection) ValidateSchema(create bool, database *mongo.Database) {
	rolesSchema := bson.M{
		"bsonType": "object",
		"title":    "Role Object Validation",
		"required": []string{"_id", "onymity", "role"},
		"properties": bson.M{
			"_id": bson.M{
				"bsonType":    "long",
				"description": "ID of the user whom this role applies to",
			},
			"name": bson.M{
				"bsonType":    "string",
				"description": "Name of the user to whom this role applies to",
			},
			"onymity": bson.M{
				"bsonType":    "string",
				"description": "The onymity of the user, either \"anon\", \"pseudonym\", or \"realname\"",
			},
			"role": bson.M{
				"bsonType":    "string",
				"description": "The name of the role",
			},
		},
	}

	configSchema := bson.M{
		"bsonType": "object",
		"title":    "Config Object Validation",
		"required": []string{"defaultOnymity", "defaultUserReopen", "relayMedia"},
		"properties": bson.M{
			"defaultOnymity": bson.M{
				"bsonType":    "string",
				"description": "Default onymity for roles, either \"anon\", \"pseudonym\", or \"realname\"",
			},
			"defaultUserReopen": bson.M{
				"bsonType":    "bool",
				"description": "Toggle whether or not users can reopen issues themselves",
			},
			"relayMedia": bson.M{
				"bsonType":    "bool",
				"description": "Toggle whether or not to relay media (photos, videos, etc)",
			},
		},
	}

	ticketSchema := bson.M{
		"bsonType": "object",
		"title":    "Ticket Object Validation",
		"required": []string{"creator", "title", "dateCreated"},
		"properties": bson.M{
			"creator": bson.M{
				"bsonType":    "long",
				"description": "The user who created this issue",
			},
			"title": bson.M{
				"bsonType":    "string",
				"description": "A brief description of this ticket",
				"maxLength":   100,
			},
			"dateCreated": bson.M{
				"bsonType":    "date",
				"description": "The date when this ticket was created",
			},
			"assignees": bson.M{
				"bsonType":    "array",
				"description": "An array of users with roles assigned to this ticket",
				"items": bson.M{
					"bsonType": "long",
				},
			},
			"messages": bson.M{
				"bsonType":    "array",
				"description": "An array of messages associated with this ticket",
				"items": bson.M{
					"bsonType": "object",
					"required": []string{"sender", "dateSent"},
					"properties": bson.M{
						"sender": bson.M{
							"bsonType":    "long",
							"description": "The ID of the user who sent this message",
						},
						"dateSent": bson.M{
							"bsonType":    "date",
							"description": "The date when this message was sent",
						},
						"text": bson.M{
							"bsonType":    "string",
							"description": "The text or caption associated with this message",
						},
						"media": bson.M{
							"bsonType":    "string",
							"description": "A file_id associated with the media in this message",
						},
					},
				},
			},
			"closedBy": bson.M{
				"bsonType":    "long",
				"description": "The user who closed this ticket",
			},
			"dateClosed": bson.M{
				"bsonType":    "date",
				"description": "The date when this ticket was closed",
			},
		},
	}

	userSchema := bson.M{
		"bsonType": "object",
		"title":    "User Object Validation",
		"required": []string{"_id"},
		"properties": bson.M{
			"_id": bson.M{
				"bsonType":    "long",
				"description": "A user that interacts with the bot",
			},
			"onymity": bson.M{
				"bsonType":    "bool",
				"description": "Whether or not the user is anonymous",
			},
			"disabledBroadcasts": bson.M{
				"bsonType":    "bool",
				"description": "Whether the user has disabled receiving broadcasts",
			},
			"canReopen": bson.M{
				"bsonType":    "bool",
				"description": "Whether the user can reopen tickets",
			},
			"banned": bson.M{
				"bsonType":    "bool",
				"description": "Whether the user is banned and cannot interact with the bot",
			},
		},
	}

	if !create {
		database.RunCommand(
			context.Background(),
			bson.D{
				{Key: "collMod", Value: "roles"},
				{Key: "validator", Value: bson.M{"$jsonSchema": rolesSchema}},
				{Key: "validationLevel", Value: "moderate"},
				{Key: "validationAction", Value: "warn"},
			},
		)
		database.RunCommand(
			context.Background(),
			bson.D{
				{Key: "collMod", Value: "config"},
				{Key: "validator", Value: bson.M{"$jsonSchema": configSchema}},
				{Key: "validationLevel", Value: "moderate"},
				{Key: "validationAction", Value: "warn"},
			},
		)
		database.RunCommand(
			context.Background(),
			bson.D{
				{Key: "collMod", Value: "tickets"},
				{Key: "validator", Value: bson.M{"$jsonSchema": ticketSchema}},
				{Key: "validationLevel", Value: "moderate"},
				{Key: "validationAction", Value: "warn"},
			},
		)
		database.RunCommand(
			context.Background(),
			bson.D{
				{Key: "collMod", Value: "users"},
				{Key: "validator", Value: bson.M{"$jsonSchema": userSchema}},
				{Key: "validationLevel", Value: "moderate"},
				{Key: "validationAction", Value: "warn"},
			},
		)
	} else {
		rolesOpts := options.CreateCollection().SetValidator(bson.M{"$jsonSchema": rolesSchema})
		configOpts := options.CreateCollection().SetValidator(bson.M{"$jsonSchema": configSchema})
		ticketOpts := options.CreateCollection().SetValidator(bson.M{"$jsonSchema": ticketSchema})
		userOpts := options.CreateCollection().SetValidator(bson.M{"$jsonSchema": userSchema})

		rolesOpts.SetValidationLevel("moderate")
		configOpts.SetValidationLevel("moderate")
		ticketOpts.SetValidationLevel("moderate")
		userOpts.SetValidationLevel("moderate")

		rolesOpts.SetValidationAction("warn")
		configOpts.SetValidationAction("warn")
		ticketOpts.SetValidationAction("warn")
		userOpts.SetValidationAction("warn")

		createRolesErr := database.CreateCollection(context.Background(), "roles", rolesOpts)
		if createRolesErr != nil {
			log.Println(createRolesErr)
		}
		createConfigErr := database.CreateCollection(context.Background(), "config", configOpts)
		if createConfigErr != nil {
			log.Println(createConfigErr)
		}
		createTicketsErr := database.CreateCollection(context.Background(), "tickets", ticketOpts)
		if createTicketsErr != nil {
			log.Println(createTicketsErr)
		}
		createUsersErr := database.CreateCollection(context.Background(), "users", userOpts)
		if createUsersErr != nil {
			log.Println(createUsersErr)
		}
	}
}
