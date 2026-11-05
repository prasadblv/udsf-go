package server

import (
	"net/http"

	"udsf-go/internal/udsf/api"
	udsfroute "udsf-go/internal/udsf/route"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
)

func (s *Server) RegisterRoutes() http.Handler {
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173"}, // Add your frontend URL
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true, // Enable cookies/auth
	}))

	// Registers net/http/pprof handlers under /debug/pprof
	pprof.Register(r)

	//Dummy hello world handler
	r.GET("/", s.HelloWorldHandler)

	//Database health handler
	r.GET("/dbhealthz", s.healthHandler)

	// UDSF (Nudsf_DataRepository) API: gorilla/mux router built from all generated controllers,
	// mounted into gin under the /nudsf-dr prefix.
	udsfRouter := udsfroute.NewRouter(
		api.NewBlockCRUDAPIController(api.NewBlockCRUDAPIService()),
		api.NewMetaSchemaCRUDAPIController(api.NewMetaSchemaCRUDAPIService()),
		api.NewNotificationSubscriptionCRUDAPIController(api.NewNotificationSubscriptionCRUDAPIService()),
		api.NewNotificationSubscriptionsCRUDAPIController(api.NewNotificationSubscriptionsCRUDAPIService()),
		api.NewRecordCRUDAPIController(api.NewRecordCRUDAPIService()),
		api.NewTimerGetAPIController(api.NewTimerGetAPIService()),
		api.NewTimerSearchAPIController(api.NewTimerSearchAPIService()),
		api.NewTimerStartAPIController(api.NewTimerStartAPIService()),
		api.NewTimerStopAPIController(api.NewTimerStopAPIService()),
		api.NewTimerUpdateAPIController(api.NewTimerUpdateAPIService()),
		api.NewTimersDeleteAPIController(api.NewTimersDeleteAPIService()),
	)
	r.Any("/nudsf-dr/*path", gin.WrapH(udsfRouter))

	return r
}

func (s *Server) HelloWorldHandler(c *gin.Context) {
	resp := make(map[string]string)
	resp["message"] = "Hello World"

	c.JSON(http.StatusOK, resp)
}

func (s *Server) healthHandler(c *gin.Context) {
	if s.db == nil {
		c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "database not configured"})
		return
	}
	c.JSON(http.StatusOK, s.db.Health())
}
