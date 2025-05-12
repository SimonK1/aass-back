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

// implPaymentAPI implements the PaymentManagementAPI interface.
type implPaymentAPI struct{}

// NewPaymentAPI returns an implementation of PaymentManagementAPI.
func NewPaymentAPI() PaymentManagementAPI {
    return &implPaymentAPI{}
}

// getPaymentDB extracts the DbService[Payment] from the context.
func getPaymentDB(c *gin.Context) db_service.DbService[Payment] {
    return c.MustGet("db_service").(db_service.DbService[Payment])
}

// withPaymentByID loads a Payment and calls fn; fn may return an updated doc.
func withPaymentByID(
    c *gin.Context,
    fn func(*gin.Context, *Payment) (*Payment, interface{}, int),
) {
    id := c.Param("paymentId")
    if id == "" {
        c.JSON(http.StatusBadRequest, gin.H{"message": "paymentId is required"})
        return
    }

    db := getPaymentDB(c)
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    p, err := db.FindDocument(ctx, id)
    if err != nil {
        if err == db_service.ErrNotFound {
            c.JSON(http.StatusNotFound, gin.H{"message": "Payment not found"})
        } else {
            log.Println("FindDocument error:", err)
            c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal error"})
        }
        return
    }

    updated, result, status := fn(c, p)
    if updated != nil {
        if err := db.UpdateDocument(ctx, id, updated); err != nil {
            log.Println("UpdateDocument error:", err)
            c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to update payment"})
            return
        }
    }
    c.JSON(status, result)
}

// CreatePayment implements POST /api/payments
func (o *implPaymentAPI) CreatePayment(c *gin.Context) {
    var p Payment
    if err := c.ShouldBindJSON(&p); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request", "error": err.Error()})
        return
    }
    if p.Id == "" {
        p.Id = uuid.NewString()
    }

    db := getPaymentDB(c)
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    if err := db.CreateDocument(ctx, p.Id, &p); err != nil {
        switch err {
        case db_service.ErrConflict:
            c.JSON(http.StatusConflict, gin.H{"message": "Payment already exists"})
        default:
            log.Println("CreateDocument error:", err)
            c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to create payment"})
        }
        return
    }
    c.JSON(http.StatusCreated, p)
}

// GetPaymentById implements GET /api/payments/:paymentId
func (o *implPaymentAPI) GetPaymentById(c *gin.Context) {
    withPaymentByID(c, func(_ *gin.Context, p *Payment) (*Payment, interface{}, int) {
        return nil, p, http.StatusOK
    })
}

// GetPayments implements GET /api/payments
func (o *implPaymentAPI) GetPayments(c *gin.Context) {
    // Listing is not supported by DbService interface
    c.JSON(http.StatusNotImplemented, gin.H{
        "message": "Listing payments is not supported by the current DbService interface",
    })
}

// UpdatePayment implements PUT /api/payments/:paymentId
func (o *implPaymentAPI) UpdatePayment(c *gin.Context) {
    withPaymentByID(c, func(_ *gin.Context, existing *Payment) (*Payment, interface{}, int) {
        var upd Payment
        if err := c.ShouldBindJSON(&upd); err != nil {
            return nil, gin.H{"message": "Invalid request", "error": err.Error()}, http.StatusBadRequest
        }
        if upd.Insurance != "" {
            existing.Insurance = upd.Insurance
        }
        if upd.Amount != 0 {
            existing.Amount = upd.Amount
        }
        if upd.Timestamp != "" {
            existing.Timestamp = upd.Timestamp
        }
        return existing, existing, http.StatusOK
    })
}

// DeletePayment implements DELETE /api/payments/:paymentId
func (o *implPaymentAPI) DeletePayment(c *gin.Context) {
    withPaymentByID(c, func(_ *gin.Context, p *Payment) (*Payment, interface{}, int) {
        db := getPaymentDB(c)
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        if err := db.DeleteDocument(ctx, p.Id); err != nil {
            log.Println("DeleteDocument error:", err)
            return nil, gin.H{"message": "Failed to delete payment"}, http.StatusInternalServerError
        }
        return nil, nil, http.StatusNoContent
    })
}
