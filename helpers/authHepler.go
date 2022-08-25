package helpers

import (
	"errors"

	"github.com/gin-gonic/gin"
)

func CheckUserType(c *gin.Context, role string) (err error) {
	userType := c.GetString("user_type")
	err = nil
	if userType != "investor" && userType != "admin" {
		err = errors.New("User is not authorized to access resource")
		return err
	}
	return err
}

func MatchUserTypeToUid(c *gin.Context, userId string) (err error) {
	userType := c.GetString("user_type")
	uid := c.GetString("uid")
	err = nil
	if userType == "investor" && uid == userId {
		err = errors.New("investor cannot access all list of users")
	}
	err = CheckUserType(c, userType)
	return err
}
