package services

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"tigerswap/backend/models"
)

var nftTestDBSeq uint64

// nftTestEnv bundles the database, ledger and NFT marketplace under test.
type nftTestEnv struct {
	db     *gorm.DB
	ledger *LedgerService
	engine *NFTMarketplaceService
}

func newNFTTestEnv(t *testing.T) *nftTestEnv {
	t.Helper()
	dsn := fmt.Sprintf("file:nft-%s-%d?mode=memory&cache=shared", t.Name(), atomic.AddUint64(&nftTestDBSeq, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })

	if err := db.AutoMigrate(
		&models.NFTCollection{}, &models.NFTToken{}, &models.NFTListing{}, &models.NFTOffer{},
		&models.AccountBalance{}, &models.AdminLog{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ledger := NewLedgerService(db)
	return &nftTestEnv{db: db, ledger: ledger, engine: NewNFTMarketplaceService(db, ledger)}
}

// nftPaymentAddress is a valid hex address used as the payment token in tests.
const nftPaymentAddress = "0x4444444444444444444444444444444444444444"

// nftCollectionID seeds an active collection through the admin handler and
// returns its id.
func nftCollectionID(t *testing.T, env *nftTestEnv) string {
	t.Helper()
	body := fmt.Sprintf(`{
		"key":"tiger-art",
		"name":"Tiger Art",
		"symbol":"TART",
		"chain_id":1,
		"contract_address":"0x5555555555555555555555555555555555555555",
		"base_uri":"https://cdn.tigerswap.example/nft/",
		"status":"active"
	}`)
	status, response := callUserHandler(t, env.engine.CreateCollection, "admin-1", http.MethodPost, "/admin/nft/collections", body, nil)
	if status != http.StatusCreated {
		t.Fatalf("create collection: status %d body %s", status, response)
	}
	var parsed struct {
		Collection models.NFTCollection `json:"collection"`
	}
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		t.Fatalf("decode collection: %v", err)
	}
	return parsed.Collection.ID.String()
}

// nftMint mints a token into `owner` and returns its token id.
func nftMint(t *testing.T, env *nftTestEnv, collectionID, owner string) uint64 {
	t.Helper()
	body := fmt.Sprintf(`{"uri":"token.json","owner":%q}`, owner)
	status, response := callUserHandler(t, env.engine.Mint, "admin-1", http.MethodPost,
		"/admin/nft/collections/"+collectionID+"/mint", body, gin.Params{{Key: "id", Value: collectionID}})
	if status != http.StatusCreated {
		t.Fatalf("mint: status %d body %s", status, response)
	}
	var parsed struct {
		Token models.NFTToken `json:"token"`
	}
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		t.Fatalf("decode token: %v", err)
	}
	return parsed.Token.TokenID
}

// fund gives a user a real ledger balance so a buy or an offer has funds.
func fund(t *testing.T, env *nftTestEnv, userID string, amount int64) {
	t.Helper()
	if err := env.ledger.Credit(userID, 1, nftPaymentAddress, "RUSD", 18, big.NewInt(amount)); err != nil {
		t.Fatalf("fund %s: %v", userID, err)
	}
}

func nftBalance(t *testing.T, env *nftTestEnv, userID string) *big.Int {
	t.Helper()
	bal, err := env.ledger.Available(userID, 1, nftPaymentAddress)
	if err != nil {
		t.Fatalf("balance %s: %v", userID, err)
	}
	return bal
}

// listNFT opens a listing as `seller` and returns the listing id.
func listNFT(t *testing.T, env *nftTestEnv, collectionID string, tokenID uint64, seller, price string) string {
	t.Helper()
	body := fmt.Sprintf(`{
		"collection_id":%q,
		"token_id":%d,
		"payment_token_address":%q,
		"payment_token_symbol":"RUSD",
		"price":%q,
		"deadline":%q
	}`, collectionID, tokenID, nftPaymentAddress, price, time.Now().Add(24*time.Hour).Format(time.RFC3339))
	status, response := callUserHandler(t, env.engine.ListNFT, seller, http.MethodPost, "/nft/listings", body, nil)
	if status != http.StatusCreated {
		t.Fatalf("list: status %d body %s", status, response)
	}
	var parsed struct {
		Listing models.NFTListing `json:"listing"`
	}
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		t.Fatalf("decode listing: %v", err)
	}
	return parsed.Listing.ID.String()
}

// TestNFTBuySettlesEscrowAndFee checks the full sale path: the buyer pays, the
// seller receives the price net of the marketplace fee, the fee account is
// credited, and the escrowed token moves to the buyer.
func TestNFTBuySettlesEscrowAndFee(t *testing.T) {
	env := newNFTTestEnv(t)
	collectionID := nftCollectionID(t, env)
	tokenID := nftMint(t, env, collectionID, "seller-1")

	fund(t, env, "buyer-1", 1_000_000)
	listingID := listNFT(t, env, collectionID, tokenID, "seller-1", "100000")

	// The token is escrowed: the seller no longer owns it outright.
	var token models.NFTToken
	if err := env.db.First(&token, "collection_id = ? AND token_id = ?", collectionID, tokenID).Error; err != nil {
		t.Fatalf("load token: %v", err)
	}
	if token.OwnerUserID != nftEscrowOwner {
		t.Fatalf("expected escrow owner, got %q", token.OwnerUserID)
	}
	if token.EscrowListingID == nil {
		t.Fatalf("expected escrow listing id to be set")
	}

	status, response := callUserHandler(t, env.engine.BuyNFT, "buyer-1", http.MethodPost,
		"/nft/listings/"+listingID+"/buy", "", gin.Params{{Key: "id", Value: listingID}})
	if status != http.StatusOK {
		t.Fatalf("buy: status %d body %s", status, response)
	}

	// Fee is 2.5% of 100000 = 2500; seller proceeds 97500.
	if got := nftBalance(t, env, "buyer-1"); got.String() != "900000" {
		t.Fatalf("buyer balance = %s, want 900000", got)
	}
	if got := nftBalance(t, env, "seller-1"); got.String() != "97500" {
		t.Fatalf("seller balance = %s, want 97500", got)
	}
	if got := nftBalance(t, env, nftDefaultFeeAccount); got.String() != "2500" {
		t.Fatalf("fee account = %s, want 2500", got)
	}

	// The token now belongs to the buyer and is no longer escrowed. A fresh
	// struct is used because a reused one keeps the stale pointer.
	var bought models.NFTToken
	if err := env.db.First(&bought, "collection_id = ? AND token_id = ?", collectionID, tokenID).Error; err != nil {
		t.Fatalf("reload token: %v", err)
	}
	if bought.OwnerUserID != "buyer-1" {
		t.Fatalf("token owner = %q, want buyer-1", bought.OwnerUserID)
	}
	if bought.EscrowListingID != nil {
		t.Fatalf("expected escrow to be cleared")
	}
}

// TestNFTBuyCannotBeReplayed checks that a second buy of the same listing is
// rejected, so a listing can never pay the seller twice.
func TestNFTBuyCannotBeReplayed(t *testing.T) {
	env := newNFTTestEnv(t)
	collectionID := nftCollectionID(t, env)
	tokenID := nftMint(t, env, collectionID, "seller-1")
	fund(t, env, "buyer-1", 1_000_000)
	fund(t, env, "buyer-2", 1_000_000)
	listingID := listNFT(t, env, collectionID, tokenID, "seller-1", "100000")

	if status, body := callUserHandler(t, env.engine.BuyNFT, "buyer-1", http.MethodPost,
		"/nft/listings/"+listingID+"/buy", "", gin.Params{{Key: "id", Value: listingID}}); status != http.StatusOK {
		t.Fatalf("first buy: status %d body %s", status, body)
	}
	status, body := callUserHandler(t, env.engine.BuyNFT, "buyer-2", http.MethodPost,
		"/nft/listings/"+listingID+"/buy", "", gin.Params{{Key: "id", Value: listingID}})
	if status != http.StatusConflict {
		t.Fatalf("second buy: status %d body %s, want 409", status, body)
	}
	if got := nftBalance(t, env, "seller-1"); got.String() != "97500" {
		t.Fatalf("seller balance = %s, want 97500 (paid once)", got)
	}
}

// TestNFTSellerCannotBuyOwnListing checks the self-trade guard.
func TestNFTSellerCannotBuyOwnListing(t *testing.T) {
	env := newNFTTestEnv(t)
	collectionID := nftCollectionID(t, env)
	tokenID := nftMint(t, env, collectionID, "seller-1")
	fund(t, env, "seller-1", 1_000_000)
	listingID := listNFT(t, env, collectionID, tokenID, "seller-1", "100000")

	status, body := callUserHandler(t, env.engine.BuyNFT, "seller-1", http.MethodPost,
		"/nft/listings/"+listingID+"/buy", "", gin.Params{{Key: "id", Value: listingID}})
	if status != http.StatusConflict {
		t.Fatalf("self buy: status %d body %s, want 409", status, body)
	}
}

// TestNFTInsufficientBalanceRejected checks that a buyer without funds is
// rejected and the token stays escrowed for a real buyer.
func TestNFTInsufficientBalanceRejected(t *testing.T) {
	env := newNFTTestEnv(t)
	collectionID := nftCollectionID(t, env)
	tokenID := nftMint(t, env, collectionID, "seller-1")
	listingID := listNFT(t, env, collectionID, tokenID, "seller-1", "100000")

	status, body := callUserHandler(t, env.engine.BuyNFT, "buyer-1", http.MethodPost,
		"/nft/listings/"+listingID+"/buy", "", gin.Params{{Key: "id", Value: listingID}})
	if status != http.StatusConflict {
		t.Fatalf("buy with no funds: status %d body %s, want 409", status, body)
	}
	var token models.NFTToken
	if err := env.db.First(&token, "collection_id = ? AND token_id = ?", collectionID, tokenID).Error; err != nil {
		t.Fatalf("load token: %v", err)
	}
	if token.OwnerUserID != nftEscrowOwner {
		t.Fatalf("token should remain escrowed, owner = %q", token.OwnerUserID)
	}
}

// TestNFTCancelListingReturnsToken checks that cancelling returns the escrowed
// token to the seller and frees it for a later listing.
func TestNFTCancelListingReturnsToken(t *testing.T) {
	env := newNFTTestEnv(t)
	collectionID := nftCollectionID(t, env)
	tokenID := nftMint(t, env, collectionID, "seller-1")
	listingID := listNFT(t, env, collectionID, tokenID, "seller-1", "100000")

	status, body := callUserHandler(t, env.engine.CancelListing, "seller-1", http.MethodPost,
		"/nft/listings/"+listingID+"/cancel", "", gin.Params{{Key: "id", Value: listingID}})
	if status != http.StatusOK {
		t.Fatalf("cancel: status %d body %s", status, body)
	}
	var token models.NFTToken
	if err := env.db.First(&token, "collection_id = ? AND token_id = ?", collectionID, tokenID).Error; err != nil {
		t.Fatalf("load token: %v", err)
	}
	if token.OwnerUserID != "seller-1" {
		t.Fatalf("token owner = %q, want seller-1", token.OwnerUserID)
	}
	if token.EscrowListingID != nil {
		t.Fatalf("expected escrow to be cleared after cancel")
	}

	// The token can be listed again.
	listNFT(t, env, collectionID, tokenID, "seller-1", "200000")
}

// TestNFTNonSellerCannotCancel checks that a stranger cannot cancel a listing.
func TestNFTNonSellerCannotCancel(t *testing.T) {
	env := newNFTTestEnv(t)
	collectionID := nftCollectionID(t, env)
	tokenID := nftMint(t, env, collectionID, "seller-1")
	listingID := listNFT(t, env, collectionID, tokenID, "seller-1", "100000")

	status, body := callUserHandler(t, env.engine.CancelListing, "stranger-1", http.MethodPost,
		"/nft/listings/"+listingID+"/cancel", "", gin.Params{{Key: "id", Value: listingID}})
	if status != http.StatusForbidden {
		t.Fatalf("stranger cancel: status %d body %s, want 403", status, body)
	}
}

// TestNFTOfferEscrowsAndAcceptPaysSeller checks the offer path end to end: the
// buyer's funds are escrowed on the offer, and accepting pays the seller net of
// fee and moves the token.
func TestNFTOfferEscrowsAndAcceptPaysSeller(t *testing.T) {
	env := newNFTTestEnv(t)
	collectionID := nftCollectionID(t, env)
	tokenID := nftMint(t, env, collectionID, "seller-1")
	fund(t, env, "buyer-1", 1_000_000)

	body := fmt.Sprintf(`{
		"collection_id":%q,
		"token_id":%d,
		"payment_token_address":%q,
		"payment_token_symbol":"RUSD",
		"amount":"50000",
		"deadline":%q
	}`, collectionID, tokenID, nftPaymentAddress, time.Now().Add(24*time.Hour).Format(time.RFC3339))
	status, response := callUserHandler(t, env.engine.MakeOffer, "buyer-1", http.MethodPost, "/nft/offers", body, nil)
	if status != http.StatusCreated {
		t.Fatalf("make offer: status %d body %s", status, response)
	}
	var parsed struct {
		Offer models.NFTOffer `json:"offer"`
	}
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		t.Fatalf("decode offer: %v", err)
	}

	// The bid is escrowed: the buyer's available balance dropped by the amount.
	if got := nftBalance(t, env, "buyer-1"); got.String() != "950000" {
		t.Fatalf("buyer balance after offer = %s, want 950000", got)
	}

	acceptURL := "/nft/offers/" + parsed.Offer.ID.String() + "/accept?collection_id=" + collectionID + fmt.Sprintf("&token_id=%d", tokenID)
	status, response = callUserHandler(t, env.engine.AcceptOffer, "seller-1", http.MethodPost, acceptURL, "", gin.Params{{Key: "id", Value: parsed.Offer.ID.String()}})
	if status != http.StatusOK {
		t.Fatalf("accept offer: status %d body %s", status, response)
	}

	// Fee 2.5% of 50000 = 1250; seller proceeds 48750.
	if got := nftBalance(t, env, "seller-1"); got.String() != "48750" {
		t.Fatalf("seller balance = %s, want 48750", got)
	}
	if got := nftBalance(t, env, nftDefaultFeeAccount); got.String() != "1250" {
		t.Fatalf("fee account = %s, want 1250", got)
	}

	var token models.NFTToken
	if err := env.db.First(&token, "collection_id = ? AND token_id = ?", collectionID, tokenID).Error; err != nil {
		t.Fatalf("load token: %v", err)
	}
	if token.OwnerUserID != "buyer-1" {
		t.Fatalf("token owner = %q, want buyer-1", token.OwnerUserID)
	}
}

// TestNFTOfferCancelRefunds checks that cancelling an offer refunds the exact
// escrowed amount.
func TestNFTOfferCancelRefunds(t *testing.T) {
	env := newNFTTestEnv(t)
	collectionID := nftCollectionID(t, env)
	tokenID := nftMint(t, env, collectionID, "seller-1")
	fund(t, env, "buyer-1", 1_000_000)

	body := fmt.Sprintf(`{
		"collection_id":%q,
		"token_id":%d,
		"payment_token_address":%q,
		"payment_token_symbol":"RUSD",
		"amount":"50000",
		"deadline":%q
	}`, collectionID, tokenID, nftPaymentAddress, time.Now().Add(24*time.Hour).Format(time.RFC3339))
	status, response := callUserHandler(t, env.engine.MakeOffer, "buyer-1", http.MethodPost, "/nft/offers", body, nil)
	if status != http.StatusCreated {
		t.Fatalf("make offer: status %d body %s", status, response)
	}
	var parsed struct {
		Offer models.NFTOffer `json:"offer"`
	}
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		t.Fatalf("decode offer: %v", err)
	}

	status, response = callUserHandler(t, env.engine.CancelOffer, "buyer-1", http.MethodPost,
		"/nft/offers/"+parsed.Offer.ID.String()+"/cancel", "", gin.Params{{Key: "id", Value: parsed.Offer.ID.String()}})
	if status != http.StatusOK {
		t.Fatalf("cancel offer: status %d body %s", status, response)
	}
	if got := nftBalance(t, env, "buyer-1"); got.String() != "1000000" {
		t.Fatalf("buyer balance after refund = %s, want 1000000", got)
	}
}

// TestNFTAcceptOfferWhileEscrowedRejected checks that a seller cannot accept an
// offer on a token a live listing holds.
func TestNFTAcceptOfferWhileEscrowedRejected(t *testing.T) {
	env := newNFTTestEnv(t)
	collectionID := nftCollectionID(t, env)
	tokenID := nftMint(t, env, collectionID, "seller-1")
	fund(t, env, "buyer-1", 1_000_000)

	body := fmt.Sprintf(`{
		"collection_id":%q,
		"token_id":%d,
		"payment_token_address":%q,
		"payment_token_symbol":"RUSD",
		"amount":"50000",
		"deadline":%q
	}`, collectionID, tokenID, nftPaymentAddress, time.Now().Add(24*time.Hour).Format(time.RFC3339))
	status, response := callUserHandler(t, env.engine.MakeOffer, "buyer-1", http.MethodPost, "/nft/offers", body, nil)
	if status != http.StatusCreated {
		t.Fatalf("make offer: status %d body %s", status, response)
	}
	var parsed struct {
		Offer models.NFTOffer `json:"offer"`
	}
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		t.Fatalf("decode offer: %v", err)
	}

	listNFT(t, env, collectionID, tokenID, "seller-1", "100000")

	acceptURL := "/nft/offers/" + parsed.Offer.ID.String() + "/accept?collection_id=" + collectionID + fmt.Sprintf("&token_id=%d", tokenID)
	status, response = callUserHandler(t, env.engine.AcceptOffer, "seller-1", http.MethodPost, acceptURL, "", gin.Params{{Key: "id", Value: parsed.Offer.ID.String()}})
	if status != http.StatusConflict {
		t.Fatalf("accept offer while escrowed: status %d body %s, want 409", status, response)
	}
}

// TestNFTListRequiresOwnership checks that a non-owner cannot list a token.
func TestNFTListRequiresOwnership(t *testing.T) {
	env := newNFTTestEnv(t)
	collectionID := nftCollectionID(t, env)
	tokenID := nftMint(t, env, collectionID, "seller-1")

	body := fmt.Sprintf(`{
		"collection_id":%q,
		"token_id":%d,
		"payment_token_address":%q,
		"payment_token_symbol":"RUSD",
		"price":"100000",
		"deadline":%q
	}`, collectionID, tokenID, nftPaymentAddress, time.Now().Add(24*time.Hour).Format(time.RFC3339))
	status, response := callUserHandler(t, env.engine.ListNFT, "stranger-1", http.MethodPost, "/nft/listings", body, nil)
	if status != http.StatusForbidden {
		t.Fatalf("stranger list: status %d body %s, want 403", status, response)
	}
}

// TestNFTListRejectsUnsupportedPayment checks that a listing priced in an
// unsupported token is rejected.
func TestNFTListRejectsUnsupportedPayment(t *testing.T) {
	env := newNFTTestEnv(t)
	collectionID := nftCollectionID(t, env)
	tokenID := nftMint(t, env, collectionID, "seller-1")

	body := fmt.Sprintf(`{
		"collection_id":%q,
		"token_id":%d,
		"payment_token_address":%q,
		"payment_token_symbol":"DOGE",
		"price":"100000",
		"deadline":%q
	}`, collectionID, tokenID, nftPaymentAddress, time.Now().Add(24*time.Hour).Format(time.RFC3339))
	status, response := callUserHandler(t, env.engine.ListNFT, "seller-1", http.MethodPost, "/nft/listings", body, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("unsupported payment: status %d body %s, want 400", status, response)
	}
}

// TestNFTMintRequiresActiveCollection checks that a paused collection rejects a
// mint.
func TestNFTMintRequiresActiveCollection(t *testing.T) {
	env := newNFTTestEnv(t)
	body := `{"key":"paused","name":"Paused","symbol":"PAU","chain_id":1}`
	status, response := callUserHandler(t, env.engine.CreateCollection, "admin-1", http.MethodPost, "/admin/nft/collections", body, nil)
	if status != http.StatusCreated {
		t.Fatalf("create collection: status %d body %s", status, response)
	}
	var parsed struct {
		Collection models.NFTCollection `json:"collection"`
	}
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		t.Fatalf("decode collection: %v", err)
	}
	if parsed.Collection.Status != "paused" {
		t.Fatalf("new collection status = %q, want paused", parsed.Collection.Status)
	}
	id := parsed.Collection.ID.String()
	mintBody := `{"uri":"a.json"}`
	status, response = callUserHandler(t, env.engine.Mint, "admin-1", http.MethodPost,
		"/admin/nft/collections/"+id+"/mint", mintBody, gin.Params{{Key: "id", Value: id}})
	if status != http.StatusConflict {
		t.Fatalf("mint into paused collection: status %d body %s, want 409", status, response)
	}
}

// TestNFTMintIncrementsSupply checks that the collection counter advances and
// ids are unique.
func TestNFTMintIncrementsSupply(t *testing.T) {
	env := newNFTTestEnv(t)
	collectionID := nftCollectionID(t, env)
	first := nftMint(t, env, collectionID, "user-1")
	second := nftMint(t, env, collectionID, "user-2")
	if first != 1 || second != 2 {
		t.Fatalf("token ids = %d,%d, want 1,2", first, second)
	}
	var collection models.NFTCollection
	if err := env.db.First(&collection, "id = ?", collectionID).Error; err != nil {
		t.Fatalf("load collection: %v", err)
	}
	if collection.TotalSupply != 2 {
		t.Fatalf("total supply = %d, want 2", collection.TotalSupply)
	}
}

// TestNFTDeleteCollectionWithTokensRejected checks that a collection holding
// tokens cannot be deleted.
func TestNFTDeleteCollectionWithTokensRejected(t *testing.T) {
	env := newNFTTestEnv(t)
	collectionID := nftCollectionID(t, env)
	nftMint(t, env, collectionID, "user-1")

	status, response := callUserHandler(t, env.engine.DeleteCollection, "admin-1", http.MethodDelete,
		"/admin/nft/collections/"+collectionID, "", gin.Params{{Key: "id", Value: collectionID}})
	if status != http.StatusConflict {
		t.Fatalf("delete collection with tokens: status %d body %s, want 409", status, response)
	}
}

// TestNFTFeeBpsBounds checks the fee ceiling matches the contract's MAX_FEE_BPS.
func TestNFTFeeBpsBounds(t *testing.T) {
	env := newNFTTestEnv(t)
	if err := env.engine.SetFeeBps(500); err != nil {
		t.Fatalf("set fee 500: %v", err)
	}
	if err := env.engine.SetFeeBps(501); err == nil {
		t.Fatalf("set fee 501 should be rejected")
	}
	if err := env.engine.SetFeeBps(-1); err == nil {
		t.Fatalf("negative fee should be rejected")
	}
}

// TestNFTFeeSplitRoundsDown checks that the fee rounds down and the seller gets
// the remainder.
func TestNFTFeeSplitRoundsDown(t *testing.T) {
	fee, proceeds := nftFee(big.NewInt(100003), 250)
	// 100003 * 250 / 10000 = 2500 (floored).
	if fee.String() != "2500" {
		t.Fatalf("fee = %s, want 2500", fee)
	}
	if proceeds.String() != "97503" {
		t.Fatalf("proceeds = %s, want 97503", proceeds)
	}
	if new(big.Int).Add(fee, proceeds).String() != "100003" {
		t.Fatalf("fee + proceeds must equal the price")
	}
}

// TestNFTHighestOfferListing checks the public offer read returns live offers.
func TestNFTOfferListing(t *testing.T) {
	env := newNFTTestEnv(t)
	collectionID := nftCollectionID(t, env)
	tokenID := nftMint(t, env, collectionID, "seller-1")
	fund(t, env, "buyer-1", 1_000_000)

	body := fmt.Sprintf(`{
		"collection_id":%q,
		"token_id":%d,
		"payment_token_address":%q,
		"payment_token_symbol":"RUSD",
		"amount":"50000",
		"deadline":%q
	}`, collectionID, tokenID, nftPaymentAddress, time.Now().Add(24*time.Hour).Format(time.RFC3339))
	if status, response := callUserHandler(t, env.engine.MakeOffer, "buyer-1", http.MethodPost, "/nft/offers", body, nil); status != http.StatusCreated {
		t.Fatalf("make offer: status %d body %s", status, response)
	}

	gin.SetMode(gin.TestMode)
	target := "/nft/offers?collection_id=" + collectionID + fmt.Sprintf("&token_id=%d", tokenID)
	status, response := callUserHandler(t, env.engine.GetOffers, "", http.MethodGet, target, "", nil)
	if status != http.StatusOK {
		t.Fatalf("get offers: status %d body %s", status, response)
	}
	var parsed struct {
		Offers []NFTOfferView `json:"offers"`
	}
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		t.Fatalf("decode offers: %v", err)
	}
	if len(parsed.Offers) != 1 {
		t.Fatalf("offers = %d, want 1", len(parsed.Offers))
	}
	if parsed.Offers[0].Offer.Amount != "50000" {
		t.Fatalf("offer amount = %q, want 50000", parsed.Offers[0].Offer.Amount)
	}
}
