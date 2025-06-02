the package mocks

import (
	"context"

	"stripe-demo/services/user-service/internal/model"

	"github.com/stretchr/testify/mock"
)

// UserRepository est un mock de UserRepository
type UserRepository struct {
	mock.Mock
}

func (m *UserRepository) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

// Implémentez ici les autres méthodes nécessaires pour les tests
// Par exemple :
// CreateUser, UpdateUser, DeleteUser, etc.

// Exemple d'implémentation pour une méthode supplémentaire :
func (m *UserRepository) CreateUser(ctx context.Context, user *model.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

// Ajoutez d'autres méthodes mockées selon les besoins de vos tests

func (m *UserRepository) AddUserRole(ctx context.Context, userID, role string) error {
	args := m.Called(ctx, userID, role)
	return args.Error(0)
}

func (m *UserRepository) RemoveUserRole(ctx context.Context, userID, role string) error {
	args := m.Called(ctx, userID, role)
	return args.Error(0)
}

func (m *UserRepository) CountUsers(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}
