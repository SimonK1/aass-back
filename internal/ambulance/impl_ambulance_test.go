package ambulance

import (
    "context"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/suite"
    "github.com/wac-project/wac-api/internal/db_service"
)

// DbServiceMock is a testify mock for db_service.DbService[Ambulance]
type DbServiceMock[DocType any] struct {
    mock.Mock
}

func (m *DbServiceMock[DocType]) CreateDocument(ctx context.Context, id string, document *DocType) error {
    args := m.Called(ctx, id, document)
    return args.Error(0)
}

func (m *DbServiceMock[DocType]) FindDocument(ctx context.Context, id string) (*DocType, error) {
    args := m.Called(ctx, id)
    return args.Get(0).(*DocType), args.Error(1)
}

func (m *DbServiceMock[DocType]) UpdateDocument(ctx context.Context, id string, document *DocType) error {
    args := m.Called(ctx, id, document)
    return args.Error(0)
}

func (m *DbServiceMock[DocType]) DeleteDocument(ctx context.Context, id string) error {
    args := m.Called(ctx, id)
    return args.Error(0)
}

func (m *DbServiceMock[DocType]) Disconnect(ctx context.Context) error {
    args := m.Called(ctx)
    return args.Error(0)
}

// Ensure mock implements the DbService interface
var _ db_service.DbService[Ambulance] = (*DbServiceMock[Ambulance])(nil)

// AmbulanceSuite defines the suite for ambulance handler tests
type AmbulanceSuite struct {
    suite.Suite
    dbServiceMock *DbServiceMock[Ambulance]
}

func TestAmbulanceSuite(t *testing.T) {
    suite.Run(t, new(AmbulanceSuite))
}

func (suite *AmbulanceSuite) SetupTest() {
    suite.dbServiceMock = &DbServiceMock[Ambulance]{}
    // Stub FindDocument to return a sample Ambulance
    suite.dbServiceMock.
        On("FindDocument", mock.Anything, "test-ambulance").
        Return(
            &Ambulance{
                Id:         "test-ambulance",
                Name:       "TestName",
                Location:   "TestLoc",
                Department: "TestDept",
                Capacity:   5,
                Status:     "active",
            },
            nil,
        )
}

func (suite *AmbulanceSuite) Test_CreateAmbulance_CallsCreateDocument() {
    suite.dbServiceMock.
        On("CreateDocument", mock.Anything, mock.Anything, mock.Anything).
        Return(nil)

    payload := `{"name":"TestName","location":"TestLoc","department":"TestDept","capacity":5,"status":"active"}`
    gin.SetMode(gin.TestMode)
    recorder := httptest.NewRecorder()
    ctx, _ := gin.CreateTestContext(recorder)
    ctx.Set("db_service", suite.dbServiceMock)
    ctx.Request = httptest.NewRequest("POST", "/api/ambulances", strings.NewReader(payload))
    ctx.Request.Header.Set("Content-Type", "application/json")

    sut := implAmbulanceAPI{}
    sut.CreateAmbulance(ctx)

    suite.dbServiceMock.AssertCalled(suite.T(), "CreateDocument", mock.Anything, mock.Anything, mock.Anything)
    suite.Equal(http.StatusCreated, recorder.Code)
}

func (suite *AmbulanceSuite) Test_GetAmbulanceById_ReturnsOK() {
    gin.SetMode(gin.TestMode)
    recorder := httptest.NewRecorder()
    ctx, _ := gin.CreateTestContext(recorder)
    ctx.Set("db_service", suite.dbServiceMock)
    ctx.Params = []gin.Param{{Key: "ambulanceId", Value: "test-ambulance"}}
    ctx.Request = httptest.NewRequest("GET", "/api/ambulances/test-ambulance", nil)

    sut := implAmbulanceAPI{}
    sut.GetAmbulanceById(ctx)

    suite.Equal(http.StatusOK, recorder.Code)
}

func (suite *AmbulanceSuite) Test_DeleteAmbulance_CallsDeleteDocument() {
    suite.dbServiceMock.
        On("DeleteDocument", mock.Anything, "test-ambulance").
        Return(nil)

    gin.SetMode(gin.TestMode)
    recorder := httptest.NewRecorder()
    ctx, _ := gin.CreateTestContext(recorder)
    ctx.Set("db_service", suite.dbServiceMock)
    ctx.Params = []gin.Param{{Key: "ambulanceId", Value: "test-ambulance"}}
    ctx.Request = httptest.NewRequest("DELETE", "/api/ambulances/test-ambulance", nil)

    sut := implAmbulanceAPI{}
    sut.DeleteAmbulance(ctx)

    suite.dbServiceMock.AssertCalled(suite.T(), "DeleteDocument", mock.Anything, "test-ambulance")
    suite.Equal(http.StatusNoContent, recorder.Code)
}

func (suite *AmbulanceSuite) Test_GetAmbulanceSummary_ReturnsSummary() {
    gin.SetMode(gin.TestMode)
    recorder := httptest.NewRecorder()
    ctx, _ := gin.CreateTestContext(recorder)
    ctx.Set("db_service", suite.dbServiceMock)
    ctx.Params = []gin.Param{{Key: "ambulanceId", Value: "test-ambulance"}}
    ctx.Request = httptest.NewRequest("GET", "/api/ambulances/test-ambulance/summary", nil)

    sut := implAmbulanceAPI{}
    sut.GetAmbulanceSummary(ctx)

    suite.Equal(http.StatusOK, recorder.Code)
}
