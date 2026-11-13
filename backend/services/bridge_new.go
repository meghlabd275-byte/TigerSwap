package services

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"tigerswap/backend/config"
	"tigerswap/backend/models"
)

// tigerBridgeABI is the TigerBridge interface used by the relayer. It mirrors
// TigerSwapBridge.sol exactly: `lock` is the outbound call (the relayer signs it
// with the holder's custody key, so msg.sender is the account whose tokens are
// locked), and `completeBridge` is the inbound release gated by the validator
// supermajority. A mismatched ABI here would make the relayer build calldata the
// contract cannot decode.
const tigerBridgeABI = `[
 {"inputs":[{"internalType":"uint256","name":"toChain","type":"uint256"},{"internalType":"address","name":"token","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"},{"internalType":"address","name":"receiver","type":"address"},{"internalType":"uint256","name":"nonce","type":"uint256"}],"name":"lock","outputs":[{"internalType":"bytes32","name":"messageId","type":"bytes32"}],"stateMutability":"nonpayable","type":"function"},
 {"inputs":[{"components":[{"internalType":"bytes32","name":"messageId","type":"bytes32"},{"internalType":"uint256","name":"fromChain","type":"uint256"},{"internalType":"uint256","name":"toChain","type":"uint256"},{"internalType":"address","name":"token","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"},{"internalType":"address","name":"receiver","type":"address"}],"internalType":"struct TigerSwapBridge.BridgeMessage","name":"message","type":"tuple"},{"internalType":"bytes","name":"signatures","type":"bytes"}],"name":"completeBridge","outputs":[],"stateMutability":"nonpayable","type":"function"},
 {"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"sender","type":"address"},{"indexed":true,"internalType":"uint256","name":"toChain","type":"uint256"},{"indexed":false,"internalType":"address","name":"token","type":"address"},{"indexed":false,"internalType":"uint256","name":"amount","type":"uint256"},{"indexed":false,"internalType":"uint256","name":"fee","type":"uint256"},{"indexed":false,"internalType":"address","name":"receiver","type":"address"},{"indexed":false,"internalType":"bytes32","name":"messageId","type":"bytes32"}],"name":"BridgeInitiated","type":"event"},
 {"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"receiver","type":"address"},{"indexed":true,"internalType":"uint256","name":"fromChain","type":"uint256"},{"indexed":false,"internalType":"address","name":"token","type":"address"},{"indexed":false,"internalType":"uint256","name":"amount","type":"uint256"},{"indexed":false,"internalType":"bytes32","name":"messageId","type":"bytes32"}],"name":"BridgeCompleted","type":"event"}
]`

// bridgeMessage is the ABI-tuple form of TigerSwapBridge.BridgeMessage, used to
// encode the inbound payload the validators sign and the relayer submits.
type bridgeMessage struct {
	MessageID [32]byte       `abi:"messageId"`
	FromChain *big.Int       `abi:"fromChain"`
	ToChain   *big.Int       `abi:"toChain"`
	Token     common.Address `abi:"token"`
	Amount    *big.Int       `abi:"amount"`
	Receiver  common.Address `abi:"receiver"`
}

// bridgeMessageTuple is the ABI type of the BridgeMessage struct, used to build
// the payload hash the contract's inboundDigest hashes.
var bridgeMessageTuple = mustParseABIArgTuple([]abi.ArgumentMarshaling{
	{Name: "messageId", Type: "bytes32"},
	{Name: "fromChain", Type: "uint256"},
	{Name: "toChain", Type: "uint256"},
	{Name: "token", Type: "address"},
	{Name: "amount", Type: "uint256"},
	{Name: "receiver", Type: "address"},
})

func mustParseABIArgTuple(fields []abi.ArgumentMarshaling) abi.Type {
	t, err := abi.NewType("tuple", "", fields)
	if err != nil {
		panic("bridge: build BridgeMessage ABI tuple: " + err.Error())
	}
	return t
}

// bridgeInboundDigest recomputes the digest validators must sign, matching
// TigerSwapBridge.inboundDigest digit for digit:
//
// payloadHash = keccak256(abi.encode(messageId, fromChain, toChain, token, amount, receiver))
// digest      = keccak256("\x19Ethereum Signed Message:\n32" || payloadHash)
func bridgeInboundDigest(msg bridgeMessage) ([]byte, error) {
	packed, err := abi.Arguments{{Type: bridgeMessageTuple}}.Pack(msg)
	if err != nil {
		return nil, fmt.Errorf("encode bridge message: %w", err)
	}
	payloadHash := crypto.Keccak256(packed)
	prefix := []byte("\x19Ethereum Signed Message:\n32")
	return crypto.Keccak256(append(prefix, payloadHash...)), nil
}

// BridgeService handles TigerBridge cross-chain lock/mint transfers.
type BridgeService struct {
	blockchain *BlockchainClient
	priceAgg   *PriceAggregator
	db         *gorm.DB
	wallet     *WalletService
	mu         sync.RWMutex

	// Supported bridges (self-hosted relayer routes)
	supportedBridges map[int64][]BridgeConfig
}

// BridgeTransfer represents a bridge transfer.
type BridgeTransfer struct {
	ID             string     `json:"id"`
	SrcChain       int64      `json:"src_chain"`
	DstChain       int64      `json:"dst_chain"`
	Sender         string     `json:"sender"`
	Recipient      string     `json:"recipient"`
	Token          string     `json:"token"`
	Amount         string     `json:"amount"`
	MessageID      string     `json:"message_id"`
	MessageAmount  string     `json:"message_amount"`
	SrcTxHash      string     `json:"src_tx_hash"`
	DstTxHash      string     `json:"dst_tx_hash"`
	Nonce          uint64     `json:"nonce"`
	Status         string     `json:"status"` // pending, confirmed, delivered, failed
	Fee            string     `json:"fee"`
	FeeUSD         float64    `json:"fee_usd"`
	AmountReceived string     `json:"amount_received"`
	CreatedAt      time.Time  `json:"created_at"`
	ConfirmedAt    *time.Time `json:"confirmed_at"`
	DeliveredAt    *time.Time `json:"delivered_at"`
}

// newBridgeNonce returns a random 64-bit nonce for a bridge transfer. The
// nonce is part of the on-chain messageId, so a value that repeats would make
// the contract reject the second transfer with MESSAGE_EXISTS. The wall-clock
// nanosecond collides when two transfers are created in the same tick, so the
// nonce is drawn from the system CSPRNG instead.
func newBridgeNonce() (uint64, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 0, fmt.Errorf("generate bridge nonce: %w", err)
	}
	return binary.BigEndian.Uint64(buf[:]), nil
}

// BridgeConfig represents a self-hosted relayer route configuration.
type BridgeConfig struct {
	Name          string  `json:"name"`
	Router        string  `json:"router"`
	ChainAgnostic bool    `json:"chain_agnostic"`
	AvgTime       int     `json:"avg_time_minutes"`
	FeePercent    float64 `json:"fee_percent"`
}

// BridgeQuote represents a bridge quote.
type BridgeQuote struct {
	SrcChain      int64   `json:"src_chain"`
	DstChain      int64   `json:"dst_chain"`
	Token         string  `json:"token"`
	AmountIn      string  `json:"amount_in"`
	AmountOut     string  `json:"amount_out"`
	Fee           string  `json:"fee"`
	FeeUSD        float64 `json:"fee_usd"`
	EstimatedTime string  `json:"estimated_time"`
	Router        string  `json:"router"`
	PriceImpact   float64 `json:"price_impact"`
}

func NewBridgeService(db *gorm.DB, blockchain *BlockchainClient, priceAgg *PriceAggregator) *BridgeService {
	return &BridgeService{
		blockchain: blockchain,
		priceAgg:   priceAgg,
		db:         db,
		supportedBridges: map[int64][]BridgeConfig{
			1:     {{Name: "TigerBridge", Router: tigerBridgeRouter(1), ChainAgnostic: true, AvgTime: 15, FeePercent: 0.1}},
			137:   {{Name: "TigerBridge", Router: tigerBridgeRouter(137), ChainAgnostic: true, AvgTime: 10, FeePercent: 0.08}},
			56:    {{Name: "TigerBridge", Router: tigerBridgeRouter(56), ChainAgnostic: true, AvgTime: 12, FeePercent: 0.08}},
			42161: {{Name: "TigerBridge", Router: tigerBridgeRouter(42161), ChainAgnostic: true, AvgTime: 15, FeePercent: 0.1}},
			10:    {{Name: "TigerBridge", Router: tigerBridgeRouter(10), ChainAgnostic: true, AvgTime: 15, FeePercent: 0.1}},
			8453:  {{Name: "TigerBridge", Router: tigerBridgeRouter(8453), ChainAgnostic: true, AvgTime: 15, FeePercent: 0.1}},
			43114: {{Name: "TigerBridge", Router: tigerBridgeRouter(43114), ChainAgnostic: true, AvgTime: 20, FeePercent: 0.12}},
		},
	}
}

// SetWallet wires the custody wallet service. Outbound locks are signed with
// the holder's own custody key so the on-chain msg.sender is the token owner;
// the relayer never spends on behalf of an account whose key it does not hold.
func (s *BridgeService) SetWallet(wallet *WalletService) {
	s.wallet = wallet
}

// bridgeSigningKey returns the key that signs an outbound lock. The sender must
// be a custody wallet the platform holds the key for; anything else is refused.
func (s *BridgeService) bridgeSigningKey(sender string) (*ecdsa.PrivateKey, error) {
	if s.wallet == nil {
		return nil, fmt.Errorf("custody wallet service is not configured")
	}
	return s.wallet.custodyKeyForAddress(sender)
}

// validatorKeys loads the TigerBridge validator signing keys. The contract
// requires a supermajority of distinct validator signatures before it releases
// locked liquidity, so the relayer signs the inbound digest with each key it
// holds. Keys come from TIGERBRIDGE_VALIDATOR_KEYS (comma-separated hex); a
// single key is accepted for single-validator test deployments.
func validatorKeys() ([]*ecdsa.PrivateKey, error) {
	raw := strings.TrimSpace(config.Secret("TIGERBRIDGE_VALIDATOR_KEYS"))
	if raw == "" {
		return nil, fmt.Errorf("TIGERBRIDGE_VALIDATOR_KEYS is not configured")
	}
	var keys []*ecdsa.PrivateKey
	for _, part := range strings.Split(raw, ",") {
		hexKey := strings.TrimSpace(part)
		if hexKey == "" {
			continue
		}
		key, err := crypto.HexToECDSA(strings.TrimPrefix(hexKey, "0x"))
		if err != nil {
			return nil, fmt.Errorf("invalid validator key: %w", err)
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("no TigerBridge validator keys configured")
	}
	return keys, nil
}

// signInboundDigest signs the inbound digest with every configured validator
// key and returns the concatenated 65-byte (r||s||v) signatures the contract's
// completeBridge expects.
func signInboundDigest(digest []byte) ([]byte, error) {
	keys, err := validatorKeys()
	if err != nil {
		return nil, err
	}
	var sigs []byte
	for _, key := range keys {
		sig, err := crypto.Sign(digest, key)
		if err != nil {
			return nil, fmt.Errorf("sign bridge digest: %w", err)
		}
		// crypto.Sign emits v as 0/1; the contract range-checks for 27/28.
		sig[64] += 27
		sigs = append(sigs, sig...)
	}
	return sigs, nil
}

// tigerBridgeRouter returns the configured TigerBridge router for a chain.
func tigerBridgeRouter(chainID int64) string {
	if v := os.Getenv("TIGERBRIDGE_ROUTER_" + strconv.FormatInt(chainID, 10)); v != "" {
		return v
	}
	return os.Getenv("TIGERBRIDGE_ROUTER")
}

// relayerKey loads the TigerBridge relayer signing key.
func relayerKey() (*ecdsa.PrivateKey, error) {
	raw := config.Secret("TIGERBRIDGE_RELAYER_KEY")
	if raw == "" {
		return nil, fmt.Errorf("TIGERBRIDGE_RELAYER_KEY is not configured")
	}
	return crypto.HexToECDSA(strings.TrimPrefix(raw, "0x"))
}

// GetSupportedChains returns chains supported for bridging.
func (s *BridgeService) GetSupportedChains() []int64 {
	chains := make([]int64, 0, len(s.supportedBridges))
	for id := range s.supportedBridges {
		chains = append(chains, id)
	}
	return chains
}

// route returns the relayer route for a source chain.
func (s *BridgeService) route(srcChain int64) (BridgeConfig, error) {
	bridges := s.supportedBridges[srcChain]
	if len(bridges) == 0 {
		return BridgeConfig{}, fmt.Errorf("no TigerBridge route configured for chain %d", srcChain)
	}
	if bridges[0].Router == "" {
		return BridgeConfig{}, fmt.Errorf("TigerBridge router not configured for chain %d", srcChain)
	}
	return bridges[0], nil
}

// GetQuote returns a bridge quote based on the configured relayer fee.
func (s *BridgeService) GetQuote(ctx context.Context, srcChain, dstChain int64, token, amount string) (*BridgeQuote, error) {
	route, err := s.route(srcChain)
	if err != nil {
		return nil, err
	}
	if srcChain == dstChain {
		return nil, fmt.Errorf("source and destination chains must differ")
	}
	if _, ok := s.supportedBridges[dstChain]; !ok {
		return nil, fmt.Errorf("destination chain %d is not supported", dstChain)
	}

	amountFloat, err := strconv.ParseFloat(amount, 64)
	if err != nil || amountFloat <= 0 {
		return nil, fmt.Errorf("invalid amount %q", amount)
	}

	tokenSymbol := s.getSymbolFromAddress(srcChain, token)
	priceData, err := s.priceAgg.GetRealPrice(ctx, tokenSymbol)
	if err != nil {
		return nil, fmt.Errorf("price unavailable for %s: %w", tokenSymbol, err)
	}
	if priceData.USD <= 0 {
		return nil, fmt.Errorf("invalid price for %s", tokenSymbol)
	}

	amountUSD := amountFloat * priceData.USD
	feeUSD := amountUSD * route.FeePercent / 100
	fee := feeUSD / priceData.USD
	amountOut := amountFloat - fee
	if amountOut <= 0 {
		return nil, fmt.Errorf("amount too small to cover bridge fee")
	}

	return &BridgeQuote{
		SrcChain:      srcChain,
		DstChain:      dstChain,
		Token:         token,
		AmountIn:      amount,
		AmountOut:     strconv.FormatFloat(amountOut, 'f', 8, 64),
		Fee:           strconv.FormatFloat(fee, 'f', 8, 64),
		FeeUSD:        feeUSD,
		EstimatedTime: fmt.Sprintf("%d minutes", route.AvgTime),
		Router:        route.Router,
		PriceImpact:   0.0,
	}, nil
}

// InitiateTransfer locks the token on the source chain via the TigerBridge
// router. The lock is signed with the sender's own custody key, so the on-chain
// msg.sender is the account whose tokens are moved; the relayer never spends on
// behalf of an account it does not custody.
func (s *BridgeService) InitiateTransfer(ctx context.Context, srcChain, dstChain int64, sender, recipient, token, amount string) (*BridgeTransfer, error) {
	quote, err := s.GetQuote(ctx, srcChain, dstChain, token, amount)
	if err != nil {
		return nil, err
	}
	if !common.IsHexAddress(sender) || !common.IsHexAddress(recipient) {
		return nil, fmt.Errorf("invalid sender or recipient address")
	}
	if !common.IsHexAddress(token) {
		return nil, fmt.Errorf("invalid token address")
	}
	if s.blockchain == nil {
		return nil, fmt.Errorf("blockchain client not configured")
	}

	route, err := s.route(srcChain)
	if err != nil {
		return nil, err
	}

	amountInt, ok := new(big.Int).SetString(amount, 10)
	if !ok || amountInt.Sign() <= 0 {
		return nil, fmt.Errorf("invalid amount %q", amount)
	}

	key, err := s.bridgeSigningKey(sender)
	if err != nil {
		return nil, err
	}
	client, err := s.blockchain.GetClient(srcChain)
	if err != nil {
		return nil, err
	}

	nonce, err := newBridgeNonce()
	if err != nil {
		return nil, err
	}
	transfer := &BridgeTransfer{
		ID:             "bridge_" + uuid.New().String(),
		SrcChain:       srcChain,
		DstChain:       dstChain,
		Sender:         sender,
		Recipient:      recipient,
		Token:          token,
		Amount:         amount,
		Nonce:          nonce,
		Status:         "pending",
		Fee:            quote.Fee,
		FeeUSD:         quote.FeeUSD,
		AmountReceived: quote.AmountOut,
		CreatedAt:      time.Now(),
	}

	// Persist before broadcasting so a crash never loses the intent.
	if err := s.persist(transfer); err != nil {
		return nil, fmt.Errorf("persist transfer: %w", err)
	}

	router := common.HexToAddress(route.Router)
	// The bridge pulls the tokens with transferFrom, so the router needs an
	// allowance from the holder first.
	if err := ensureBridgeAllowance(ctx, client, key, srcChain, common.HexToAddress(token), router, amountInt); err != nil {
		return nil, fmt.Errorf("approve bridge router: %w", err)
	}

	bridgeABI, err := abi.JSON(strings.NewReader(tigerBridgeABI))
	if err != nil {
		return nil, err
	}
	data, err := bridgeABI.Pack("lock",
		big.NewInt(dstChain), common.HexToAddress(token), amountInt,
		common.HexToAddress(recipient), new(big.Int).SetUint64(nonce))
	if err != nil {
		return nil, err
	}

	txHash, err := signAndSend(ctx, client, key, srcChain, router, big.NewInt(0), data)
	if err != nil {
		transfer.Status = "failed"
		s.persistOrLog(transfer)
		return nil, fmt.Errorf("lock transaction failed: %w", err)
	}
	transfer.SrcTxHash = txHash

	// Wait for the source-chain confirmation before the relayer mints.
	receipt, err := waitForReceipt(ctx, client, common.HexToHash(txHash), 5*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("awaiting lock confirmation: %w", err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		transfer.Status = "failed"
		s.persistOrLog(transfer)
		return nil, fmt.Errorf("lock transaction reverted: %s", txHash)
	}

	// A successful receipt is not proof the lock happened with the intended
	// parameters: decode the BridgeInitiated event and require it to match this
	// transfer's recipient, destination chain and token. Without this a relayer
	// would confirm a transfer whose on-chain message differs from what it stored.
	msgID, msgAmount, err := parseBridgeInitiated(bridgeABI, receipt, router, common.HexToAddress(sender), dstChain, common.HexToAddress(token), common.HexToAddress(recipient))
	if err != nil {
		transfer.Status = "failed"
		s.persistOrLog(transfer)
		return nil, fmt.Errorf("verify lock event: %w", err)
	}
	transfer.MessageID = msgID
	transfer.MessageAmount = msgAmount.String()

	now := time.Now()
	transfer.Status = "confirmed"
	transfer.ConfirmedAt = &now
	if err := s.persist(transfer); err != nil {
		return nil, fmt.Errorf("persist confirmed transfer: %w", err)
	}

	return transfer, nil
}

// ensureBridgeAllowance approves the bridge router from the holder's key when the
// current allowance is below the amount being locked.
func ensureBridgeAllowance(ctx context.Context, client *ethclient.Client, key *ecdsa.PrivateKey, chainID int64, token, spender common.Address, amount *big.Int) error {
	owner := crypto.PubkeyToAddress(key.PublicKey)
	allowance, err := readAllowance(ctx, client, token, owner, spender)
	if err == nil && allowance.Cmp(amount) >= 0 {
		return nil
	}
	approveABI, err := abi.JSON(strings.NewReader(erc20ApproveABI))
	if err != nil {
		return err
	}
	max := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	data, err := approveABI.Pack("approve", spender, max)
	if err != nil {
		return err
	}
	if _, err := signAndSend(ctx, client, key, chainID, token, big.NewInt(0), data); err != nil {
		return err
	}
	return nil
}

// parseBridgeInitiated decodes the BridgeInitiated event from a lock receipt and
// verifies it matches the expected sender, destination chain, token and
// recipient. It returns the on-chain messageId and the transfer amount (after
// the protocol fee), which are what the inbound release must use.
func parseBridgeInitiated(bridgeABI abi.ABI, receipt *types.Receipt, router, sender common.Address, dstChain int64, token, recipient common.Address) (string, *big.Int, error) {
	eventID := bridgeABI.Events["BridgeInitiated"].ID
	for _, lg := range receipt.Logs {
		if lg.Address != router || len(lg.Topics) == 0 || lg.Topics[0] != eventID {
			continue
		}
		if len(lg.Topics) < 3 {
			return "", nil, fmt.Errorf("BridgeInitiated event has %d topics, want 3", len(lg.Topics))
		}
		// Topics: [eventID, sender, toChain]; data: token, amount, fee, receiver, messageId.
		eventSender := common.BytesToAddress(lg.Topics[1].Bytes())
		eventToChain := new(big.Int).SetBytes(lg.Topics[2].Bytes())
		if eventSender != sender {
			return "", nil, fmt.Errorf("BridgeInitiated sender %s does not match %s", eventSender.Hex(), sender.Hex())
		}
		if eventToChain.Cmp(big.NewInt(dstChain)) != 0 {
			return "", nil, fmt.Errorf("BridgeInitiated destination chain %s does not match %d", eventToChain.String(), dstChain)
		}
		var fields struct {
			Token     common.Address `abi:"token"`
			Amount    *big.Int       `abi:"amount"`
			Fee       *big.Int       `abi:"fee"`
			Receiver  common.Address `abi:"receiver"`
			MessageID [32]byte       `abi:"messageId"`
		}
		if err := bridgeABI.UnpackIntoInterface(&fields, "BridgeInitiated", lg.Data); err != nil {
			return "", nil, fmt.Errorf("decode BridgeInitiated data: %w", err)
		}
		if fields.Token != token {
			return "", nil, fmt.Errorf("BridgeInitiated token %s does not match %s", fields.Token.Hex(), token.Hex())
		}
		if fields.Receiver != recipient {
			return "", nil, fmt.Errorf("BridgeInitiated receiver %s does not match %s", fields.Receiver.Hex(), recipient.Hex())
		}
		if fields.Amount == nil || fields.Amount.Sign() <= 0 {
			return "", nil, fmt.Errorf("BridgeInitiated amount is not positive")
		}
		return common.Hash(fields.MessageID).Hex(), fields.Amount, nil
	}
	return "", nil, fmt.Errorf("no BridgeInitiated event from the bridge router in the lock receipt")
}

// ReleaseMint releases the bridged token on the destination chain by calling
// TigerSwapBridge.completeBridge with the validator supermajority signature. It
// is driven by the relayer once a source-chain lock is confirmed, and uses the
// on-chain message id and post-fee amount captured from the source BridgeInitiated
// event so the released amount is exactly what was locked.
func (s *BridgeService) ReleaseMint(ctx context.Context, transferID string) (*BridgeTransfer, error) {
	transfer, err := s.GetTransfer(transferID)
	if err != nil {
		return nil, err
	}
	if transfer.Status != "confirmed" {
		return nil, fmt.Errorf("transfer %s is not confirmed (status %s)", transferID, transfer.Status)
	}
	if s.blockchain == nil {
		return nil, fmt.Errorf("blockchain client not configured")
	}
	if transfer.MessageID == "" {
		return nil, fmt.Errorf("transfer %s has no on-chain message id; the source lock was never verified", transferID)
	}

	route, err := s.route(transfer.DstChain)
	if err != nil {
		return nil, err
	}

	// The released amount is the post-fee amount recorded from the source event.
	amountStr := transfer.MessageAmount
	if amountStr == "" {
		amountStr = transfer.AmountReceived
	}
	amountInt, ok := new(big.Int).SetString(amountStr, 10)
	if !ok || amountInt.Sign() <= 0 {
		return nil, fmt.Errorf("invalid stored release amount %q", amountStr)
	}

	msgIDBytes, err := hex.DecodeString(strings.TrimPrefix(transfer.MessageID, "0x"))
	if err != nil || len(msgIDBytes) != 32 {
		return nil, fmt.Errorf("stored message id %q is not a 32-byte hash", transfer.MessageID)
	}
	var msgID [32]byte
	copy(msgID[:], msgIDBytes)

	msg := bridgeMessage{
		MessageID: msgID,
		FromChain: big.NewInt(transfer.SrcChain),
		ToChain:   big.NewInt(transfer.DstChain),
		Token:     common.HexToAddress(transfer.Token),
		Amount:    amountInt,
		Receiver:  common.HexToAddress(transfer.Recipient),
	}
	digest, err := bridgeInboundDigest(msg)
	if err != nil {
		return nil, err
	}
	sigs, err := signInboundDigest(digest)
	if err != nil {
		return nil, err
	}

	bridgeABI, err := abi.JSON(strings.NewReader(tigerBridgeABI))
	if err != nil {
		return nil, err
	}
	data, err := bridgeABI.Pack("completeBridge", msg, sigs)
	if err != nil {
		return nil, fmt.Errorf("encode completeBridge: %w", err)
	}

	key, err := relayerKey()
	if err != nil {
		return nil, err
	}
	client, err := s.blockchain.GetClient(transfer.DstChain)
	if err != nil {
		return nil, err
	}

	txHash, err := signAndSend(ctx, client, key, transfer.DstChain, common.HexToAddress(route.Router), big.NewInt(0), data)
	if err != nil {
		return nil, fmt.Errorf("release transaction failed: %w", err)
	}
	transfer.DstTxHash = txHash

	receipt, err := waitForReceipt(ctx, client, common.HexToHash(txHash), 5*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("awaiting release confirmation: %w", err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		transfer.Status = "failed"
		s.persistOrLog(transfer)
		return nil, fmt.Errorf("release transaction reverted: %s", txHash)
	}

	now := time.Now()
	transfer.Status = "delivered"
	transfer.DeliveredAt = &now
	if err := s.persist(transfer); err != nil {
		return nil, fmt.Errorf("persist delivered transfer: %w", err)
	}

	return transfer, nil
}

// DefaultBridgeSweepInterval is how often the relayer releases destination
// mints for transfers whose source-chain lock has confirmed.
const DefaultBridgeSweepInterval = 30 * time.Second

// StartBridgeRelayer drives the TigerBridge relayer: every interval it releases
// the destination mint for each confirmed lock. Without it a transfer stops at
// `confirmed` and the recipient never receives the bridged asset.
func StartBridgeRelayer(ctx context.Context, bridge *BridgeService, interval time.Duration) {
	if bridge == nil {
		log.Println("bridge relayer not started: bridge service not configured")
		return
	}
	if interval <= 0 {
		interval = DefaultBridgeSweepInterval
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := bridge.ProcessPending(ctx); err != nil {
					log.Printf("bridge relayer: sweep failed: %v", err)
				}
			}
		}
	}()
}

// ProcessPending sweeps confirmed transfers and releases mints on the destination chain.
func (s *BridgeService) ProcessPending(ctx context.Context) error {
	if s.db == nil {
		return fmt.Errorf("database not configured")
	}
	var rows []models.BridgeTransfer
	if err := s.db.Where("status = ?", "confirmed").Order("created_at asc").Limit(50).Find(&rows).Error; err != nil {
		return err
	}
	for i := range rows {
		if _, err := s.ReleaseMint(ctx, rows[i].TransferID); err != nil {
			// Continue processing others; the row stays confirmed for the next sweep.
			continue
		}
	}
	return nil
}

// persistOrLog records a transfer's terminal state on a path that is already
// returning an error. A persistence failure is logged rather than discarded: it
// would otherwise leave the row showing a stale status, which an operator
// reading the bridge table would misread as an in-flight transfer.
func (s *BridgeService) persistOrLog(transfer *BridgeTransfer) {
	if err := s.persist(transfer); err != nil {
		log.Printf("bridge: persist transfer %s status %q failed: %v", transfer.ID, transfer.Status, err)
	}
}

// persist upserts a transfer into the database.
func (s *BridgeService) persist(transfer *BridgeTransfer) error {
	if s.db == nil {
		return fmt.Errorf("database not configured")
	}
	var row models.BridgeTransfer
	err := s.db.Where("transfer_id = ?", transfer.ID).First(&row).Error
	row.TransferID = transfer.ID
	row.SrcChain = transfer.SrcChain
	row.DstChain = transfer.DstChain
	row.Sender = transfer.Sender
	row.Recipient = transfer.Recipient
	row.Token = transfer.Token
	row.Amount = transfer.Amount
	row.MessageID = transfer.MessageID
	row.MessageAmount = transfer.MessageAmount
	row.SrcTxHash = transfer.SrcTxHash
	row.DstTxHash = transfer.DstTxHash
	row.Nonce = transfer.Nonce
	row.Status = transfer.Status
	row.Fee = transfer.Fee
	row.FeeUSD = transfer.FeeUSD
	row.AmountReceived = transfer.AmountReceived
	row.ConfirmedAt = transfer.ConfirmedAt
	row.DeliveredAt = transfer.DeliveredAt
	row.UpdatedAt = time.Now()
	if err == nil {
		return s.db.Save(&row).Error
	}
	row.CreatedAt = transfer.CreatedAt
	return s.db.Create(&row).Error
}

// GetTransfer returns transfer status.
func (s *BridgeService) GetTransfer(transferID string) (*BridgeTransfer, error) {
	if s.db != nil {
		var row models.BridgeTransfer
		if err := s.db.Where("transfer_id = ?", transferID).First(&row).Error; err == nil {
			return bridgeTransferFromModel(&row), nil
		}
	}
	return nil, fmt.Errorf("transfer not found")
}

// GetUserTransfers returns all transfers for a user.
func (s *BridgeService) GetUserTransfers(userAddress string) []*BridgeTransfer {
	if s.db == nil {
		return nil
	}
	var rows []models.BridgeTransfer
	if err := s.db.Where("sender = ? OR recipient = ?", userAddress, userAddress).
		Order("created_at desc").Find(&rows).Error; err != nil {
		return nil
	}
	result := make([]*BridgeTransfer, 0, len(rows))
	for i := range rows {
		result = append(result, bridgeTransferFromModel(&rows[i]))
	}
	return result
}

func bridgeTransferFromModel(m *models.BridgeTransfer) *BridgeTransfer {
	return &BridgeTransfer{
		ID:             m.TransferID,
		SrcChain:       m.SrcChain,
		DstChain:       m.DstChain,
		Sender:         m.Sender,
		Recipient:      m.Recipient,
		Token:          m.Token,
		Amount:         m.Amount,
		MessageID:      m.MessageID,
		MessageAmount:  m.MessageAmount,
		SrcTxHash:      m.SrcTxHash,
		DstTxHash:      m.DstTxHash,
		Nonce:          m.Nonce,
		Status:         m.Status,
		Fee:            m.Fee,
		FeeUSD:         m.FeeUSD,
		AmountReceived: m.AmountReceived,
		CreatedAt:      m.CreatedAt,
		ConfirmedAt:    m.ConfirmedAt,
		DeliveredAt:    m.DeliveredAt,
	}
}

// GetSupportedBridges returns bridges for a chain.
func (s *BridgeService) GetSupportedBridges(chainID int64) []BridgeConfig {
	if bridges, ok := s.supportedBridges[chainID]; ok {
		return bridges
	}
	return []BridgeConfig{}
}

// getSymbolFromAddress resolves a token address to its symbol from the shared
// chain registry, keyed by the source chain, so a bridge quote prices the
// token that is actually being bridged rather than a hardcoded list.
func (s *BridgeService) getSymbolFromAddress(chainID int64, address string) string {
	if symbol, ok := config.TokenSymbol(chainID, address); ok {
		return symbol
	}
	return ""
}

// API Handlers

func (s *BridgeService) GetQuoteHandler(c *gin.Context) {
	var req struct {
		SrcChain int64  `json:"src_chain" binding:"required"`
		DstChain int64  `json:"dst_chain" binding:"required"`
		Token    string `json:"token" binding:"required"`
		Amount   string `json:"amount" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}

	quote, err := s.GetQuote(c.Request.Context(), req.SrcChain, req.DstChain, req.Token, req.Amount)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}

	c.JSON(200, quote)
}

// userWalletAddress resolves the custody wallet address bound to the
// authenticated caller. Bridge identity is the caller's own custody wallet, so
// a client can never claim another address as the sender of a transfer.
func (s *BridgeService) userWalletAddress(userID string) (string, error) {
	if s.db == nil {
		return "", fmt.Errorf("database not configured")
	}
	var wallet models.MasterWallet
	if err := s.db.Where("user_id = ?", userID).First(&wallet).Error; err != nil {
		return "", fmt.Errorf("no custody wallet for this account")
	}
	if !common.IsHexAddress(wallet.Address) {
		return "", fmt.Errorf("custody wallet address is not a valid address")
	}
	return wallet.Address, nil
}

// InitiateTransferHandler locks the caller's tokens on the source chain.
func (s *BridgeService) InitiateTransferHandler(c *gin.Context) {
	var req struct {
		SrcChain  int64  `json:"src_chain" binding:"required"`
		DstChain  int64  `json:"dst_chain" binding:"required"`
		Recipient string `json:"recipient" binding:"required"`
		Token     string `json:"token" binding:"required"`
		Amount    string `json:"amount" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}

	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	sender, err := s.userWalletAddress(userID)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	transfer, err := s.InitiateTransfer(c.Request.Context(), req.SrcChain, req.DstChain, sender, req.Recipient, req.Token, req.Amount)
	if err != nil {
		respondInternalError(c, err)
		return
	}

	c.JSON(200, transfer)
}

// GetTransferHandler returns one transfer, but only to a party of it.
func (s *BridgeService) GetTransferHandler(c *gin.Context) {
	transferID := c.Param("id")

	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	caller, err := s.userWalletAddress(userID)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	transfer, err := s.GetTransfer(transferID)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	if !strings.EqualFold(transfer.Sender, caller) && !strings.EqualFold(transfer.Recipient, caller) {
		c.JSON(404, gin.H{"error": "transfer not found"})
		return
	}

	c.JSON(200, transfer)
}

// GetUserTransfersHandler returns the caller's own transfers.
func (s *BridgeService) GetUserTransfersHandler(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	caller, err := s.userWalletAddress(userID)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	transfers := s.GetUserTransfers(caller)
	c.JSON(200, gin.H{
		"transfers": transfers,
		"count":     len(transfers),
	})
}

func (s *BridgeService) GetSupportedBridgesHandler(c *gin.Context) {
	chainID := c.Param("chain_id")

	var chainIDInt int64
	fmt.Sscanf(chainID, "%d", &chainIDInt)

	bridges := s.GetSupportedBridges(chainIDInt)
	c.JSON(200, gin.H{
		"chain_id": chainIDInt,
		"bridges":  bridges,
	})
}

func (s *BridgeService) GetSupportedChainsHandler(c *gin.Context) {
	chains := s.GetSupportedChains()
	c.JSON(200, gin.H{
		"chains": chains,
	})
}
