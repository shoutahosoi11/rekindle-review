package router

import (
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/shout/ai-study-tool/backend/internal/di"
)

func RegisterAPI(e *echo.Echo, container *di.Container) {
	api := e.Group("/api")
	apiV1 := e.Group("/api/v1")
	authMiddleware := container.FirebaseMiddleware.Authenticate
	ingestRateLimit := container.RateLimitMiddleware.Limit

	registerUserRoutes(api, container, authMiddleware)
	registerPostRoutes(api, container, authMiddleware)
	registerHighlightRoutes(api, container, authMiddleware, ingestRateLimit)
	registerQuestionRoutes(api, container, authMiddleware)
	registerSocialRoutes(api, container, authMiddleware)
	registerMonetizationRoutes(apiV1, container, authMiddleware)
	e.POST("/webhooks/stripe", container.StripeHandler.HandleWebhook)
}

func registerUserRoutes(api *echo.Group, container *di.Container, authMiddleware echo.MiddlewareFunc) {
	users := api.Group("/users", authMiddleware)
	users.POST("/signup", container.UserHandler.SignUp)
	users.GET("/me", container.UserHandler.GetMe)
	users.PUT("/me/question-settings", container.UserHandler.UpdateQuestionSettings)
	users.GET("/:id", container.UserHandler.GetUser)
	users.PUT("/me", container.UserHandler.UpdateProfile)
}

func registerPostRoutes(api *echo.Group, container *di.Container, authMiddleware echo.MiddlewareFunc) {
	posts := api.Group("/posts", authMiddleware)
	posts.GET("/timeline", container.PostHandler.GetTimeline)
	posts.GET("/:id/questions", container.PostHandler.ListQuestions)
	posts.GET("/:id", container.PostHandler.GetPost)
	posts.POST("", container.PostHandler.CreatePost)
}

func registerHighlightRoutes(
	api *echo.Group,
	container *di.Container,
	authMiddleware echo.MiddlewareFunc,
	ingestRateLimit echo.MiddlewareFunc,
) {
	highlights := api.Group("/highlights", authMiddleware)
	highlights.POST("/sync/check", container.HighlightHandler.CheckExistingHashes)
	highlights.POST("/import", container.HighlightHandler.Import, ingestRateLimit)
	highlights.POST("/share", container.HighlightHandler.ImportShared, ingestRateLimit)
	highlights.POST("/paste", container.HighlightHandler.ImportPaste, echomiddleware.BodyLimit("5K"), ingestRateLimit)
	highlights.GET("/books", container.HighlightHandler.ListBooks)
	highlights.GET("/books/search/items", container.HighlightHandler.ListByBookMetadata)
	highlights.GET("/books/:asin/items", container.HighlightHandler.ListByASIN)
	highlights.PUT("/:id/explanation", container.HighlightHandler.UpdateExplanation)
}

func registerQuestionRoutes(api *echo.Group, container *di.Container, authMiddleware echo.MiddlewareFunc) {
	questions := api.Group("/questions", authMiddleware)
	questions.GET("", container.QuestionHandler.List)
	questions.GET("/prepared", container.QuestionHandler.ListPrepared)
	questions.GET("/saved", container.QuestionHandler.ListSaved)
	questions.GET("/incorrect", container.QuestionHandler.ListIncorrect)
	questions.POST("/sync", container.QuestionHandler.SyncStock)
	questions.POST("/generate/manual", container.QuestionHandler.ManualGenerate)
	questions.POST("/:id/save", container.QuestionHandler.SaveQuestion)
	questions.POST("/:id/answer", container.AnswerHandler.SubmitAnswer)
	questions.POST("/:id/grade", container.AnswerHandler.SubmitAnswer)
}

func registerMonetizationRoutes(api *echo.Group, container *di.Container, authMiddleware echo.MiddlewareFunc) {
	tokens := api.Group("/tokens", authMiddleware)
	tokens.POST("/award", container.TokenHandler.Award)
	tokens.GET("/balance", container.TokenHandler.Balance)

	checkout := api.Group("/checkout", authMiddleware)
	checkout.POST("/session", container.StripeHandler.CreateCheckoutSession)

	questions := api.Group("/questions", authMiddleware)
	questions.POST("/generate/manual", container.QuestionHandler.ManualGenerate)
}

func registerSocialRoutes(api *echo.Group, container *di.Container, authMiddleware echo.MiddlewareFunc) {
	social := api.Group("", authMiddleware)
	social.POST("/users/:id/follow", container.SocialHandler.Follow)
	social.DELETE("/users/:id/follow", container.SocialHandler.Unfollow)
	social.POST("/posts/:id/like", container.SocialHandler.Like)
	social.DELETE("/posts/:id/like", container.SocialHandler.Unlike)
	social.POST("/posts/:id/repost", container.SocialHandler.Repost)
	social.DELETE("/posts/:id/repost", container.SocialHandler.Unrepost)
	social.POST("/posts/:id/comments", container.SocialHandler.CreateComment)
	social.GET("/posts/:id/comments", container.SocialHandler.ListComments)
}
