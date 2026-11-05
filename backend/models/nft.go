package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NFT listing and offer status values.
const (
	NFTStatusActive   = "active"
	NFTStatusSold     = "sold"
	NFTStatusCanceled = "canceled"
	NFTStatusExpired  = "expired"
)

// nftPaymentSymbols is the set of settlement assets the platform ledger can
// move. A listing priced in anything else could never be settled, so it is
// rejected at creation rather than left unfillable.
var nftPaymentSymbols = map[string]bool{
	"RUSD":  true,
	"USDT":  true,
	"USDC":  true,
	"TGR":   true,
	"RYUSD": true,
}

// IsSupportedNFTPayment reports whether a symbol may price an NFT sale.
func IsSupportedNFTPayment(symbol string) bool {
	return nftPaymentSymbols[symbol]
}

// NFTCollection is an admin-managed ERC-721 collection. It mirrors the
// `TigerSwapNFT` contract: the marketplace mints every token itself, so a
// collection has no external registry dependency.
type NFTCollection struct {
	ID      uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	Key     string    `gorm:"uniqueIndex;size:120" json:"key"`
	Name    string    `gorm:"size:160;not null" json:"name"`
	Symbol  string    `gorm:"size:40;not null" json:"symbol"`
	ChainID int64     `gorm:"index" json:"chain_id"`

	// ContractAddress is the deployed collection address once it has been
	// deployed. A collection with no address is database-only and cannot mint
	// on chain; the service refuses to mint in that case.
	ContractAddress string `gorm:"size:42" json:"contract_address"`

	// BaseURI is prefixed to a token's metadata path.
	BaseURI string `gorm:"size:400" json:"base_uri"`

	// TotalSupply counts the tokens minted so far, matching the contract's own
	// counter so an off-chain row and an on-chain collection agree.
	TotalSupply int64 `gorm:"default:0" json:"total_supply"`

	Status    string         `gorm:"size:20;index;default:'paused'" json:"status"`
	CreatedBy string         `gorm:"size:100" json:"created_by"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (m *NFTCollection) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// NFTToken is one minted token. `OwnerUserID` is the platform account that
// currently holds it. While a token is escrowed by a live listing the holder is
// the marketplace escrow account, recorded in `EscrowListingID`, so a seller
// cannot transfer a token out from under a buyer.
type NFTToken struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	CollectionID uuid.UUID `gorm:"uniqueIndex:idx_nft_token,priority:1;not null" json:"collection_id"`

	// TokenID is the collection-scoped token id, matching the contract counter.
	TokenID uint64 `gorm:"uniqueIndex:idx_nft_token,priority:2;not null" json:"token_id"`

	// OwnerUserID is the platform account holding the token. It is the escrow
	// account while a listing is live.
	OwnerUserID string `gorm:"size:100;index;not null" json:"owner_user_id"`

	URI string `gorm:"size:500" json:"uri"`

	// EscrowListingID is set to the listing that holds the token. A token in
	// escrow cannot be listed again or transferred.
	EscrowListingID *uuid.UUID `gorm:"type:uuid" json:"escrow_listing_id,omitempty"`

	MintedBy  string    `gorm:"size:100" json:"minted_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (m *NFTToken) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// NFTListing is a fixed-price sale. The token is escrowed (owner moved to the
// marketplace) for the life of the listing, so a live listing is always
// fillable and a cancel always returns the token.
type NFTListing struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	CollectionID uuid.UUID `gorm:"index;not null" json:"collection_id"`
	TokenID      uint64    `gorm:"index;not null" json:"token_id"`

	SellerUserID string `gorm:"size:100;index;not null" json:"seller_user_id"`

	PaymentTokenAddress  string `gorm:"size:42" json:"payment_token_address"`
	PaymentTokenSymbol   string `gorm:"size:20" json:"payment_token_symbol"`
	PaymentTokenDecimals int    `json:"payment_token_decimals"`

	// Price is an exact integer string in the payment token's smallest unit.
	Price string `gorm:"size:78" json:"price"`

	// FeeBps is the marketplace fee captured at sale time, frozen onto the
	// listing so a later fee change cannot alter an existing sale's split.
	FeeBps int `json:"fee_bps"`

	Deadline time.Time `gorm:"index" json:"deadline"`

	BuyerUserID *string    `gorm:"size:100" json:"buyer_user_id,omitempty"`
	SoldAt      *time.Time `json:"sold_at,omitempty"`

	Status    string    `gorm:"size:20;index;default:'active'" json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (m *NFTListing) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// NFTOffer is a standing bid. The buyer's payment is escrowed (debited) when
// the offer is made, so an accepted offer cannot fail for lack of funds, and a
// cancel refunds the exact escrowed amount.
type NFTOffer struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	CollectionID uuid.UUID `gorm:"index;not null" json:"collection_id"`
	TokenID      uint64    `gorm:"index;not null" json:"token_id"`

	BuyerUserID string `gorm:"size:100;index;not null" json:"buyer_user_id"`

	PaymentTokenAddress  string `gorm:"size:42" json:"payment_token_address"`
	PaymentTokenSymbol   string `gorm:"size:20" json:"payment_token_symbol"`
	PaymentTokenDecimals int    `json:"payment_token_decimals"`

	// Amount is the escrowed bid, an exact integer string in the payment
	// token's smallest unit.
	Amount string `gorm:"size:78" json:"amount"`

	FeeBps int `json:"fee_bps"`

	Deadline time.Time `gorm:"index" json:"deadline"`

	Status    string    `gorm:"size:20;index;default:'active'" json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (m *NFTOffer) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}
