package auction

import (
	"context"
	"fmt"
	"fullcycle-auction_go/configuration/logger"
	"fullcycle-auction_go/internal/entity/auction_entity"
	"fullcycle-auction_go/internal/internal_error"
	"os"
	"strconv"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	defaultAuctionDuration = 60 // minutos
	checkInterval          = 30 // segundos
)

type AuctionEntityMongo struct {
	Id          string                          `bson:"_id"`
	ProductName string                          `bson:"product_name"`
	Category    string                          `bson:"category"`
	Description string                          `bson:"description"`
	Condition   auction_entity.ProductCondition `bson:"condition"`
	Status      auction_entity.AuctionStatus    `bson:"status"`
	Timestamp   int64                           `bson:"timestamp"`
}
type AuctionRepository struct {
	Collection *mongo.Collection
	mu         sync.Mutex
}

func NewAuctionRepository(database *mongo.Database) *AuctionRepository {
	repo := &AuctionRepository{
		Collection: database.Collection("auctions"),
	}

	// Inicia a goroutine para verificar leilões vencidos
	go repo.startAuctionChecker()

	return repo
}

func (ar *AuctionRepository) CreateAuction(
	ctx context.Context,
	auctionEntity *auction_entity.Auction) *internal_error.InternalError {
	auctionEntityMongo := &AuctionEntityMongo{
		Id:          auctionEntity.Id,
		ProductName: auctionEntity.ProductName,
		Category:    auctionEntity.Category,
		Description: auctionEntity.Description,
		Condition:   auctionEntity.Condition,
		Status:      auctionEntity.Status,
		Timestamp:   auctionEntity.Timestamp.Unix(),
	}
	_, err := ar.Collection.InsertOne(ctx, auctionEntityMongo)
	if err != nil {
		logger.Error("Error trying to insert auction", err)
		return internal_error.NewInternalServerError("Error trying to insert auction")
	}

	return nil
}

// getAuctionDuration obtém a duração do leilão a partir de variáveis de ambiente
func getAuctionDuration() time.Duration {
	durationStr := os.Getenv("AUCTION_DURATION_MINUTES")
	if durationStr == "" {
		return time.Duration(defaultAuctionDuration) * time.Minute
	}

	duration, err := strconv.Atoi(durationStr)
	if err != nil || duration <= 0 {
		logger.Info(fmt.Sprintf("Invalid AUCTION_DURATION_MINUTES: %s, using default: %d minutes",
			durationStr, defaultAuctionDuration))
		return time.Duration(defaultAuctionDuration) * time.Minute
	}

	return time.Duration(duration) * time.Minute
}

// startAuctionChecker inicia a goroutine que verifica leilões vencidos
func (ar *AuctionRepository) startAuctionChecker() {
	ticker := time.NewTicker(checkInterval * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ar.checkAndCloseExpiredAuctions()
	}
}

// checkAndCloseExpiredAuctions verifica e fecha leilões vencidos
func (ar *AuctionRepository) checkAndCloseExpiredAuctions() {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Calcula o tempo limite para leilões expirados
	auctionDuration := getAuctionDuration()
	expirationTime := time.Now().Add(-auctionDuration).Unix()

	// Busca leilões abertos que expiraram
	filter := bson.M{
		"status": auction_entity.Active,
		"timestamp": bson.M{
			"$lte": expirationTime,
		},
	}

	cursor, err := ar.Collection.Find(ctx, filter)
	if err != nil {
		logger.Error("Error finding expired auctions", err)
		return
	}
	defer cursor.Close(ctx)

	var expiredAuctions []AuctionEntityMongo
	if err := cursor.All(ctx, &expiredAuctions); err != nil {
		logger.Error("Error decoding expired auctions", err)
		return
	}

	// Fecha os leilões expirados
	for _, auction := range expiredAuctions {
		updateFilter := bson.M{"_id": auction.Id}
		update := bson.M{
			"$set": bson.M{
				"status": auction_entity.Completed,
			},
		}

		_, err := ar.Collection.UpdateOne(ctx, updateFilter, update)
		if err != nil {
			logger.Error(fmt.Sprintf("Error closing auction %s", auction.Id), err)
			continue
		}

		logger.Info(fmt.Sprintf("Auction %s closed automatically", auction.Id))
	}
}

// GetAuctionByID - Método auxiliar para testes
func (ar *AuctionRepository) GetAuctionByID(ctx context.Context, auctionID string) (*AuctionEntityMongo, *internal_error.InternalError) {
	filter := bson.M{"_id": auctionID}

	var auction AuctionEntityMongo
	err := ar.Collection.FindOne(ctx, filter).Decode(&auction)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, internal_error.NewNotFoundError("Auction not found")
		}
		logger.Error("Error finding auction", err)
		return nil, internal_error.NewInternalServerError("Error finding auction")
	}

	return &auction, nil
}
