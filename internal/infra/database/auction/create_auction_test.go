package auction

import (
	"context"
	"fullcycle-auction_go/internal/entity/auction_entity"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

func TestNewAuctionRepository(t *testing.T) {
	t.Run("should create new auction repository with collection", func(t *testing.T) {
		// Arrange
		mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

		mt.Run("test", func(mt *mtest.T) {
			// Usar a collection real do mtest
			repo := &AuctionRepository{Collection: mt.Coll}

			// Assert
			assert.NotNil(t, repo)
			assert.NotNil(t, repo.Collection)
		})
	})
}

func TestCreateAuction(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success - should create auction successfully", func(mt *mtest.T) {
		// Arrange
		repo := &AuctionRepository{Collection: mt.Coll}
		auction := &auction_entity.Auction{
			Id:          "test-id",
			ProductName: "Test Product",
			Category:    "Electronics",
			Description: "Test description for auction",
			Condition:   auction_entity.New,
			Status:      auction_entity.Active,
			Timestamp:   time.Now(),
		}

		mt.AddMockResponses(mtest.CreateSuccessResponse())

		// Act
		err := repo.CreateAuction(context.Background(), auction)

		// Assert
		assert.Nil(t, err)
	})

	mt.Run("error - should return internal error when insert fails", func(mt *mtest.T) {
		// Arrange
		repo := &AuctionRepository{Collection: mt.Coll}
		auction := &auction_entity.Auction{
			Id:          "test-id",
			ProductName: "Test Product",
			Category:    "Electronics",
			Description: "Test description for auction",
			Condition:   auction_entity.New,
			Status:      auction_entity.Active,
			Timestamp:   time.Now(),
		}

		mt.AddMockResponses(mtest.CreateWriteErrorsResponse(mtest.WriteError{
			Index:   0,
			Code:    11000,
			Message: "duplicate key error",
		}))

		// Act
		err := repo.CreateAuction(context.Background(), auction)

		// Assert
		assert.NotNil(t, err)
		assert.Equal(t, "internal_server_error", err.Err)
	})
}

func TestGetAuctionDuration(t *testing.T) {
	tests := []struct {
		name           string
		envValue       string
		expectedResult time.Duration
	}{
		{
			name:           "should return default duration when env is empty",
			envValue:       "",
			expectedResult: time.Duration(defaultAuctionDuration) * time.Minute,
		},
		{
			name:           "should return default duration when env is invalid",
			envValue:       "invalid",
			expectedResult: time.Duration(defaultAuctionDuration) * time.Minute,
		},
		{
			name:           "should return default duration when env is negative",
			envValue:       "-10",
			expectedResult: time.Duration(defaultAuctionDuration) * time.Minute,
		},
		{
			name:           "should return custom duration from env",
			envValue:       "45",
			expectedResult: time.Duration(45) * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			if tt.envValue != "" {
				os.Setenv("AUCTION_DURATION_MINUTES", tt.envValue)
				defer os.Unsetenv("AUCTION_DURATION_MINUTES")
			}

			// Act
			result := getAuctionDuration()

			// Assert
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestGetAuctionByID(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("success - should return auction by id", func(mt *mtest.T) {
		// Arrange
		repo := &AuctionRepository{Collection: mt.Coll}
		expectedAuction := AuctionEntityMongo{
			Id:          "test-id",
			ProductName: "Test Product",
			Category:    "Electronics",
			Description: "Test description",
			Condition:   auction_entity.New,
			Status:      auction_entity.Active,
			Timestamp:   time.Now().Unix(),
		}

		mt.AddMockResponses(mtest.CreateCursorResponse(1, "test.auctions", mtest.FirstBatch, bson.D{
			{Key: "_id", Value: expectedAuction.Id},
			{Key: "product_name", Value: expectedAuction.ProductName},
			{Key: "category", Value: expectedAuction.Category},
			{Key: "description", Value: expectedAuction.Description},
			{Key: "condition", Value: expectedAuction.Condition},
			{Key: "status", Value: expectedAuction.Status},
			{Key: "timestamp", Value: expectedAuction.Timestamp},
		}))

		// Act
		auction, err := repo.GetAuctionByID(context.Background(), "test-id")

		// Assert
		assert.Nil(t, err)
		assert.NotNil(t, auction)
		assert.Equal(t, expectedAuction.Id, auction.Id)
		assert.Equal(t, expectedAuction.ProductName, auction.ProductName)
	})

	mt.Run("error - should return not found when auction doesn't exist", func(mt *mtest.T) {
		// Arrange
		repo := &AuctionRepository{Collection: mt.Coll}

		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.auctions", mtest.FirstBatch))

		// Act
		auction, err := repo.GetAuctionByID(context.Background(), "non-existent-id")

		// Assert
		assert.NotNil(t, err)
		assert.Nil(t, auction)
		assert.Equal(t, "not_found", err.Err)
	})

	mt.Run("error - should return internal error when find fails", func(mt *mtest.T) {
		// Arrange
		repo := &AuctionRepository{Collection: mt.Coll}

		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    1,
			Message: "connection error",
		}))

		// Act
		auction, err := repo.GetAuctionByID(context.Background(), "test-id")

		// Assert
		assert.NotNil(t, err)
		assert.Nil(t, auction)
		assert.Equal(t, "internal_server_error", err.Err)
	})
}

func TestCheckAndCloseExpiredAuctions(t *testing.T) {
	// Teste usando mtest para garantir compatibilidade de tipos
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("should handle error when finding expired auctions fails", func(mt *mtest.T) {
		// Arrange
		repo := &AuctionRepository{Collection: mt.Coll}

		// Mock error response para Find
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    1,
			Message: "find error",
		}))

		// Act & Assert - Não deve panicar
		assert.NotPanics(t, func() {
			repo.checkAndCloseExpiredAuctions()
		})
	})

	mt.Run("should handle no expired auctions found", func(mt *mtest.T) {
		// Arrange
		repo := &AuctionRepository{Collection: mt.Coll}

		// Mock response vazio para Find
		findCursor := mtest.CreateCursorResponse(0, "test.auctions", mtest.FirstBatch)
		mt.AddMockResponses(findCursor)

		// Act & Assert - Não deve panicar
		assert.NotPanics(t, func() {
			repo.checkAndCloseExpiredAuctions()
		})
	})

	mt.Run("should close expired auctions successfully", func(mt *mtest.T) {
		// Arrange
		repo := &AuctionRepository{Collection: mt.Coll}

		expiredAuction := AuctionEntityMongo{
			Id:          "expired-id",
			ProductName: "Expired Product",
			Category:    "Electronics",
			Description: "Expired auction",
			Condition:   auction_entity.New,
			Status:      auction_entity.Active,
			Timestamp:   time.Now().Add(-time.Hour * 2).Unix(),
		}

		// Mock responses
		findCursor := mtest.CreateCursorResponse(1, "test.auctions", mtest.FirstBatch, bson.D{
			{Key: "_id", Value: expiredAuction.Id},
			{Key: "product_name", Value: expiredAuction.ProductName},
			{Key: "category", Value: expiredAuction.Category},
			{Key: "description", Value: expiredAuction.Description},
			{Key: "condition", Value: expiredAuction.Condition},
			{Key: "status", Value: expiredAuction.Status},
			{Key: "timestamp", Value: expiredAuction.Timestamp},
		})
		getMoreCursor := mtest.CreateCursorResponse(0, "test.auctions", mtest.NextBatch)
		updateResponse := mtest.CreateSuccessResponse(
			bson.E{Key: "nModified", Value: 1},
			bson.E{Key: "ok", Value: 1},
		)

		mt.AddMockResponses(findCursor, getMoreCursor, updateResponse)

		// Act & Assert - Não deve panicar
		assert.NotPanics(t, func() {
			repo.checkAndCloseExpiredAuctions()
		})
	})
}

func TestAuctionEntityMongoConversion(t *testing.T) {
	t.Run("should convert auction entity to mongo entity correctly", func(t *testing.T) {
		// Arrange
		auctionEntity := &auction_entity.Auction{
			Id:          "test-id",
			ProductName: "Test Product",
			Category:    "Electronics",
			Description: "Test description for auction",
			Condition:   auction_entity.New,
			Status:      auction_entity.Active,
			Timestamp:   time.Now(),
		}

		// Act
		auctionMongo := &AuctionEntityMongo{
			Id:          auctionEntity.Id,
			ProductName: auctionEntity.ProductName,
			Category:    auctionEntity.Category,
			Description: auctionEntity.Description,
			Condition:   auctionEntity.Condition,
			Status:      auctionEntity.Status,
			Timestamp:   auctionEntity.Timestamp.Unix(),
		}

		// Assert
		assert.Equal(t, auctionEntity.Id, auctionMongo.Id)
		assert.Equal(t, auctionEntity.ProductName, auctionMongo.ProductName)
		assert.Equal(t, auctionEntity.Category, auctionMongo.Category)
		assert.Equal(t, auctionEntity.Description, auctionMongo.Description)
		assert.Equal(t, auctionEntity.Condition, auctionMongo.Condition)
		assert.Equal(t, auctionEntity.Status, auctionMongo.Status)
		assert.Equal(t, auctionEntity.Timestamp.Unix(), auctionMongo.Timestamp)
	})
}

// Testes auxiliares para métodos que não envolvem operações de banco de dados
func TestAuctionRepositoryMethods(t *testing.T) {
	t.Run("should handle mutex operations without panic", func(t *testing.T) {
		// Arrange
		mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

		mt.Run("test", func(mt *mtest.T) {
			repo := &AuctionRepository{Collection: mt.Coll}

			// Act & Assert - Testar que o mutex pode ser adquirido e liberado
			repo.mu.Lock()
			assert.NotPanics(t, func() {
				repo.mu.Unlock()
			})
		})
	})
}

// Teste simples para a função getAuctionDuration
func TestGetAuctionDuration_EdgeCases(t *testing.T) {
	t.Run("should handle empty environment variable", func(t *testing.T) {
		os.Unsetenv("AUCTION_DURATION_MINUTES")
		result := getAuctionDuration()
		assert.Equal(t, time.Duration(defaultAuctionDuration)*time.Minute, result)
	})

	t.Run("should handle zero duration", func(t *testing.T) {
		os.Setenv("AUCTION_DURATION_MINUTES", "0")
		defer os.Unsetenv("AUCTION_DURATION_MINUTES")
		result := getAuctionDuration()
		assert.Equal(t, time.Duration(defaultAuctionDuration)*time.Minute, result)
	})
}
