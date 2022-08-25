package controllers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/Double-DOS/go-jwt/database"
	"github.com/Double-DOS/go-jwt/helpers"
	"github.com/Double-DOS/go-jwt/models"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

var userCollection *mongo.Collection = database.OpenCollection(database.Client, "user")
var validate = validator.New()

func HashPassword(userPassword string) string {
	bytes, err := bcrypt.GenerateFromPassword([]byte(userPassword), 14)
	if err != nil {
		log.Panic(err)
	}
	return string(bytes)

}

func VerifyPassword(userPassword string, providedPassword string) (bool, string) {
	err := bcrypt.CompareHashAndPassword([]byte(providedPassword), []byte(userPassword))
	check := true
	msg := ""
	if err != nil {
		msg = fmt.Sprintf("email or password is incorrect")
		check = false
	}
	return check, msg

}

func Signup() gin.HandlerFunc {
	return func(c *gin.Context) {
		var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
		var user models.User
		if err := c.BindJSON(&user); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		validationErr := validate.Struct(user)

		if validationErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": validationErr.Error()})
		}
		count, err := userCollection.CountDocuments(ctx, bson.M{"email": user.Email})
		defer cancel()
		if err != nil {
			log.Panic(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error while checking for user's email"})

		}
		password := HashPassword(*user.Password)
		user.Password = &password
		count, err = userCollection.CountDocuments(ctx, bson.M{"phone": user.Phone})
		defer cancel()
		if err != nil {
			log.Panic(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error while checking for user's phone number"})

		}
		if count > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User with email or phone number already exists"})
			return
		}
		user.CreatedAt, _ = time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
		user.UpdatedAt, _ = time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
		user.ID = primitive.NewObjectID()
		user.UserID = user.ID.Hex()
		token, refreshToken, _ := helpers.GenerateAllTokens(*user.Email, *user.FirstName, *user.LastName, *user.UserType, *&user.UserID)
		user.Token = &token
		user.RefreshToken = &refreshToken
		resultInsertionNumber, insertErr := userCollection.InsertOne(ctx, user)
		if insertErr != nil {
			msg := fmt.Sprintf("Error while creating user")
			c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
			return
		}
		defer cancel()
		c.JSON(http.StatusOK, gin.H{"message": "User created successfully", "user_id": resultInsertionNumber.InsertedID})
	}
}

func Login() gin.HandlerFunc {
	return func(c *gin.Context) {
		var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
		var user models.User
		var foundUser models.User
		if err := c.BindJSON(&user); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		err := userCollection.FindOne(ctx, bson.M{"email": user.Email}).Decode(&foundUser)
		defer cancel()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Email or Password is incorrect"})
			return
		}
		passwordIsValid, msg := VerifyPassword(*user.Password, *foundUser.Password)
		if passwordIsValid != true {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}
		if foundUser.Email == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User not found"})
			return
		}
		token, refreshToken, _ := helpers.GenerateAllTokens(*foundUser.Email, *foundUser.FirstName, *foundUser.LastName, *foundUser.UserType, *&foundUser.UserID)
		helpers.UpdateAllTokens(token, refreshToken, foundUser.UserID)
		err = userCollection.FindOne(ctx, bson.M{"user_id": foundUser.UserID}).Decode(&foundUser)
		defer cancel()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, foundUser)

		defer cancel()

	}
}

func GetUsers() gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := helpers.CheckUserType(c, "super-admin"); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
		recordPerPage, err := strconv.Atoi(c.Query("recordPerPage"))
		if err != nil || recordPerPage < 1 {
			recordPerPage = 10
		}
		page, err1 := strconv.Atoi(c.Query("page"))
		if err1 != nil || page < 1 {
			page = 1
		}
		startIndex := (page - 1) * recordPerPage
		startIndex, err = strconv.Atoi(c.Query("startIndex"))
		matchStage := bson.D{{Key: "$match", Value: bson.D{{}}}}
		groupStage := bson.D{
			{Key: "$group", Value: bson.D{
				{Key: "_id", Value: bson.D{{Key: "_id", Value: "null"}}},
				{Key: "total_count", Value: bson.D{{Key: "$sum", Value: 1}}},
				{Key: "data", Value: bson.D{{Key: "$push", Value: "$$ROOT"}}}}}}
		projectStage := bson.D{
			{Key: "$project", Value: bson.D{
				{Key: "_id", Value: 0},
				{Key: "total_caount", Value: 1},
				{Key: "user_items", Value: bson.D{{Key: "$slice", Value: []interface{}{"$data", startIndex, recordPerPage}}}},
			}},
		}
		result, err := userCollection.Aggregate(ctx, mongo.Pipeline{matchStage, groupStage, projectStage})
		defer cancel()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error occured while listing users"})

		}
		var allUsers []bson.M

		if err = result.All(ctx, &allUsers); err != nil {
			log.Fatal(err)
		}
		c.JSON(http.StatusOK, allUsers[0])

	}
}

func GetUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		userId := c.Param("user_id")
		if err := helpers.MatchUserTypeToUid(c, userId); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var ctx, cancel = context.WithTimeout(context.Background(), 100*time.Second)
		var user models.User
		err := userCollection.FindOne(ctx, bson.M{"user_id": userId}).Decode(&user)
		defer cancel()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, user)
	}
}
