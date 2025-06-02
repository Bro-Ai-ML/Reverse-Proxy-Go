package service

import (
	"context"
	"encoding/json"
	"testing"

	"stripe-demo/services/user-service/internal/model"
	"stripe-demo/services/user-service/internal/repository/mocks"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRGPDSvc_ExportUserData(t *testing.T) {
	mockRepo := new(mocks.UserRepository)
	svc := NewRGPDSvc(mockRepo)
	userID := uuid.New().String()
	expectedUser := &model.User{ID: uuid.MustParse(userID), Email: "test@example.com"}
	mockRepo.On("GetUserByID", mock.Anything, userID).Return(expectedUser, nil)
	data, err := svc.ExportUserData(context.Background(), userID)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "test@example.com")

	var exported map[string]interface{}
	err = json.Unmarshal(data, &exported)
	assert.NoError(t, err)

	assert.Equal(t, expectedUser.Email, exported["email"])
	mockRepo.AssertExpectations(t)

	mockRepo.On("GetUserByID", mock.Anything, "notfound").Return(nil, assert.AnError)
	_, err = svc.ExportUserData(context.Background(), "notfound")
	assert.Error(t, err)
}
