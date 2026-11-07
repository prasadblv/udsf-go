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
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(s.rejectReqWhenShuttingDown())

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	// Registers net/http/pprof handlers under /debug/pprof
	pprof.Register(r)

	//Liveness Handler
	r.GET("/healthz", s.livenessHandler)

	//Readiness Handler
	r.GET("/readyz", s.readinessdHandler)

	//Database health handler
	r.GET("/dbhealthz", s.dbhealthHandler)

	udsfRouter := udsfroute.NewRouter(
		api.NewBlockCRUDAPIController(api.NewBlockCRUDAPIService(s.db)),
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

func (s *Server) livenessHandler(c *gin.Context) {
	c.JSON(http.StatusOK, "OK")
}

func (s *Server) readinessdHandler(c *gin.Context) {
	c.JSON(http.StatusOK, "OK")
}

func (s *Server) dbhealthHandler(c *gin.Context) {
	if s.db == nil {
		c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "database not configured"})
		return
	}
	c.JSON(http.StatusOK, s.db.Health())
}

func (s *Server) rejectReqWhenShuttingDown() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.shuttingDown.Load() && c.Request.URL.Path != "/healthz" {
			c.Header("Connection", "close")
			c.Header("Retry-After", "5")
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "udsf is shutting down"})
			return
		}
		c.Next()
	}
}
