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

    // 🎯 Zeebe & Camunda integration
    zb "github.com/camunda-cloud/zeebe/clients/go/pkg/zbc"
    "github.com/aass/internal/workers"
    perfapi "github.com/aass/internal/api" // alias so it doesn’t conflict
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

    // ─── Zeebe Client & Workflow Deployment ────────────────────────────────────────
    zc, err := zb.NewClient(&zb.ClientConfig{
        GatewayAddress:         "0.0.0.0:26500",
        UsePlaintextConnection: true,
    })
    if err != nil {
        log.Fatalf("Zeebe client error: %v", err)
    }

    resp, err := zc.NewDeployProcessCommand().
        AddResourceFile("deployments/detailed_project_diagram_v2.bpmn").
        Send(context.Background())
    if err != nil {
        log.Fatalf("Workflow deployment failed: %v", err)
    }
    for _, wf := range resp.Processes {
        log.Printf("Deployed %s (version %d)", wf.BpmnProcessId, wf.Version)
    }

    // ─── Start all Zeebe workers ───────────────────────────────────────────────────
    workers.RegisterAll(zc)

    dbService := db_service.NewMongoService[ambulance.Ambulance](db_service.MongoServiceConfig{})
    defer dbService.Disconnect(context.Background())

    engine.Use(func(ctx *gin.Context) {
        ctx.Set("db_service", dbService)
        ctx.Next()
    })

    handleFunctions := &ambulance.ApiHandleFunctions{
        AmbulanceManagementAPI: ambulance.NewAmbulanceAPI(),
        PaymentManagementAPI:   ambulance.NewPaymentAPI(),
        ProcedureManagementAPI: ambulance.NewProcedureAPI(),
    }

    ambulance.NewRouterWithGinEngine(engine, *handleFunctions)

    // 🎯 Add the performance‐start endpoint
    engine.POST("/api/v1/performance", perfapi.StartPerformanceHandler(zc))

    engine.GET("/openapi", api.HandleOpenApi)
    engine.Run(":" + port)
}
