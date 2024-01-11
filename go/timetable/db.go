package main

import (
	"context"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

// Создание основного класса, аннотации bson и json
type Class struct {
	Title        string `bson:"title" json:"title"`
	Type         string `bson:"type" json:"type"`
	Teacher      string `bson:"teacher" json:"teacher"`
	LessonNumber string `bson:"lesson_number" json:"lesson_number"`
	Group        string `bson:"group" json:"group"`
	Date         string `bson:"date" json:"date"`
	Comment      string `bson:"comment" json:"comment"`
	Address      string `bson:"address" json:"address"`
}

// Создание интерфейса для закрытия монго базы
type Closer interface {
	Close(context.Context) error
}

// Создание структуры для MongoDB
type MongoColl struct {
	coll    *mongo.Collection
	timeout time.Duration
}

// Функция Close для закрытия Mongo базы
func (m MongoColl) Close(ctx context.Context) error {
	client := m.coll.Database().Client()
	if err := client.Disconnect(ctx); err != nil {
		return err
	}
	return nil
}

// Создание нового ClassCRUD
func newClassCRUD() (ClassCRUD, error) {
	var timeout = 3 * time.Second
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

	return &MongoColl{client.Database("ClassesDB").Collection("class"), timeout}, nil
}

// интерфейс ClassCRUD (основные функции для работы с монго)
type ClassCRUD interface {
	Create(Class) (Class, error)
	Update(Class) (Class, error)
	Delete(date string, lessonNumber string, group string) (Class, error)
	DeleteAll() ([]Class, error)
	ReadAll() ([]Class, error)
	ReadByDateAndLessonNumberAndGroup(date string, lessonNumber string, group string) (Class, error)
	ReadByLessonNumberAndGroupAndTeacher(lessonNumber string, group string, teacher string) ([]Class, error)
	ReadByLessonNumberAndGroup(lessonNumber string, group string) ([]Class, error)
	ReadByTeacher(teacher string) ([]Class, error)
	ReadByDate(date string) ([]Class, error)
	ReadByDateAndTeacher(date string, teacher string) ([]Class, error)
	ReadByDateAndGroup(date string, group string) ([]Class, error)
	ReadByGroupAndType(group string, classType string) ([]Class, error)
	ReadByGroupAndAfterDateInclusive(group string, date string) ([]Class, error)
	ReadByTeacherAndAfterDateInclusive(group string, date string) ([]Class, error)
	ReadByGroupAndDateAndLessonNumber(group string, date string, lessonNumber string) (Class, error)
	ReadByTeacherAndDateAndLessonNumber(teacher string, date string, lessonNumber string) (Class, error)
	Closer
}

func (m MongoColl) Create(class Class) (Class, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	_, err := m.coll.InsertOne(ctx, class)
	if err != nil {
		return Class{}, err
	}

	return class, nil
}

func (m MongoColl) Update(class Class) (Class, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	filter := bson.D{{"date", class.Date}, {"lesson_number", class.LessonNumber}, {"group", class.Group}}

	updateData := bson.D{{"$set", bson.D{
		{"comment", class.Comment},
	}}}

	_, err := m.coll.UpdateOne(ctx, filter, updateData)
	if err != nil {
		return Class{}, err
	}

	return class, nil
}

func (m MongoColl) Delete(date string, lessonNumber string, group string) (Class, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	filter := bson.D{{"date", date}, {"lesson_number", lessonNumber}, {"group", group}}

	var deletedClass Class
	err := m.coll.FindOneAndDelete(ctx, filter).Decode(&deletedClass)
	if err != nil {
		return Class{}, err
	}

	return deletedClass, nil
}

func (m MongoColl) DeleteAll() ([]Class, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	cursor, findErr := m.coll.Find(ctx, bson.D{})
	if findErr != nil {
		return []Class{}, findErr
	}
	defer cursor.Close(ctx)

	var results []Class
	if decodeErr := cursor.All(ctx, &results); decodeErr != nil {
		return []Class{}, decodeErr
	}

	_, err := m.coll.DeleteMany(ctx, bson.D{})
	if err != nil {
		return []Class{}, err
	}

	return results, nil
}

func (m MongoColl) ReadAll() ([]Class, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	cursor, findErr := m.coll.Find(ctx, bson.D{})
	if findErr != nil {
		return []Class{}, findErr
	}
	defer cursor.Close(ctx)

	var results []Class
	if decodeErr := cursor.All(ctx, &results); decodeErr != nil {
		return []Class{}, decodeErr
	}

	return results, nil
}

func (m MongoColl) ReadByDateAndLessonNumberAndGroup(date string, lessonNumber string, group string) (Class, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	filter := bson.D{{"date", date}, {"lesson_number", lessonNumber}, {"group", group}}
	var result Class
	err := m.coll.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		return Class{}, err
	}

	return result, nil
}

func (m MongoColl) ReadByLessonNumberAndGroupAndTeacher(lessonNumber string, group string, teacher string) ([]Class, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	filter := bson.D{{"teacher", teacher}, {"lesson_number", lessonNumber}, {"group", group}}
	cursor, findErr := m.coll.Find(ctx, filter)
	if findErr != nil {
		return []Class{}, findErr
	}
	defer cursor.Close(ctx)

	var results []Class
	if decodeErr := cursor.All(ctx, &results); decodeErr != nil {
		return []Class{}, decodeErr
	}

	return results, nil
}

func (m MongoColl) ReadByLessonNumberAndGroup(lessonNumber string, group string) ([]Class, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	filter := bson.D{{"lesson_number", lessonNumber}, {"group", group}}
	cursor, findErr := m.coll.Find(ctx, filter)
	if findErr != nil {
		return []Class{}, findErr
	}
	defer cursor.Close(ctx)

	var results []Class
	if decodeErr := cursor.All(ctx, &results); decodeErr != nil {
		return []Class{}, decodeErr
	}

	return results, nil
}

func (m MongoColl) ReadByTeacher(teacher string) ([]Class, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	filter := bson.D{{"teacher", teacher}}
	cursor, findErr := m.coll.Find(ctx, filter)
	if findErr != nil {
		return []Class{}, findErr
	}
	defer cursor.Close(ctx)

	var results []Class
	if decodeErr := cursor.All(ctx, &results); decodeErr != nil {
		return []Class{}, decodeErr
	}

	return results, nil
}

func (m MongoColl) ReadByDate(date string) ([]Class, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	filter := bson.D{{"date", date}}
	cursor, findErr := m.coll.Find(ctx, filter)
	if findErr != nil {
		return []Class{}, findErr
	}
	defer cursor.Close(ctx)

	var results []Class
	if decodeErr := cursor.All(ctx, &results); decodeErr != nil {
		return []Class{}, decodeErr
	}

	return results, nil
}

func (m MongoColl) ReadByDateAndTeacher(date string, teacher string) ([]Class, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	filter := bson.D{{"date", date}, {"teacher", teacher}}
	cursor, findErr := m.coll.Find(ctx, filter)
	if findErr != nil {
		return []Class{}, findErr
	}
	defer cursor.Close(ctx)

	var results []Class
	if decodeErr := cursor.All(ctx, &results); decodeErr != nil {
		return []Class{}, decodeErr
	}

	return results, nil
}

func (m MongoColl) ReadByDateAndGroup(date string, group string) ([]Class, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	filter := bson.D{{"date", date}, {"group", group}}
	cursor, findErr := m.coll.Find(ctx, filter)
	if findErr != nil {
		return []Class{}, findErr
	}
	defer cursor.Close(ctx)

	var results []Class
	if decodeErr := cursor.All(ctx, &results); decodeErr != nil {
		return []Class{}, decodeErr
	}

	return results, nil
}

func (m MongoColl) ReadByGroupAndType(group string, classType string) ([]Class, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	filter := bson.D{{"group", group}, {"type", classType}}
	cursor, findErr := m.coll.Find(ctx, filter)
	if findErr != nil {
		return []Class{}, findErr
	}
	defer cursor.Close(ctx)

	var results []Class
	if decodeErr := cursor.All(ctx, &results); decodeErr != nil {
		return []Class{}, decodeErr
	}

	return results, nil
}

func (m MongoColl) ReadByGroupAndAfterDateInclusive(group string, date string) ([]Class, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	filter := bson.D{
		{"group", group},
	}

	cursor, findErr := m.coll.Find(ctx, filter)
	if findErr != nil {
		return nil, findErr
	}
	defer cursor.Close(ctx)

	var allResults []Class
	if decodeErr := cursor.All(ctx, &allResults); decodeErr != nil {
		return nil, decodeErr
	}

	targetDate, parseErr := time.Parse("02.01.06", date)
	if parseErr != nil {
		return nil, parseErr
	}

	var filteredResults []Class
	for _, result := range allResults {
		classDate, parseErr := time.Parse("02.01.06", result.Date)
		if parseErr != nil {
			return nil, parseErr
		}
		if classDate.After(targetDate) || classDate.Equal(targetDate) {
			filteredResults = append(filteredResults, result)
		}
	}

	return filteredResults, nil
}

func (m MongoColl) ReadByTeacherAndAfterDateInclusive(teacher string, date string) ([]Class, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	filter := bson.D{
		{"teacher", teacher},
	}

	cursor, findErr := m.coll.Find(ctx, filter)
	if findErr != nil {
		return nil, findErr
	}
	defer cursor.Close(ctx)

	var allResults []Class
	if decodeErr := cursor.All(ctx, &allResults); decodeErr != nil {
		return nil, decodeErr
	}

	targetDate, parseErr := time.Parse("02.01.06", date)
	if parseErr != nil {
		return nil, parseErr
	}

	var filteredResults []Class
	for _, result := range allResults {
		classDate, parseErr := time.Parse("02.01.06", result.Date)
		if parseErr != nil {
			return nil, parseErr
		}
		if classDate.After(targetDate) || classDate.Equal(targetDate) {
			filteredResults = append(filteredResults, result)
		}
	}

	return filteredResults, nil
}

func (m MongoColl) ReadByGroupAndDateAndLessonNumber(group string, date string, lessonNumber string) (Class, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	filter := bson.D{
		{"group", group},
		{"date", date},
		{"lesson_number", lessonNumber},
	}

	var result Class
	err := m.coll.FindOne(ctx, filter).Decode(&result)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			// Обработка случая, когда ничего не найдено
			return Class{}, nil
		}
		return Class{}, err
	}
	return result, nil
}

func (m MongoColl) ReadByTeacherAndDateAndLessonNumber(teacher string, date string, lessonNumber string) (Class, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	filter := bson.D{
		{"teacher", teacher},
		{"date", date},
		{"lesson_number", lessonNumber},
	}

	var result Class
	err := m.coll.FindOne(ctx, filter).Decode(&result)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			// Обработка случая, когда ничего не найдено
			return Class{}, nil
		}
		return Class{}, err
	}
	return result, nil
}
