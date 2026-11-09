package services

import (
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"tigerswap/backend/models"
)

// NFTMarketplaceService implements the self-built NFT marketplace described by
// contracts/TigerSwapNFTMarketplace.sol. Collections are minted by the platform
// itself, a listing escrows the token, and an offer escrows the buyer's
// payment, so neither a sale nor an accepted offer can fail for a missing asset
// or a missing balance.
//
// Every money movement goes through the platform ledger: a sale debits the
// buyer, credits the seller net of the marketplace fee, and credits the fee to
// the configured fee account. The fee is frozen onto the listing (and the
// offer) when it is created, exactly as the contract snapshots `feeBps` at
// settlement, so an admin fee change cannot alter a sale that was already
// agreed.
type NFTMarketplaceService struct {
	db     *gorm.DB
	ledger *LedgerService

	// feeAccount is the platform account that receives the marketplace fee. A
	// zero-value account disables fee capture rather than silently dropping the
	// fee on the floor.
	feeAccount string
	feeBps     int
}

// nftMaxFeeBps is the ceiling on the marketplace fee, matching the contract's
// MAX_FEE_BPS so an operator cannot configure a fee the contract would reject.
const nftMaxFeeBps = 500

// nftDefaultFeeBps is the initial marketplace fee, 2.5%, matching the contract's
// documented default.
const nftDefaultFeeBps = 250

// nftEscrowOwner is the sentinel owner recorded on a token while a listing
// holds it. It is not a real account, so no user id can collide with it.
const nftEscrowOwner = "__nft_escrow__"

// nftDefaultFeeAccount is the ledger account that receives marketplace fees
// until an operator points it elsewhere.
const nftDefaultFeeAccount = "__nft_fee__"

func NewNFTMarketplaceService(db *gorm.DB, ledger *LedgerService) *NFTMarketplaceService {
	return &NFTMarketplaceService{
		db:         db,
		ledger:     ledger,
		feeAccount: nftDefaultFeeAccount,
		feeBps:     nftDefaultFeeBps,
	}
}

// SetFeeAccount points fee capture at a ledger account. An empty value falls
// back to the default so a sale never silently loses its fee.
func (s *NFTMarketplaceService) SetFeeAccount(account string) {
	account = strings.TrimSpace(account)
	if account == "" {
		s.feeAccount = nftDefaultFeeAccount
		return
	}
	s.feeAccount = account
}

// SetFeeBps sets the marketplace fee, rejecting a value above the contract
// ceiling.
func (s *NFTMarketplaceService) SetFeeBps(bps int) error {
	if bps < 0 || bps > nftMaxFeeBps {
		return fmt.Errorf("fee must be between 0 and %d basis points", nftMaxFeeBps)
	}
	s.feeBps = bps
	return nil
}

// CurrentFeeBps reports the fee applied to newly created listings and offers.
func (s *NFTMarketplaceService) CurrentFeeBps() int { return s.feeBps }

// NFT errors. Each is a distinct value so a handler maps it to the right status
// without matching on a message.
var (
	errNFTNotFound        = errors.New("not found")
	errNFTNotOwner        = errors.New("you do not own this token")
	errNFTInEscrow        = errors.New("token is escrowed by a live listing")
	errNFTNotActive       = errors.New("listing is not active")
	errNFTExpired         = errors.New("listing or offer has expired")
	errNFTSelfTrade       = errors.New("cannot trade with yourself")
	errNFTNoBid           = errors.New("offer is not active")
	errNFTUnsupportedPay  = errors.New("unsupported payment token")
	errNFTCollectionOff   = errors.New("collection is not active")
	errNFTFeeExceedsPrice = errors.New("fee exceeds the sale price")
)

// parseUint64Query parses a positive base-10 uint64 query parameter.
func parseUint64Query(raw string) (uint64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, errors.New("value is required")
	}
	value, err := strconv.ParseUint(trimmed, 10, 64)
	if err != nil || value == 0 {
		return 0, errors.New("value must be a positive integer")
	}
	return value, nil
}

// nftFee splits a sale amount into the marketplace fee and the seller's
// proceeds. The fee is rounded down and the remainder goes to the seller, so
// the protocol never takes more than its configured share.
func nftFee(price *big.Int, feeBps int) (fee *big.Int, proceeds *big.Int) {
	if feeBps <= 0 {
		return big.NewInt(0), new(big.Int).Set(price)
	}
	fee = new(big.Int).Mul(price, big.NewInt(int64(feeBps)))
	fee.Div(fee, big.NewInt(10000))
	if fee.Cmp(price) > 0 {
		fee = new(big.Int).Set(price)
	}
	proceeds = new(big.Int).Sub(price, fee)
	return fee, proceeds
}

// nftLoadCollection reads a collection by id.
func (s *NFTMarketplaceService) nftLoadCollection(id uuid.UUID) (*models.NFTCollection, error) {
	return nftLoadCollectionTx(s.db, id)
}

// nftLoadCollectionTx reads a collection using a caller's transaction, so a
// read inside a transaction cannot deadlock on a second connection.
func nftLoadCollectionTx(tx *gorm.DB, id uuid.UUID) (*models.NFTCollection, error) {
	var collection models.NFTCollection
	if err := tx.First(&collection, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errNFTNotFound
		}
		return nil, err
	}
	return &collection, nil
}

// nftLoadToken reads a token by collection id and token id.
func (s *NFTMarketplaceService) nftLoadToken(collectionID uuid.UUID, tokenID uint64) (*models.NFTToken, error) {
	var token models.NFTToken
	if err := s.db.First(&token, "collection_id = ? AND token_id = ?", collectionID, tokenID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errNFTNotFound
		}
		return nil, err
	}
	return &token, nil
}

// ---- Admin: collections ----------------------------------------------------

type createNFTCollectionRequest struct {
	Key             string `json:"key" binding:"required"`
	Name            string `json:"name" binding:"required"`
	Symbol          string `json:"symbol" binding:"required"`
	ChainID         int64  `json:"chain_id" binding:"required"`
	ContractAddress string `json:"contract_address"`
	BaseURI         string `json:"base_uri"`
	Status          string `json:"status"`
}

// CreateCollection creates an admin-managed collection. It starts paused, like
// every other listing, and accepts no mint until an operator activates it.
func (s *NFTMarketplaceService) CreateCollection(c *gin.Context) {
	var req createNFTCollectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}

	key := strings.TrimSpace(req.Key)
	name := strings.TrimSpace(req.Name)
	symbol := strings.ToUpper(strings.TrimSpace(req.Symbol))
	if key == "" || name == "" || symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key, name and symbol are required"})
		return
	}
	if req.ChainID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chain_id must be positive"})
		return
	}

	contractAddress := ""
	if raw := strings.TrimSpace(req.ContractAddress); raw != "" {
		if !common.IsHexAddress(raw) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "contract_address is not a valid address"})
			return
		}
		contractAddress = strings.ToLower(common.HexToAddress(raw).Hex())
	}

	status := models.NFTStatusActive
	if raw := strings.ToLower(strings.TrimSpace(req.Status)); raw != "" {
		if raw != models.NFTStatusActive && raw != "paused" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "status must be active or paused"})
			return
		}
		status = raw
	} else {
		status = "paused"
	}

	var existing models.NFTCollection
	if err := s.db.First(&existing, "key = ?", key).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "a collection with that key already exists"})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check for an existing collection"})
		return
	}

	collection := models.NFTCollection{
		Key:             key,
		Name:            name,
		Symbol:          symbol,
		ChainID:         req.ChainID,
		ContractAddress: contractAddress,
		BaseURI:         strings.TrimSpace(req.BaseURI),
		Status:          status,
		CreatedBy:       c.GetString("user_id"),
	}
	if err := s.db.Create(&collection).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create collection"})
		return
	}
	logListingAction(c, s.db, "create_nft_collection", "nft_collection", collection.ID.String(), gin.H{
		"key": key, "symbol": symbol, "chain_id": collection.ChainID,
	})
	c.JSON(http.StatusCreated, gin.H{"success": true, "collection": collection})
}

// GetCollections lists every collection, active or not. Admin-only.
func (s *NFTMarketplaceService) GetCollections(c *gin.Context) {
	collections := []models.NFTCollection{}
	if err := s.db.Order("created_at desc").Find(&collections).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list collections"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "collections": collections})
}

// GetActiveCollections lists the collections a user may mint in.
func (s *NFTMarketplaceService) GetActiveCollections(c *gin.Context) {
	collections := []models.NFTCollection{}
	if err := s.db.Where("status = ?", models.NFTStatusActive).Order("created_at desc").Find(&collections).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list collections"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "collections": collections})
}

type updateNFTCollectionRequest struct {
	Name            *string `json:"name"`
	BaseURI         *string `json:"base_uri"`
	ContractAddress *string `json:"contract_address"`
	Status          *string `json:"status"`
}

// UpdateCollection edits a collection's metadata or status.
func (s *NFTMarketplaceService) UpdateCollection(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid collection id"})
		return
	}
	var req updateNFTCollectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	collection, err := s.nftLoadCollection(id)
	if err != nil {
		if errors.Is(err, errNFTNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "collection not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load collection"})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name must not be empty"})
			return
		}
		updates["name"] = name
	}
	if req.BaseURI != nil {
		updates["base_uri"] = strings.TrimSpace(*req.BaseURI)
	}
	if req.ContractAddress != nil {
		raw := strings.TrimSpace(*req.ContractAddress)
		if raw != "" {
			if !common.IsHexAddress(raw) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "contract_address is not a valid address"})
				return
			}
			raw = strings.ToLower(common.HexToAddress(raw).Hex())
		}
		updates["contract_address"] = raw
	}
	if req.Status != nil {
		status := strings.ToLower(strings.TrimSpace(*req.Status))
		if status != models.NFTStatusActive && status != "paused" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "status must be active or paused"})
			return
		}
		updates["status"] = status
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}
	if err := s.db.Model(&models.NFTCollection{}).Where("id = ?", collection.ID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update collection"})
		return
	}
	logListingAction(c, s.db, "update_nft_collection", "nft_collection", collection.ID.String(), updates)

	updated, err := s.nftLoadCollection(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reload collection"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "collection": updated})
}

// DeleteCollection refuses while any token exists, so a collection with minted
// tokens is never removed out from under its holders.
func (s *NFTMarketplaceService) DeleteCollection(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid collection id"})
		return
	}
	collection, err := s.nftLoadCollection(id)
	if err != nil {
		if errors.Is(err, errNFTNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "collection not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load collection"})
		return
	}

	var count int64
	if err := s.db.Model(&models.NFTToken{}).Where("collection_id = ?", collection.ID).Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count tokens"})
		return
	}
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "collection has minted tokens and cannot be deleted"})
		return
	}
	if err := s.db.Delete(&models.NFTCollection{}, "id = ?", collection.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete collection"})
		return
	}
	logListingAction(c, s.db, "delete_nft_collection", "nft_collection", collection.ID.String(), nil)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ---- Minting ---------------------------------------------------------------

type mintNFTRequest struct {
	TokenID *uint64 `json:"token_id"`
	URI     string  `json:"uri"`
	Owner   string  `json:"owner"`
}

// Mint issues a new token in a collection. The platform is the only minter,
// matching the contract's `onlyOwner` mint. The token id is taken from the
// collection's own counter so ids are unique and gapless.
func (s *NFTMarketplaceService) Mint(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid collection id"})
		return
	}
	var req mintNFTRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	collection, err := s.nftLoadCollection(id)
	if err != nil {
		if errors.Is(err, errNFTNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "collection not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load collection"})
		return
	}
	if collection.Status != models.NFTStatusActive {
		c.JSON(http.StatusConflict, gin.H{"error": "collection is not active"})
		return
	}
	uri := strings.TrimSpace(req.URI)
	if uri == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uri is required"})
		return
	}

	// The recipient defaults to the admin minting; an explicit owner lets an
	// operator mint into a user's account for an airdrop or a primary sale.
	owner := strings.TrimSpace(req.Owner)
	if owner == "" {
		owner = c.GetString("user_id")
	}
	if owner == "" || owner == nftEscrowOwner {
		c.JSON(http.StatusBadRequest, gin.H{"error": "owner is required"})
		return
	}

	token := models.NFTToken{}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var locked models.NFTCollection
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, "id = ?", collection.ID).Error; err != nil {
			return err
		}
		if locked.Status != models.NFTStatusActive {
			return errNFTCollectionOff
		}
		tokenID := uint64(locked.TotalSupply) + 1
		if req.TokenID != nil {
			// An explicit id is allowed only when it does not collide and does not
			// move the counter backwards.
			if *req.TokenID == 0 {
				return errors.New("token_id must be positive")
			}
			var existing models.NFTToken
			err := tx.First(&existing, "collection_id = ? AND token_id = ?", locked.ID, *req.TokenID).Error
			if err == nil {
				return errors.New("token_id already exists in this collection")
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			tokenID = *req.TokenID
		}
		token = models.NFTToken{
			CollectionID: locked.ID,
			TokenID:      tokenID,
			OwnerUserID:  owner,
			URI:          uri,
			MintedBy:     c.GetString("user_id"),
		}
		if err := tx.Create(&token).Error; err != nil {
			return err
		}
		next := locked.TotalSupply
		if int64(tokenID) > next {
			next = int64(tokenID)
		}
		return tx.Model(&models.NFTCollection{}).Where("id = ?", locked.ID).
			Update("total_supply", next).Error
	})
	if err != nil {
		switch {
		case errors.Is(err, errNFTCollectionOff):
			c.JSON(http.StatusConflict, gin.H{"error": "collection is not active"})
		default:
			// A failure here is a database or uniqueness error whose text can
			// carry a SQL fragment; log it and answer generically.
			log.Printf("nft: mint failed: %v", err)
			c.JSON(http.StatusConflict, gin.H{"error": "failed to mint token"})
		}
		return
	}
	logListingAction(c, s.db, "mint_nft", "nft_token", token.ID.String(), gin.H{
		"collection_id": collection.ID, "token_id": token.TokenID, "owner": owner,
	})
	c.JSON(http.StatusCreated, gin.H{"success": true, "token": token})
}

// ---- Listings --------------------------------------------------------------

// nftPaymentToken resolves a payment token from its address and symbol. The
// address is the ledger key; the symbol selects the decimals and is checked
// against the supported set.
func nftPaymentToken(address, symbol string) (string, string, int, error) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if !models.IsSupportedNFTPayment(sym) {
		return "", "", 0, errNFTUnsupportedPay
	}
	key, err := tokenKeyOrNative(address)
	if err != nil {
		return "", "", 0, err
	}
	return key, sym, 18, nil
}

type createNFTListingRequest struct {
	CollectionID         string `json:"collection_id" binding:"required"`
	TokenID              uint64 `json:"token_id" binding:"required"`
	PaymentTokenAddress  string `json:"payment_token_address"`
	PaymentTokenSymbol   string `json:"payment_token_symbol" binding:"required"`
	PaymentTokenDecimals *int   `json:"payment_token_decimals"`
	Price                string `json:"price" binding:"required"`
	Deadline             string `json:"deadline" binding:"required"`
}

// ListNFT escrows a token and opens a fixed-price listing. The token's owner is
// moved to the escrow sentinel in the same transaction that creates the
// listing, so a live listing is always fillable.
func (s *NFTMarketplaceService) ListNFT(c *gin.Context) {
	var req createNFTListingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	collectionID, err := uuid.Parse(strings.TrimSpace(req.CollectionID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid collection_id"})
		return
	}
	if req.TokenID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token_id must be positive"})
		return
	}
	price, err := parseTokenAmount(req.Price)
	if err != nil || price.Sign() <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "price must be a positive integer"})
		return
	}
	deadline, err := parseListingTime(req.Deadline)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	if !deadline.After(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deadline must be in the future"})
		return
	}
	payKey, paySymbol, payDecimals, err := nftPaymentToken(req.PaymentTokenAddress, req.PaymentTokenSymbol)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	if req.PaymentTokenDecimals != nil {
		if *req.PaymentTokenDecimals < 0 || *req.PaymentTokenDecimals > 36 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "payment_token_decimals must be between 0 and 36"})
			return
		}
		payDecimals = *req.PaymentTokenDecimals
	}

	collection, err := s.nftLoadCollection(collectionID)
	if err != nil {
		if errors.Is(err, errNFTNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "collection not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load collection"})
		return
	}
	if collection.Status != models.NFTStatusActive {
		c.JSON(http.StatusConflict, gin.H{"error": "collection is not active"})
		return
	}

	listing := models.NFTListing{}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var token models.NFTToken
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&token, "collection_id = ? AND token_id = ?", collection.ID, req.TokenID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errNFTNotFound
			}
			return err
		}
		if token.OwnerUserID != userID {
			return errNFTNotOwner
		}
		if token.EscrowListingID != nil {
			return errNFTInEscrow
		}

		listing = models.NFTListing{
			CollectionID:         collection.ID,
			TokenID:              token.TokenID,
			SellerUserID:         userID,
			PaymentTokenAddress:  payKey,
			PaymentTokenSymbol:   paySymbol,
			PaymentTokenDecimals: payDecimals,
			Price:                price.String(),
			FeeBps:               s.feeBps,
			Deadline:             deadline,
			Status:               models.NFTStatusActive,
		}
		if err := tx.Create(&listing).Error; err != nil {
			return err
		}
		// Escrow: move the token to the sentinel holder and record the listing.
		return tx.Model(&models.NFTToken{}).Where("id = ?", token.ID).Updates(map[string]interface{}{
			"owner_user_id":     nftEscrowOwner,
			"escrow_listing_id": listing.ID,
		}).Error
	})
	if err != nil {
		switch {
		case errors.Is(err, errNFTNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "token not found"})
		case errors.Is(err, errNFTNotOwner):
			c.JSON(http.StatusForbidden, gin.H{"error": "you do not own this token"})
		case errors.Is(err, errNFTInEscrow):
			c.JSON(http.StatusConflict, gin.H{"error": "token is escrowed by a live listing"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create listing"})
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "listing": listing})
}

// BuyNFT fills a listing: the buyer pays, the seller is credited net of fee,
// and the escrowed token moves to the buyer. State is cleared under lock before
// any credit, so a double-submit cannot buy the same listing twice.
func (s *NFTMarketplaceService) BuyNFT(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid listing id"})
		return
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	if s.ledger == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ledger is not configured"})
		return
	}

	var listing models.NFTListing
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&listing, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errNFTNotFound
			}
			return err
		}
		if listing.Status != models.NFTStatusActive {
			return errNFTNotActive
		}
		if !time.Now().Before(listing.Deadline) {
			return errNFTExpired
		}
		if listing.SellerUserID == userID {
			return errNFTSelfTrade
		}
		price, ok := new(big.Int).SetString(listing.Price, 10)
		if !ok || price.Sign() <= 0 {
			return errors.New("listing has a malformed price")
		}
		fee, proceeds := nftFee(price, listing.FeeBps)
		if fee.Cmp(price) > 0 {
			return errNFTFeeExceedsPrice
		}

		// The collection carries the chain the payment settles on; it is read
		// inside the transaction so a deleted collection fails the sale rather
		// than moving funds on an unknown chain.
		var collection models.NFTCollection
		if err := tx.First(&collection, "id = ?", listing.CollectionID).Error; err != nil {
			return err
		}

		// Mark sold before any ledger movement, so a retry cannot fill it again.
		now := time.Now()
		buyer := userID
		if err := tx.Model(&models.NFTListing{}).Where("id = ?", listing.ID).Updates(map[string]interface{}{
			"status":        models.NFTStatusSold,
			"buyer_user_id": buyer,
			"sold_at":       now,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.NFTToken{}).
			Where("collection_id = ? AND token_id = ?", listing.CollectionID, listing.TokenID).
			Updates(map[string]interface{}{
				"owner_user_id":     buyer,
				"escrow_listing_id": nil,
			}).Error; err != nil {
			return err
		}

		if err := s.ledger.debitWithin(tx, buyer, collection.ChainID, listing.PaymentTokenAddress, price); err != nil {
			return err
		}
		if err := s.ledger.creditWithin(tx, listing.SellerUserID, collection.ChainID, listing.PaymentTokenAddress,
			listing.PaymentTokenSymbol, listing.PaymentTokenDecimals, proceeds); err != nil {
			return err
		}
		if fee.Sign() > 0 {
			return s.ledger.creditWithin(tx, s.feeAccount, collection.ChainID, listing.PaymentTokenAddress,
				listing.PaymentTokenSymbol, listing.PaymentTokenDecimals, fee)
		}
		return nil
	})
	if err != nil {
		switch {
		case errors.Is(err, errNFTNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "listing not found"})
		case errors.Is(err, errNFTNotActive):
			c.JSON(http.StatusConflict, gin.H{"error": "listing is not active"})
		case errors.Is(err, errNFTExpired):
			c.JSON(http.StatusConflict, gin.H{"error": "listing has expired"})
		case errors.Is(err, errNFTSelfTrade):
			c.JSON(http.StatusConflict, gin.H{"error": "you cannot buy your own listing"})
		case errors.Is(err, ErrInsufficientBalance):
			c.JSON(http.StatusConflict, gin.H{"error": "insufficient balance to buy this listing"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to buy listing"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "listing": listing})
}

// CancelListing returns an escrowed token to its seller. The seller may cancel
// whether the listing is live or expired.
func (s *NFTMarketplaceService) CancelListing(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid listing id"})
		return
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var listing models.NFTListing
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&listing, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errNFTNotFound
			}
			return err
		}
		if listing.Status != models.NFTStatusActive {
			return errNFTNotActive
		}
		if listing.SellerUserID != userID {
			return errNFTNotOwner
		}
		if err := tx.Model(&models.NFTListing{}).Where("id = ?", listing.ID).
			Update("status", models.NFTStatusCanceled).Error; err != nil {
			return err
		}
		return tx.Model(&models.NFTToken{}).
			Where("collection_id = ? AND token_id = ?", listing.CollectionID, listing.TokenID).
			Updates(map[string]interface{}{
				"owner_user_id":     listing.SellerUserID,
				"escrow_listing_id": nil,
			}).Error
	})
	if err != nil {
		switch {
		case errors.Is(err, errNFTNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "listing not found"})
		case errors.Is(err, errNFTNotActive):
			c.JSON(http.StatusConflict, gin.H{"error": "listing is not active"})
		case errors.Is(err, errNFTNotOwner):
			c.JSON(http.StatusForbidden, gin.H{"error": "only the seller can cancel this listing"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to cancel listing"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

type updateNFTListingRequest struct {
	Price    *string `json:"price"`
	Deadline *string `json:"deadline"`
}

// UpdateListingPrice edits the asking price of a live listing. Only the seller
// may do it, and the change keeps the frozen fee, so a price edit cannot move
// the fee split.
func (s *NFTMarketplaceService) UpdateListingPrice(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid listing id"})
		return
	}
	var req updateNFTListingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	var listing models.NFTListing
	if err := s.db.First(&listing, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "listing not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load listing"})
		return
	}
	if listing.Status != models.NFTStatusActive {
		c.JSON(http.StatusConflict, gin.H{"error": "listing is not active"})
		return
	}
	if listing.SellerUserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "only the seller can edit this listing"})
		return
	}
	updates := map[string]interface{}{}
	if req.Price != nil {
		price, err := parseTokenAmount(*req.Price)
		if err != nil || price.Sign() <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "price must be a positive integer"})
			return
		}
		updates["price"] = price.String()
	}
	if req.Deadline != nil {
		deadline, err := parseListingTime(*req.Deadline)
		if err != nil {
			respondBadRequestError(c, err)
			return
		}
		if !deadline.After(time.Now()) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "deadline must be in the future"})
			return
		}
		updates["deadline"] = deadline
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}
	if err := s.db.Model(&models.NFTListing{}).Where("id = ?", listing.ID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update listing"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ---- Offers ----------------------------------------------------------------

type createNFTOfferRequest struct {
	CollectionID         string `json:"collection_id" binding:"required"`
	TokenID              uint64 `json:"token_id" binding:"required"`
	PaymentTokenAddress  string `json:"payment_token_address"`
	PaymentTokenSymbol   string `json:"payment_token_symbol" binding:"required"`
	PaymentTokenDecimals *int   `json:"payment_token_decimals"`
	Amount               string `json:"amount" binding:"required"`
	Deadline             string `json:"deadline" binding:"required"`
}

// MakeOffer escrows a buyer's payment and records a standing bid. The payment
// is debited at offer time, so an accepted offer can never fail for lack of
// funds; a cancel refunds the exact escrowed amount.
func (s *NFTMarketplaceService) MakeOffer(c *gin.Context) {
	var req createNFTOfferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	if s.ledger == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ledger is not configured"})
		return
	}
	collectionID, err := uuid.Parse(strings.TrimSpace(req.CollectionID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid collection_id"})
		return
	}
	if req.TokenID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token_id must be positive"})
		return
	}
	amount, err := parseTokenAmount(req.Amount)
	if err != nil || amount.Sign() <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be a positive integer"})
		return
	}
	deadline, err := parseListingTime(req.Deadline)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	if !deadline.After(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deadline must be in the future"})
		return
	}
	payKey, paySymbol, payDecimals, err := nftPaymentToken(req.PaymentTokenAddress, req.PaymentTokenSymbol)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}
	if req.PaymentTokenDecimals != nil {
		if *req.PaymentTokenDecimals < 0 || *req.PaymentTokenDecimals > 36 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "payment_token_decimals must be between 0 and 36"})
			return
		}
		payDecimals = *req.PaymentTokenDecimals
	}

	collection, err := s.nftLoadCollection(collectionID)
	if err != nil {
		if errors.Is(err, errNFTNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "collection not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load collection"})
		return
	}

	offer := models.NFTOffer{}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var token models.NFTToken
		if err := tx.First(&token, "collection_id = ? AND token_id = ?", collection.ID, req.TokenID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errNFTNotFound
			}
			return err
		}
		// An offer on a token held in escrow is still valid: the seller can
		// accept it once the listing is cancelled, and the funds are held until
		// then or refunded on cancel.
		if token.OwnerUserID == userID {
			return errNFTSelfTrade
		}
		offer = models.NFTOffer{
			CollectionID:         collection.ID,
			TokenID:              token.TokenID,
			BuyerUserID:          userID,
			PaymentTokenAddress:  payKey,
			PaymentTokenSymbol:   paySymbol,
			PaymentTokenDecimals: payDecimals,
			Amount:               amount.String(),
			FeeBps:               s.feeBps,
			Deadline:             deadline,
			Status:               models.NFTStatusActive,
		}
		if err := tx.Create(&offer).Error; err != nil {
			return err
		}
		return s.ledger.debitWithin(tx, userID, collection.ChainID, payKey, amount)
	})
	if err != nil {
		switch {
		case errors.Is(err, errNFTNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "token not found"})
		case errors.Is(err, errNFTSelfTrade):
			c.JSON(http.StatusConflict, gin.H{"error": "you cannot bid on your own token"})
		case errors.Is(err, ErrInsufficientBalance):
			c.JSON(http.StatusConflict, gin.H{"error": "insufficient balance to escrow this offer"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to make offer"})
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "offer": offer})
}

// AcceptOffer accepts a standing bid on a token the caller owns. The seller is
// credited net of fee and the escrowed payment returns to the buyer as the NFT.
func (s *NFTMarketplaceService) AcceptOffer(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid offer id"})
		return
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	if s.ledger == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ledger is not configured"})
		return
	}

	collectionID, err := uuid.Parse(strings.TrimSpace(c.Query("collection_id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "collection_id query parameter is required"})
		return
	}
	tokenID, err := parseUint64Query(c.Query("token_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token_id query parameter is required"})
		return
	}

	var offer models.NFTOffer
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&offer, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errNFTNotFound
			}
			return err
		}
		if offer.Status != models.NFTStatusActive {
			return errNFTNoBid
		}
		if offer.CollectionID != collectionID || offer.TokenID != tokenID {
			return errNFTNotFound
		}
		if !time.Now().Before(offer.Deadline) {
			return errNFTExpired
		}

		var token models.NFTToken
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&token, "collection_id = ? AND token_id = ?", offer.CollectionID, offer.TokenID).Error; err != nil {
			return err
		}
		// Escrow is checked before ownership: a token held by a live listing is
		// owned by the escrow sentinel, so the seller must cancel that listing
		// first. Reporting the escrow conflict is the actionable error.
		if token.EscrowListingID != nil {
			return errNFTInEscrow
		}
		if token.OwnerUserID != userID {
			return errNFTNotOwner
		}

		amount, ok := new(big.Int).SetString(offer.Amount, 10)
		if !ok || amount.Sign() <= 0 {
			return errors.New("offer has a malformed amount")
		}
		fee, proceeds := nftFee(amount, offer.FeeBps)

		collection, err := nftLoadCollectionTx(tx, offer.CollectionID)
		if err != nil {
			return err
		}

		if err := tx.Model(&models.NFTOffer{}).Where("id = ?", offer.ID).
			Update("status", models.NFTStatusSold).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.NFTToken{}).Where("id = ?", token.ID).
			Update("owner_user_id", offer.BuyerUserID).Error; err != nil {
			return err
		}
		if err := s.ledger.creditWithin(tx, userID, collection.ChainID, offer.PaymentTokenAddress,
			offer.PaymentTokenSymbol, offer.PaymentTokenDecimals, proceeds); err != nil {
			return err
		}
		if fee.Sign() > 0 {
			return s.ledger.creditWithin(tx, s.feeAccount, collection.ChainID, offer.PaymentTokenAddress,
				offer.PaymentTokenSymbol, offer.PaymentTokenDecimals, fee)
		}
		return nil
	})
	if err != nil {
		switch {
		case errors.Is(err, errNFTNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "offer not found"})
		case errors.Is(err, errNFTNoBid):
			c.JSON(http.StatusConflict, gin.H{"error": "offer is not active"})
		case errors.Is(err, errNFTExpired):
			c.JSON(http.StatusConflict, gin.H{"error": "offer has expired"})
		case errors.Is(err, errNFTNotOwner):
			c.JSON(http.StatusForbidden, gin.H{"error": "you do not own this token"})
		case errors.Is(err, errNFTInEscrow):
			c.JSON(http.StatusConflict, gin.H{"error": "cancel the live listing before accepting an offer"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to accept offer"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "offer": offer})
}

// CancelOffer refunds the escrowed amount to the buyer.
func (s *NFTMarketplaceService) CancelOffer(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid offer id"})
		return
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	if s.ledger == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ledger is not configured"})
		return
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var offer models.NFTOffer
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&offer, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errNFTNotFound
			}
			return err
		}
		if offer.Status != models.NFTStatusActive {
			return errNFTNoBid
		}
		if offer.BuyerUserID != userID {
			return errNFTNotOwner
		}
		amount, ok := new(big.Int).SetString(offer.Amount, 10)
		if !ok || amount.Sign() <= 0 {
			return errors.New("offer has a malformed amount")
		}
		collection, err := nftLoadCollectionTx(tx, offer.CollectionID)
		if err != nil {
			return err
		}
		if err := tx.Model(&models.NFTOffer{}).Where("id = ?", offer.ID).
			Update("status", models.NFTStatusCanceled).Error; err != nil {
			return err
		}
		return s.ledger.creditWithin(tx, userID, collection.ChainID, offer.PaymentTokenAddress,
			offer.PaymentTokenSymbol, offer.PaymentTokenDecimals, amount)
	})
	if err != nil {
		switch {
		case errors.Is(err, errNFTNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "offer not found"})
		case errors.Is(err, errNFTNoBid):
			c.JSON(http.StatusConflict, gin.H{"error": "offer is not active"})
		case errors.Is(err, errNFTNotOwner):
			c.JSON(http.StatusForbidden, gin.H{"error": "only the buyer can cancel this offer"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to cancel offer"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ---- Reads -----------------------------------------------------------------

// NFTListingView decorates a listing with the token it sells and its
// collection, so the UI has everything it needs in one response.
type NFTListingView struct {
	Listing    models.NFTListing    `json:"listing"`
	Token      models.NFTToken      `json:"token"`
	Collection models.NFTCollection `json:"collection"`
	Active     bool                 `json:"active"`
}

// NFTOfferView decorates an offer with its token and collection.
type NFTOfferView struct {
	Offer      models.NFTOffer      `json:"offer"`
	Token      models.NFTToken      `json:"token"`
	Collection models.NFTCollection `json:"collection"`
	Active     bool                 `json:"active"`
}

func (s *NFTMarketplaceService) buildListingView(listing models.NFTListing) NFTListingView {
	view := NFTListingView{Listing: listing, Active: listing.Status == models.NFTStatusActive && time.Now().Before(listing.Deadline)}
	token, err := s.nftLoadToken(listing.CollectionID, listing.TokenID)
	if err == nil {
		view.Token = *token
	}
	collection, err := s.nftLoadCollection(listing.CollectionID)
	if err == nil {
		view.Collection = *collection
	}
	return view
}

func (s *NFTMarketplaceService) buildListingViews(listings []models.NFTListing) []NFTListingView {
	views := make([]NFTListingView, 0, len(listings))
	for _, listing := range listings {
		views = append(views, s.buildListingView(listing))
	}
	return views
}

func (s *NFTMarketplaceService) buildOfferView(offer models.NFTOffer) NFTOfferView {
	view := NFTOfferView{Offer: offer, Active: offer.Status == models.NFTStatusActive && time.Now().Before(offer.Deadline)}
	token, err := s.nftLoadToken(offer.CollectionID, offer.TokenID)
	if err == nil {
		view.Token = *token
	}
	collection, err := s.nftLoadCollection(offer.CollectionID)
	if err == nil {
		view.Collection = *collection
	}
	return view
}

func (s *NFTMarketplaceService) buildOfferViews(offers []models.NFTOffer) []NFTOfferView {
	views := make([]NFTOfferView, 0, len(offers))
	for _, offer := range offers {
		views = append(views, s.buildOfferView(offer))
	}
	return views
}

// GetActiveListings lists every live listing across all collections.
func (s *NFTMarketplaceService) GetActiveListings(c *gin.Context) {
	listings := []models.NFTListing{}
	if err := s.db.Where("status = ? AND deadline > ?", models.NFTStatusActive, time.Now()).
		Order("created_at desc").Find(&listings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list listings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "listings": s.buildListingViews(listings)})
}

// GetListings lists every listing, live or not. Admin-only.
func (s *NFTMarketplaceService) GetListings(c *gin.Context) {
	listings := []models.NFTListing{}
	if err := s.db.Order("created_at desc").Find(&listings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list listings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "listings": s.buildListingViews(listings)})
}

// GetMyNFTs lists the tokens the caller owns.
func (s *NFTMarketplaceService) GetMyNFTs(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	tokens := []models.NFTToken{}
	if err := s.db.Where("owner_user_id = ?", userID).Order("created_at desc").Find(&tokens).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list your tokens"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "tokens": tokens})
}

// GetMyListings lists the listings the caller created.
func (s *NFTMarketplaceService) GetMyListings(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	listings := []models.NFTListing{}
	if err := s.db.Where("seller_user_id = ?", userID).Order("created_at desc").Find(&listings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list your listings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "listings": s.buildListingViews(listings)})
}

// GetMyOffers lists the offers the caller has made.
func (s *NFTMarketplaceService) GetMyOffers(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	offers := []models.NFTOffer{}
	if err := s.db.Where("buyer_user_id = ?", userID).Order("created_at desc").Find(&offers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list your offers"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "offers": s.buildOfferViews(offers)})
}

// GetOffers lists the active offers on a token. Public.
func (s *NFTMarketplaceService) GetOffers(c *gin.Context) {
	collectionID, err := uuid.Parse(strings.TrimSpace(c.Query("collection_id")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "collection_id query parameter is required"})
		return
	}
	tokenID, err := parseUint64Query(c.Query("token_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token_id query parameter is required"})
		return
	}
	offers := []models.NFTOffer{}
	if err := s.db.Where("collection_id = ? AND token_id = ? AND status = ? AND deadline > ?",
		collectionID, tokenID, models.NFTStatusActive, time.Now()).
		Order("created_at desc").Find(&offers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list offers"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "offers": s.buildOfferViews(offers)})
}

// GetMarketplaceConfig reports the current fee configuration. Public.
func (s *NFTMarketplaceService) GetMarketplaceConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"fee_bps":        s.feeBps,
		"max_fee_bps":    nftMaxFeeBps,
		"fee_account":    s.feeAccount,
		"escrow_account": nftEscrowOwner,
	})
}

type updateNFTConfigRequest struct {
	FeeBps     *int    `json:"fee_bps"`
	FeeAccount *string `json:"fee_account"`
}

// UpdateMarketplaceConfig sets the fee and fee account. Admin-only.
func (s *NFTMarketplaceService) UpdateMarketplaceConfig(c *gin.Context) {
	var req updateNFTConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	if req.FeeBps == nil && req.FeeAccount == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}
	if req.FeeBps != nil {
		if err := s.SetFeeBps(*req.FeeBps); err != nil {
			respondBadRequestError(c, err)
			return
		}
	}
	if req.FeeAccount != nil {
		s.SetFeeAccount(*req.FeeAccount)
	}
	logListingAction(c, s.db, "update_nft_config", "nft_config", "default", gin.H{
		"fee_bps": s.feeBps, "fee_account": s.feeAccount,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "fee_bps": s.feeBps, "fee_account": s.feeAccount})
}
