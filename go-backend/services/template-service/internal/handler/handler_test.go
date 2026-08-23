package handler_test

import (
    "net/http"
    "net/http/httptest"
    "testing"
    
    "github.com/rs/zerolog"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    
    "template-service/internal/handler"
)

type MockService struct {
    mock.Mock
}

func (m *MockService) HealthCheck() error {
    args := m.Called()
    return args.Error(0)
}

func (m *MockService) GetResource(id string) (interface{}, error) {
    args := m.Called(id)
    return args.Get(0), args.Error(1)
}

func (m *MockService) CreateResource(data interface{}) error {
    args := m.Called(data)
    return args.Error(0)
}

func TestHealthCheck(t *testing.T) {
    tests := []struct {
        name           string
        setupMock      func(*MockService)
        expectedStatus int
    }{
        {
            name: "healthy",
            setupMock: func(ms *MockService) {
                ms.On("HealthCheck").Return(nil)
            },
            expectedStatus: http.StatusOK,
        },
        {
            name: "unhealthy",
            setupMock: func(ms *MockService) {
                ms.On("HealthCheck").Return(assert.AnError)
            },
            expectedStatus: http.StatusServiceUnavailable,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mockSvc := new(MockService)
            tt.setupMock(mockSvc)

            h := handler.New(mockSvc, zerolog.Nop())
            
            req := httptest.NewRequest("GET", "/health", nil)
            w := httptest.NewRecorder()
            
            h.HealthCheck(w, req)
            
            assert.Equal(t, tt.expectedStatus, w.Code)
            mockSvc.AssertExpectations(t)
        })
    }
}
