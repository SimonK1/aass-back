package api

import (
    "context"
    "fmt"
    "net/http"

    "github.com/gin-gonic/gin"
    zb "github.com/camunda-cloud/zeebe/clients/go/pkg/zbc"
)

type PerformanceRequest struct {
    AmbulanceName string  `json:"ambulanceName"`
    DoctorName    string  `json:"doctorName"`
    Cost          float64 `json:"cost"`
    Payer         string  `json:"payer"`
}

func StartPerformanceHandler(client zb.Client) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req PerformanceRequest
        if err := c.BindJSON(&req); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
            return
        }
        _, err := client.NewCreateInstanceCommand().
            BPMNProcessId("SubmitMedicalPerformance").
            LatestVersion().
            VariablesFromString(fmt.Sprintf(
                `{"ambulanceName":"%s","doctorName":"%s","cost":%f,"payer":"%s"}`,
                req.AmbulanceName, req.DoctorName, req.Cost, req.Payer,
            )).
            Send(context.Background())
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start process"})
            return
        }
        c.JSON(http.StatusAccepted, gin.H{"message": "process started"})
    }
}
