package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"tigerswap/backend/models"
)

// EIP-4361 (Sign-In with Ethereum) implementation.
//
// The flow is two requests:
//  1. POST /auth/wallet/nonce  {address}  -> {nonce, message, expires_at}
//  2. POST /auth/wallet/verify {message, signature} -> {token, user}
//
// The nonce row is the single-use replay guard: the server stores the challenge
// it issued and refuses a signature over any nonce that was not issued, has
// already been consumed, or has expired. The domain named in the signed message
// must match the expected host, so a message minted for another site cannot be
// replayed here.

const (
	// walletNonceTTL bounds how long a signed-in challenge is valid. A short
	// window is enough for a user to approve the signature in their wallet and
	// limits how long a captured challenge could be replayed.
	walletNonceTTL = 5 * time.Minute
	// siweVersion is the EIP-4361 version this server speaks.
	siweVersion = "1"
)

// defaultAuthDomain is the fallback domain bound into the SIWE statement when
// AUTH_DOMAIN is not configured. It is only used for a bare local run; a
// production deployment must set AUTH_DOMAIN to its public host.
const defaultAuthDomain = "localhost"

// walletAuthDomain returns the domain a SIWE message must name. A configured
// AUTH_DOMAIN wins; otherwise the request host is used so a same-origin web app
// works without extra configuration.
func walletAuthDomain(c *gin.Context) string {
	if configured := strings.TrimSpace(os.Getenv("AUTH_DOMAIN")); configured != "" {
		return configured
	}
	if host := c.Request.Host; host != "" {
		return host
	}
	return defaultAuthDomain
}

// normalizeAddress lower-cases a hex address or returns an error. It is the
// single boundary check every wallet-auth input passes through.
func normalizeAddress(raw string) (string, error) {
	if !common.IsHexAddress(raw) {
		return "", publicErr(errors.New("invalid wallet address"))
	}
	return strings.ToLower(common.HexToAddress(raw).Hex()), nil
}

// buildSIWEMessage composes the EIP-4361 message a wallet signs.
func buildSIWEMessage(domain, address, statement, nonce string, chainID int64, issuedAt, expiresAt time.Time) string {
	var b strings.Builder
	b.WriteString(domain)
	b.WriteString(" wants you to sign in with your Ethereum account:\n")
	b.WriteString(address)
	b.WriteString("\n\n")
	b.WriteString(statement)
	b.WriteString("\n\n")
	b.WriteString("URI: ")
	b.WriteString("https://")
	b.WriteString(domain)
	b.WriteString("\nVersion: ")
	b.WriteString(siweVersion)
	b.WriteString("\nChain ID: ")
	b.WriteString(fmt.Sprintf("%d", chainID))
	b.WriteString("\nNonce: ")
	b.WriteString(nonce)
	b.WriteString("\nIssued At: ")
	b.WriteString(issuedAt.UTC().Format(time.RFC3339))
	b.WriteString("\nExpiration Time: ")
	b.WriteString(expiresAt.UTC().Format(time.RFC3339))
	return b.String()
}

// parsedSIWE is the subset of an EIP-4361 message this server verifies.
type parsedSIWE struct {
	Domain    string
	Address   string
	Statement string
	Nonce     string
	ChainID   int64
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// parseSIWE parses the canonical lines of an EIP-4361 message. It requires every
// security-relevant field (domain, address, nonce, chain id, issued at,
// expiration) and rejects a message that is missing any of them, so a verifier
// never falls back to a weaker check.
func parseSIWE(message string) (*parsedSIWE, error) {
	lines := strings.Split(message, "\n")
	if len(lines) < 1 {
		return nil, publicErr(errors.New("malformed sign-in message"))
	}

	header := " wants you to sign in with your Ethereum account:"
	if !strings.HasSuffix(lines[0], header) {
		return nil, publicErr(errors.New("malformed sign-in message header"))
	}
	parsed := &parsedSIWE{Domain: strings.TrimSuffix(lines[0], header)}
	if parsed.Domain == "" {
		return nil, publicErr(errors.New("sign-in message is missing the domain"))
	}

	// The address is the first non-empty line after the header.
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parsed.Address = strings.TrimSpace(line)
		break
	}
	if parsed.Address == "" {
		return nil, publicErr(errors.New("sign-in message is missing the address"))
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "Nonce:"):
			parsed.Nonce = strings.TrimSpace(strings.TrimPrefix(trimmed, "Nonce:"))
		case strings.HasPrefix(trimmed, "Chain ID:"):
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "Chain ID:"))
			var chainID int64
			if _, err := fmt.Sscanf(value, "%d", &chainID); err != nil {
				return nil, publicErr(errors.New("sign-in message has an invalid chain id"))
			}
			parsed.ChainID = chainID
		case strings.HasPrefix(trimmed, "Issued At:"):
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "Issued At:"))
			parsedAt, err := time.Parse(time.RFC3339, value)
			if err != nil {
				return nil, publicErr(errors.New("sign-in message has an invalid issued-at time"))
			}
			parsed.IssuedAt = parsedAt
		case strings.HasPrefix(trimmed, "Expiration Time:"):
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "Expiration Time:"))
			parsedExpiry, err := time.Parse(time.RFC3339, value)
			if err != nil {
				return nil, publicErr(errors.New("sign-in message has an invalid expiration time"))
			}
			parsed.ExpiresAt = parsedExpiry
		}
	}

	if parsed.Nonce == "" {
		return nil, publicErr(errors.New("sign-in message is missing the nonce"))
	}
	if parsed.ChainID <= 0 {
		return nil, publicErr(errors.New("sign-in message is missing the chain id"))
	}
	if parsed.IssuedAt.IsZero() || parsed.ExpiresAt.IsZero() {
		return nil, publicErr(errors.New("sign-in message is missing a timestamp"))
	}
	return parsed, nil
}

// recoverAddress reports the address that signed message, validating the
// signature's shape first. A high-s signature is malleable and recovers to a
// different address, so it is rejected before ecrecover; a bad signature makes
// ecrecover return the zero address, which is rejected too.
func recoverAddress(message string, signature []byte) (string, error) {
	if len(signature) != 65 {
		return "", publicErr(errors.New("signature must be 65 bytes"))
	}
	sig := make([]byte, 65)
	copy(sig, signature)
	if sig[64] >= 27 {
		sig[64] -= 27
	}
	if sig[64] != 0 && sig[64] != 1 {
		return "", publicErr(errors.New("signature has an invalid recovery id"))
	}

	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:64])
	if !crypto.ValidateSignatureValues(sig[64], r, s, true) {
		return "", publicErr(errors.New("signature is not valid"))
	}

	hash := accounts.TextHash([]byte(message))
	pub, err := crypto.SigToPub(hash, sig)
	if err != nil {
		return "", publicErr(errors.New("could not recover signer from signature"))
	}
	address := crypto.PubkeyToAddress(*pub)
	if address == (common.Address{}) {
		return "", publicErr(errors.New("recovered the zero address"))
	}
	return strings.ToLower(address.Hex()), nil
}

// RequestWalletNonce issues a single-use challenge bound to an address and
// stores it. Nothing is trusted from the client beyond the address, which is
// re-checked when the signature arrives.
func (s *AuthService) RequestWalletNonce(c *gin.Context) {
	var req struct {
		Address string `json:"address" binding:"required"`
		ChainID int64  `json:"chain_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	address, err := normalizeAddress(req.Address)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}

	rawNonce := make([]byte, 16)
	if _, err := rand.Read(rawNonce); err != nil {
		respondInternalError(c, err)
		return
	}
	nonce := hex.EncodeToString(rawNonce)

	domain := walletAuthDomain(c)
	chainID := req.ChainID
	if chainID <= 0 {
		chainID = 1
	}
	issuedAt := time.Now().UTC()
	expiresAt := issuedAt.Add(walletNonceTTL)
	statement := "Sign in to TigerSwap. This request will not trigger a blockchain transaction or cost any gas."

	message := buildSIWEMessage(domain, address, statement, nonce, chainID, issuedAt, expiresAt)

	row := models.WalletLoginNonce{
		Address:   address,
		Nonce:     nonce,
		Statement: statement,
		ChainID:   chainID,
		Domain:    domain,
		ExpiresAt: expiresAt,
	}
	if err := s.db.Create(&row).Error; err != nil {
		respondInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"nonce":      nonce,
		"message":    message,
		"domain":     domain,
		"expires_at": expiresAt,
	})
}

// VerifyWalletSignature verifies an EIP-4361 signature, consumes the nonce, and
// issues a session. A wallet that has never signed in gets an account created
// keyed by its address; an existing wallet account is returned. The address is
// the recovered signer's, never a client-supplied value.
func (s *AuthService) VerifyWalletSignature(c *gin.Context) {
	var req struct {
		Message   string `json:"message" binding:"required"`
		Signature string `json:"signature" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}

	parsed, err := parseSIWE(req.Message)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}

	// The signed message must name the domain this server expects, so a message
	// minted by another site cannot be replayed here.
	if !strings.EqualFold(parsed.Domain, walletAuthDomain(c)) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "sign-in domain mismatch"})
		return
	}

	now := time.Now().UTC()
	if now.After(parsed.ExpiresAt) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "sign-in message has expired"})
		return
	}
	if parsed.IssuedAt.After(now.Add(walletNonceTTL)) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "sign-in message is from the future"})
		return
	}

	claimedAddress, err := normalizeAddress(parsed.Address)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}

	signature, err := hex.DecodeString(strings.TrimPrefix(req.Signature, "0x"))
	if err != nil {
		respondBadRequestError(c, publicErr(errors.New("signature is not hex")))
		return
	}
	recovered, err := recoverAddress(req.Message, signature)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "signature verification failed"})
		return
	}
	if recovered != claimedAddress {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "signature does not match the claimed address"})
		return
	}

	// Consume the nonce atomically. A conditional update that affects exactly one
	// row proves the nonce was issued, unused, and unexpired; a second attempt
	// with the same nonce affects zero rows and is refused.
	var nonceRow models.WalletLoginNonce
	result := s.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("nonce = ? AND used = ? AND expires_at > ?", parsed.Nonce, false, now).
		First(&nonceRow)
	if result.Error != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "sign-in nonce is not valid"})
		return
	}
	if strings.ToLower(nonceRow.Address) != recovered {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "sign-in nonce does not match the signature"})
		return
	}
	consumed := s.db.Model(&models.WalletLoginNonce{}).
		Where("id = ? AND used = ?", nonceRow.ID, false).
		Update("used", true)
	if consumed.Error != nil {
		respondInternalError(c, consumed.Error)
		return
	}
	if consumed.RowsAffected != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "sign-in nonce has already been used"})
		return
	}

	user, err := s.findOrCreateWalletUser(recovered)
	if err != nil {
		respondInternalError(c, err)
		return
	}
	if !user.IsActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "account is not active"})
		return
	}

	token, err := s.generateToken(user.ID, user.Email, user.Role)
	if err != nil {
		respondInternalError(c, err)
		return
	}
	if err := s.setRefreshCookie(c, user.ID); err != nil {
		respondInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user":  user,
		"token": token,
	})
}

// findOrCreateWalletUser resolves the account for a wallet address, creating one
// on first sign-in. The lookup is inside a transaction with a row lock so two
// concurrent sign-ins from the same new wallet cannot create two accounts.
func (s *AuthService) findOrCreateWalletUser(address string) (*models.User, error) {
	var user models.User
	err := s.db.Transaction(func(tx *gorm.DB) error {
		lookup := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("wallet_address = ?", address).First(&user)
		if lookup.Error == nil {
			return nil
		}
		if !errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
			return lookup.Error
		}

		// A wallet-only account has no mailbox and no password. The email column has
		// a unique index, so a placeholder is required rather than an empty string
		// (two empty strings collide); the address, which is unique, is used to
		// build a reserved-domain address that can never be delivered to. The
		// username carries the full address so it is unique per wallet.
		username := "wallet_" + address
		email := address + "@wallet.invalid"
		user = models.User{
			ID:            uuid.New(),
			Email:         email,
			Username:      username,
			WalletAddress: address,
			Role:          models.RoleUser,
			IsActive:      true,
		}
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// hashRefreshToken returns the hex SHA-256 of a refresh token. Only the hash is
// stored, so reading the table yields no usable credential.
func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
