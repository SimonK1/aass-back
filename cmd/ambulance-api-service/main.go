package main

import (
    "context"
    "log"
    "os"
    "strings"
    "time"


    "github.com/gin-contrib/cors"
    "github.com/gin-gonic/gin"

    "github.com/wac-project/wac-api/api"
	"github.com/wac-project/wac-api/internal/ambulance"
	"github.com/wac-project/wac-api/internal/db_service"
)

func main() {
    log.Printf("Server started")

    port := os.Getenv("AMBULANCE_API_PORT")
    if port == "" {
        port = "8080"
    }

    environment := os.Getenv("AMBULANCE_API_ENVIRONMENT")
    if !strings.EqualFold(environment, "production") {
        gin.SetMode(gin.DebugMode)
    }

    engine := gin.New()
    engine.Use(gin.Recovery())

    corsMiddleware := cors.New(cors.Config{
        AllowOrigins:     []string{"*"},
        AllowMethods:     []string{"GET", "PUT", "POST", "DELETE", "PATCH", "OPTIONS"},
        AllowHeaders:     []string{"Origin", "Authorization", "Content-Type"},
        ExposeHeaders:    []string{""},
        AllowCredentials: false,
        MaxAge:           12 * time.Hour,
    })
    engine.Use(corsMiddleware)

    // one service per collection/type
   dbAmbSvc  := db_service.NewMongoService[ambulance.Ambulance](db_service.MongoServiceConfig{Collection: "ambulance"})
   dbPaySvc  := db_service.NewMongoService[ambulance.Payment](  db_service.MongoServiceConfig{Collection: "payment"})
   dbProcSvc := db_service.NewMongoService[ambulance.Procedure](db_service.MongoServiceConfig{Collection: "procedure"})

   // tear down all three on exit
   defer dbAmbSvc.Disconnect(context.Background())
   defer dbPaySvc.Disconnect(context.Background())
   defer dbProcSvc.Disconnect(context.Background())

   // inject each under its own key
   engine.Use(func(ctx *gin.Context) {
       ctx.Set("db_service_ambulance", dbAmbSvc)
       ctx.Set("db_service_payment",   dbPaySvc)
       ctx.Set("db_service_procedure",  dbProcSvc)
           ctx.Next()
    })

    handleFunctions := &ambulance.ApiHandleFunctions{
        AmbulanceManagementAPI: ambulance.NewAmbulanceAPI(),
        PaymentManagementAPI:   ambulance.NewPaymentAPI(),
        ProcedureManagementAPI: ambulance.NewProcedureAPI(),
    }

    ambulance.NewRouterWithGinEngine(engine, *handleFunctions)
    engine.GET("/openapi", api.HandleOpenApi)
    engine.Run(":" + port)
}
