package services

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// respondInternalError logs the underlying error and returns a generic body.
//
// A 5xx is raised by an internal operation (a database write, an upstream RPC,
// a signer), and the raw error can carry a SQL fragment, a filesystem path, an
// upstream URL or a credential-bearing connection string. None of that is
// useful to the caller, so it is logged for the operator and replaced with a
// fixed message on the wire.
func respondInternalError(c *gin.Context, err error) {
	log.Printf("internal error on %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}

// serviceError maps a domain sentinel to the status and message a client may
// see. The sentinel may be raised bare or wrapped with fmt.Errorf("%w: ..."),
// so a handler does not need to know how the engine annotated it.
type serviceError struct {
	sentinel error
	status   int
	message  string
}

var serviceErrors = []serviceError{
	// Margin engine.
	{ErrMarginMarketNotListed, http.StatusBadRequest, "margin market is not listed"},
	{ErrMarginAccountNotFound, http.StatusNotFound, "margin account not found"},
	{ErrMarginAssetNotAllowed, http.StatusBadRequest, "asset is not allowed by the listing"},
	{ErrMarginInsufficientCollateral, http.StatusUnprocessableEntity, "insufficient collateral for this borrow"},
	{ErrMarginNotLiquidatable, http.StatusBadRequest, "margin account is not liquidatable"},
	{ErrMarginInvalidMode, http.StatusBadRequest, "margin mode must be cross or isolated"},
	{ErrMarginNoDebt, http.StatusBadRequest, "no debt in this asset"},

	// Price oracle.
	{errPriceSymbolRequired, http.StatusBadRequest, "symbol is required"},
	{errPriceNoChain, http.StatusServiceUnavailable, "no price source is configured"},
	{errPriceUnknownToken, http.StatusNotFound, "unknown token for this chain"},

	// Perpetual engine.
	{ErrPerpPositionNotFound, http.StatusNotFound, "perpetual position not found"},
	{ErrPerpPositionNotClosable, http.StatusConflict, "position cannot be closed"},
	{ErrPerpPositionNotLiquidatable, http.StatusBadRequest, "perpetual position is not liquidatable"},
	{ErrPerpOrderNotFound, http.StatusBadRequest, "perpetual order not found"},
	{ErrPerpOrderNotCancellable, http.StatusBadRequest, "perpetual order cannot be cancelled"},
	{ErrPerpMarketNotListed, http.StatusConflict, "perpetual market is not listed"},

	// Copy trading.
	{errTraderNotFound, http.StatusNotFound, "trader not found"},
	{errAlreadyFollowing, http.StatusConflict, "already following a trader"},
	{errNotFollowing, http.StatusBadRequest, "not following any trader"},
	{errCopyPositionNotFound, http.StatusNotFound, "copy position not found"},
	{errCopyPositionClosed, http.StatusConflict, "copy position already closed"},
	{errCopyPositionForbidden, http.StatusForbidden, "copy position belongs to another account"},
	{errNoWalletBound, http.StatusForbidden, "no wallet is bound to this account"},

	// Dynamic chain/token registry.
	{errChainNotFound, http.StatusNotFound, "chain not found"},
	{errTokenNotFound, http.StatusNotFound, "token not found"},
	{errTokenAlreadyExists, http.StatusConflict, "token already exists"},
	{errDuplicateKey, http.StatusConflict, "a registration with that key already exists"},
	{errChainKeyRequired, http.StatusBadRequest, "chain key is required"},
	{errTokenKeyRequired, http.StatusBadRequest, "token key is required"},
	{errBridgeKeyRequired, http.StatusBadRequest, "bridge key is required"},
	{errFarmKeyRequired, http.StatusBadRequest, "farm key is required"},
	{errInvalidRPCURL, http.StatusBadRequest, "rpc_url must be an absolute http(s) URL"},
	{errUnsupportedChain, http.StatusBadRequest, "unsupported chain type"},
	{errPositiveDuration, http.StatusBadRequest, "duration_days must be positive"},

	// Launchpad.
	{errLaunchpadProjectNotFound, http.StatusNotFound, "launchpad project not found"},

	// Perpetual engine validation.
	{errPerpUnsupportedPair, http.StatusBadRequest, "unsupported perpetual pair"},
	{errPerpInvalidSide, http.StatusBadRequest, "side must be long or short"},
	{errPerpInvalidOrderType, http.StatusBadRequest, "unsupported order type"},
	{errPerpLimitNeedsPrice, http.StatusBadRequest, "a limit order requires a positive price"},
	{errPerpStopNeedsTrigger, http.StatusBadRequest, "a stop order requires a positive trigger price"},
	{errPerpExpiryNotPositive, http.StatusBadRequest, "expires_in must be positive"},

	// Advanced (OTC) orders.
	{errOrderNotFound, http.StatusNotFound, "order not found"},
	{errOrderNotOpen, http.StatusConflict, "order is no longer open"},
	{errOrderExpired, http.StatusConflict, "order has expired"},
	{errBelowMinFill, http.StatusBadRequest, "quantity is below the order's minimum fill"},
	{errSelfTake, http.StatusBadRequest, "you cannot take your own otc order"},
	{errFillExceedsOrder, http.StatusBadRequest, "quantity exceeds the order size"},

	// Options.
	{errOptionInsufficientPool, http.StatusConflict, "the option pool cannot underwrite this size"},
	{errOptionCollateralLocked, http.StatusConflict, "collateral is locked by open positions"},
	{errOptionCollateralReserved, http.StatusConflict, "collateral is reserved for open positions"},
	{errOptionExceedsPrincipal, http.StatusConflict, "amount exceeds the deposited principal"},
	{errOptionNoLiquidity, http.StatusConflict, "no liquidity in this series"},
	{errOptionNoPosition, http.StatusNotFound, "no option position to act on"},
	{errOptionPoolCannotCover, http.StatusConflict, "the option pool cannot cover this payout"},

	// NFT marketplace.
	{errNFTNotFound, http.StatusNotFound, "nft not found"},
	{errNFTNotOwner, http.StatusForbidden, "you do not own this token"},
	{errNFTInEscrow, http.StatusConflict, "token is escrowed by a live listing"},
	{errNFTNotActive, http.StatusConflict, "listing is not active"},
	{errNFTExpired, http.StatusConflict, "listing or offer has expired"},
	{errNFTSelfTrade, http.StatusBadRequest, "cannot trade with yourself"},
	{errNFTNoBid, http.StatusNotFound, "offer is not active"},
	{errNFTUnsupportedPay, http.StatusBadRequest, "unsupported payment token"},
	{errNFTCollectionOff, http.StatusConflict, "collection is not active"},
	{errNFTFeeExceedsPrice, http.StatusBadRequest, "fee exceeds the sale price"},

	// Airdrops.
	{errAlreadyClaimed, http.StatusConflict, "allocation already claimed"},
	{errNoAllocation, http.StatusNotFound, "no allocation for this account"},
	{errInsufficientEscrow, http.StatusConflict, "campaign escrow is insufficient"},
	{errCampaignNotOpen, http.StatusConflict, "airdrop campaign is not open for claims"},
}

// publicError marks an error whose message is safe to show a client because it
// is composed only of caller-supplied values and fixed validation text. A
// boundary validator that quotes the offending input uses this; an error that
// quotes a database, a path or an upstream endpoint must not.
type publicError struct{ err error }

func (e publicError) Error() string { return e.err.Error() }
func (e publicError) Unwrap() error { return e.err }

// publicErr marks a validation failure as safe to return to the caller.
func publicErr(err error) error { return publicError{err} }

// publicServiceMessage returns the status and fixed public message for a
// recognised sentinel, or false when the error is not one the client caused.
func publicServiceMessage(err error) (int, string, bool) {
	var pe publicError
	if errors.As(err, &pe) {
		return http.StatusBadRequest, pe.Error(), true
	}
	for _, entry := range serviceErrors {
		if errors.Is(err, entry.sentinel) {
			return entry.status, entry.message, true
		}
	}
	return 0, "", false
}

// respondServiceError answers a service call that failed. A recognised sentinel
// is rendered with its public message; anything else is an internal fault whose
// text is logged rather than returned to the client.
func respondServiceError(c *gin.Context, err error) {
	if status, message, ok := publicServiceMessage(err); ok {
		c.JSON(status, gin.H{"error": message})
		return
	}
	respondInternalError(c, err)
}

// respondBadRequestError answers a failure the caller can correct but that is
// not a registered sentinel. The detail is logged and a generic message is
// returned, so a validation helper that quotes internal state cannot leak it.
func respondBadRequestError(c *gin.Context, err error) {
	if status, message, ok := publicServiceMessage(err); ok {
		c.JSON(status, gin.H{"error": message})
		return
	}
	log.Printf("bad request on %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	c.JSON(http.StatusBadRequest, gin.H{"error": "request could not be processed"})
}
