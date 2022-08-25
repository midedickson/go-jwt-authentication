package routes

import (
	"github.com/Double-DOS/go-jwt/controllers"
	"github.com/Double-DOS/go-jwt/middleware"
	"github.com/gin-gonic/gin"
)

func UserRoutes(incomingRoutes *gin.Engine) {
	incomingRoutes.Use(middleware.Authenticate())
	incomingRoutes.GET("/users/:user_id", controllers.GetUsers())
	incomingRoutes.POST("/users/", controllers.GetUser())
}
