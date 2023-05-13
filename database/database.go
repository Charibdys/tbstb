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

type Connection struct {
	Client *mongo.Client
}

type Role struct {
	ID       int64  `bson:"_id"`
	Name     string `bson:"name"`
	Onymity  string `bson:"onymity"`
	RoleType string `bson:"role"`
}
type Config struct {
	Onymity    string `bson:"defaultOnymity"`
	UserReopen bool   `bson:"defaultUserReopen"`
	RelayMedia bool   `bson:"relayMedia"`
}

type User struct {
	ID                 int64 `bson:"_id"`
	Onymity            bool  `bson:"onymity"`
	DisabledBroadcasts bool  `bson:"disabledBroadcasts"`
	CanReopen          bool  `bson:"canReopen"`
	Banned             bool  `bson:"banned"`
}

type Ticket struct {
	Creator     int64     `bson:"creator"`
	Title       string    `bson:"title"`
	DateCreated time.Time `bson:"dateCreated"`
	Assignees   []int64   `bson:"assignees"`
	Messages    []Message `bson:"messages"`
	ClosedBy    int64     `bson:"closedBy"`
	DateClosed  time.Time `bson:"dateClosed"`
}
type Message struct {
	Sender   int64     `bson:"sender"`
	DateSent time.Time `bson:"dateSent"`
	Text     string    `bson:"text"`
	Media    string    `bson:"media"`
}

func Init() *Connection {
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

	db := Connection{
		Client: client,
	}

	return &db
}

// List the names of databases contained in the MongoDB database
func (db *Connection) ListDatabases() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	databases, err := db.Client.ListDatabaseNames(ctx, bson.M{})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(databases)
}

// CRUD operations

func (db *Connection) CreateConfig() {
	configColl := db.Client.Database("tbstb").Collection("config")

	config := Config{
		Onymity:    "realname",
		UserReopen: false,
		RelayMedia: true,
	}

	_, err := configColl.InsertOne(context.Background(), config)
	if err != nil {
		log.Fatal(err)
	}
}

func (db *Connection) CreateUser(id int64, config *Config) {
	userColl := db.Client.Database("tbstb").Collection("users")

	user := User{
		ID:                 id,
		Onymity:            false,
		DisabledBroadcasts: false,
		CanReopen:          config.UserReopen,
		Banned:             false,
	}

	_, err := userColl.InsertOne(context.Background(), user)
	if err != nil {
		log.Fatal(err)
	}
}

func (db *Connection) CreateRole(id int64, name string, roleType string, config *Config) {
	roleColl := db.Client.Database("tbstb").Collection("roles")

	if config.Onymity != "realname" {
		name = ""
	}

	role := Role{
		ID:       id,
		Name:     name,
		Onymity:  config.Onymity,
		RoleType: roleType,
	}

	_, err := roleColl.InsertOne(context.Background(), role)
	if err != nil {
		log.Fatal(err)
	}
}

func (db *Connection) GetConfig() (*Config, error) {
	configColl := db.Client.Database("tbstb").Collection("config")

	var config Config
	err := configColl.FindOne(context.Background(), bson.D{}).Decode(&config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func (db *Connection) GetUser(id int64) (*User, error) {
	userColl := db.Client.Database("tbstb").Collection("users")

	var user User
	err := userColl.FindOne(context.Background(), bson.D{{Key: "_id", Value: id}}).Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (db *Connection) GetRole(id int64) (*Role, error) {
	roleColl := db.Client.Database("tbstb").Collection("roles")

	var role Role
	err := roleColl.FindOne(context.Background(), bson.D{{Key: "_id", Value: id}}).Decode(&role)
	if err != nil {
		return nil, err
	}

	return &role, nil
}

func (db *Connection) UpdateConfig(config *Config) *Config {
	configColl := db.Client.Database("tbstb").Collection("config")

	var updatedConfig Config
	err := configColl.FindOneAndUpdate(
		context.Background(),
		bson.D{},
		bson.D{{
			Key: "$set",
			Value: bson.D{
				{Key: "defaultOnymity", Value: config.Onymity},
				{Key: "defaultUserReopen", Value: config.UserReopen},
				{Key: "relayMedia", Value: config.RelayMedia},
			},
		}},
	).Decode(&updatedConfig)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil
		}
		log.Fatal(err)
	}

	return &updatedConfig
}

func (db *Connection) UpdateUser(user *User) {
	userColl := db.Client.Database("tbstb").Collection("users")

	_, err := userColl.UpdateOne(
		context.Background(),
		bson.D{{Key: "_id", Value: user.ID}},
		bson.D{{
			Key: "$set",
			Value: bson.D{
				{Key: "banned", Value: user.Banned},
				{Key: "onymity", Value: user.Onymity},
				{Key: "disabledBroadcasts", Value: user.DisabledBroadcasts},
				{Key: "canReopen", Value: user.CanReopen},
			},
		}},
	)
	if err != nil {
		log.Fatal(err)
	}
}

func (db *Connection) UpdateRole(role *Role) {
	roleColl := db.Client.Database("tbstb").Collection("roles")

	_, err := roleColl.UpdateOne(
		context.Background(),
		bson.D{{Key: "_id", Value: role.ID}},
		bson.D{{
			Key: "$set",
			Value: bson.D{
				{Key: "name", Value: role.Name},
				{Key: "onymity", Value: role.Onymity},
				{Key: "role", Value: role.RoleType},
			},
		}},
	)
	if err != nil {
		log.Fatal(err)
	}
}

func (db *Connection) DeleteRole(id int64) {
	roleColl := db.Client.Database("tbstb").Collection("roles")

	_, err := roleColl.DeleteOne(context.Background(), bson.D{{Key: "_id", Value: id}})
	if err != nil {
		log.Fatal(err)
	}
}

func (db *Connection) DeleteUser(id int64) {
	userColl := db.Client.Database("tbstb").Collection("users")

	_, err := userColl.DeleteOne(context.Background(), bson.D{{Key: "_id", Value: id}})
	if err != nil {
		log.Fatal(err)
	}
}

func (db *Connection) GetUserCount() int64 {
	userColl := db.Client.Database("tbstb").Collection("users")

	opts := options.Count().SetHint("_id_")

	count, err := userColl.CountDocuments(context.Background(), bson.D{}, opts)
	if err != nil {
		log.Fatal(err)
	}

	return count
}

func (db *Connection) HandleConfigError() *Config {
	configColl := db.Client.Database("tbstb").Collection("config")

	opts := options.Count().SetHint("_id_")

	count, err := configColl.CountDocuments(context.Background(), bson.D{}, opts)
	if err != nil {
		log.Fatal(err)
	}

	if count > 1 {
		configColl.Drop(context.Background())
		db.CreateConfig()
		config, _ := db.GetConfig()
		return config
	} else {
		db.CreateConfig()
		config, _ := db.GetConfig()
		return config
	}
}

// Check if the required collections exist in the database.
// If all or only some collections do not exit, create them.
// Otherwise, ensure that the collections have the latest validation schema.
func (db *Connection) CheckCollections() {
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
func (db *Connection) ValidateSchema(create bool, database *mongo.Database) {
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
