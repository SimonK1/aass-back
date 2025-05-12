package ambulance

import (
    "context"
    "log"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/wac-project/wac-api/internal/db_service"
)

// implProcedureAPI implements the ProcedureManagementAPI interface.
type implProcedureAPI struct{}

// NewProcedureAPI returns an implementation of ProcedureManagementAPI.
func NewProcedureAPI() ProcedureManagementAPI {
    return &implProcedureAPI{}
}

// getProcedureDB extracts the DbService[Procedure] from the context.
func getProcedureDB(c *gin.Context) db_service.DbService[Procedure] {
    return c.MustGet("db_service").(db_service.DbService[Procedure])
}

// withProcedureByID loads a Procedure and calls fn; fn may return an updated doc.
func withProcedureByID(
    c *gin.Context,
    fn func(*gin.Context, *Procedure) (*Procedure, interface{}, int),
) {
    id := c.Param("procedureId")
    if id == "" {
        c.JSON(http.StatusBadRequest, gin.H{"message": "procedureId is required"})
        return
    }

    db := getProcedureDB(c)
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    proc, err := db.FindDocument(ctx, id)
    if err != nil {
        if err == db_service.ErrNotFound {
            c.JSON(http.StatusNotFound, gin.H{"message": "Procedure not found"})
        } else {
            log.Println("FindDocument error:", err)
            c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal error"})
        }
        return
    }

    updated, result, status := fn(c, proc)
    if updated != nil {
        if err := db.UpdateDocument(ctx, id, updated); err != nil {
            log.Println("UpdateDocument error:", err)
            c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to update procedure"})
            return
        }
    }
    c.JSON(status, result)
}

// CreateProcedure implements POST /api/procedures
func (o *implProcedureAPI) CreateProcedure(c *gin.Context) {
    var p Procedure
    if err := c.ShouldBindJSON(&p); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request", "error": err.Error()})
        return
    }
    if p.Id == "" {
        p.Id = uuid.NewString()
    }

    db := getProcedureDB(c)
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    if err := db.CreateDocument(ctx, p.Id, &p); err != nil {
        switch err {
        case db_service.ErrConflict:
            c.JSON(http.StatusConflict, gin.H{"message": "Procedure already exists"})
        default:
            log.Println("CreateDocument error:", err)
            c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to create procedure"})
        }
        return
    }
    c.JSON(http.StatusCreated, p)
}

// GetProcedureById implements GET /api/procedures/:procedureId
func (o *implProcedureAPI) GetProcedureById(c *gin.Context) {
    withProcedureByID(c, func(_ *gin.Context, p *Procedure) (*Procedure, interface{}, int) {
        return nil, p, http.StatusOK
    })
}

// GetProcedures implements GET /api/procedures
func (o *implProcedureAPI) GetProcedures(c *gin.Context) {
    // Listing is not supported by DbService interface
    c.JSON(http.StatusNotImplemented, gin.H{
        "message": "Listing procedures is not supported by the current DbService interface",
    })
}

// UpdateProcedure implements PUT /api/procedures/:procedureId
func (o *implProcedureAPI) UpdateProcedure(c *gin.Context) {
    withProcedureByID(c, func(_ *gin.Context, existing *Procedure) (*Procedure, interface{}, int) {
        var upd Procedure
        if err := c.ShouldBindJSON(&upd); err != nil {
            return nil, gin.H{"message": "Invalid request", "error": err.Error()}, http.StatusBadRequest
        }
        if upd.Patient != "" {
            existing.Patient = upd.Patient
        }
        if upd.VisitType != "" {
            existing.VisitType = upd.VisitType
        }
        if upd.Price != 0 {
            existing.Price = upd.Price
        }
        if upd.Payer != "" {
            existing.Payer = upd.Payer
        }
        if upd.AmbulanceId != "" {
            existing.AmbulanceId = upd.AmbulanceId
        }
        if upd.Timestamp != "" {
            existing.Timestamp = upd.Timestamp
        }
        return existing, existing, http.StatusOK
    })
}

// DeleteProcedure implements DELETE /api/procedures/:procedureId
func (o *implProcedureAPI) DeleteProcedure(c *gin.Context) {
    withProcedureByID(c, func(_ *gin.Context, p *Procedure) (*Procedure, interface{}, int) {
        db := getProcedureDB(c)
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        if err := db.DeleteDocument(ctx, p.Id); err != nil {
            log.Println("DeleteDocument error:", err)
            return nil, gin.H{"message": "Failed to delete procedure"}, http.StatusInternalServerError
        }
        return nil, nil, http.StatusNoContent
    })
}
