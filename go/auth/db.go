package main

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

type About struct {
	FullName string `bson:"full_name" json:"full_name"`
	Group    string `bson:"group" json:"group"`
}

type User struct {
	GithubId int64    `bson:"github_id" json:"github_id"`
	ChatId   int64    `bson:"chat_id" json:"chat_id"`
	Roles    []string `bson:"roles" json:"roles"` // student lecturer admin
	About    About    `bson:"about" json:"about"`
}

type UserCRUD interface {
	Create(User) (User, error)
	ReadByChatId(int64) (User, error)
	ReadByGithubId(int64) (User, error)
	Update(User) (User, error)
	Delete(int64) (User, error)
	ReadAll() ([]User, error)
	Closer
}

type Closer interface {
	Close(context.Context) error
}

type MongoColl struct {
	coll    *mongo.Collection
	timeout time.Duration
}

func (m MongoColl) Close(ctx context.Context) error {
	client := m.coll.Database().Client()
	if err := client.Disconnect(ctx); err != nil {
		return err
	}
	return nil
}

func newUserCRUD() (UserCRUD, error) {
	var timeout = 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	client, err := mongo.NewClient(options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		return nil, err
	}
	if err := client.Connect(ctx); err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}
	return &MongoColl{client.Database("UsersDB").Collection("user"), timeout}, nil
}

func (m MongoColl) Create(user User) (User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	_, err := m.coll.InsertOne(ctx, user)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (m MongoColl) ReadByChatId(chatId int64) (User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	filter := bson.D{{"chat_id", chatId}}
	var result User
	err := m.coll.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		return User{}, err
	}
	return result, nil
}

func (m MongoColl) ReadByGithubId(githubId int64) (User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	filter := bson.D{{"github_id", githubId}}
	var result User
	err := m.coll.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		return User{}, err
	}
	return result, nil
}

func (m MongoColl) Update(user User) (User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	filter := bson.D{{"chat_id", user.ChatId}}
	updateData := bson.D{{"$set", bson.D{
		{"github_id", user.GithubId},
		{"chat_id", user.ChatId},
		{"roles", user.Roles},
		{"about", user.About},
	}}}
	rg, err := m.coll.UpdateOne(ctx, filter, updateData)
	println(rg)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (m MongoColl) Delete(tgId int64) (User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	filter := bson.D{{"chat_id", tgId}}
	var deletedUser User
	err := m.coll.FindOneAndDelete(ctx, filter).Decode(&deletedUser)
	if err != nil {
		return User{}, err
	}
	return deletedUser, nil
}

func (m MongoColl) ReadAll() ([]User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	cursor, findErr := m.coll.Find(ctx, bson.D{})
	if findErr != nil {
		return []User{}, findErr
	}
	defer cursor.Close(ctx)
	var results []User
	if decodeErr := cursor.All(ctx, &results); decodeErr != nil {
		return []User{}, decodeErr
	}
	return results, nil
}
