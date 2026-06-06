package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"golang.org/x/crypto/bcrypt"
)

var db *sql.DB

// Constants
const (
	SEED_PHRASE_WORDS = 24
	SEED_PHRASE_BITS  = 256
)

// Database connection
func initDB() error {
	var err error
	connStr := "host=localhost port=5432 user=tigerswap password=securepass dbname=tigerswap sslmode=disable"
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		return err
	}
	return db.Ping()
}

// BIP39 Word List (first 100 words for demo - full list would be 2048 words)
var bip39Words = []string{
	"abandon", "ability", "able", "about", "above", "absent", "absorb", "abstract",
	"absurd", "abuse", "access", "accident", "account", "accuse", "achieve", "acid",
	"acoustic", "acquire", "across", "act", "action", "actor", "actress", "actual",
	"adapt", "add", "addict", "address", "adjust", "admit", "adult", "advance",
	"advice", "aerobic", "affair", "afford", "afraid", "again", "age", "agent",
	"agree", "ahead", "aim", "air", "airport", "aisle", "alarm", "album",
	"alcohol", "alert", "alien", "all", "alley", "allow", "almost", "alone",
	"alpha", "already", "also", "alter", "always", "amateur", "amazing", "among",
	"amount", "amused", "analyst", "anchor", "ancient", "anger", "angle", "angry",
	"animal", "ankle", "announce", "annual", "another", "answer", "antenna", "antique",
	"anxiety", "any", "apart", "apology", "appear", "apple", "approve", "april",
	"arch", "arctic", "arena", "argue", "arm", "armed", "armor", "army",
	"around", "arrange", "arrest", "arrive", "arrow", "art", "artist", "artwork",
	"ask", "aspect", "assault", "asset", "assist", "assume", "asthma", "athlete",
	"atom", "attack", "attend", "attitude", "attract", "auction", "audit", "august",
	"aunt", "author", "auto", "autumn", "average", "avocado", "avoid", "awake",
	"aware", "away", "awesome", "awful", "awkward", "axis", "baby", "bachelor",
	"bacon", "badge", "bag", "balance", "balcony", "ball", "bamboo", "banana",
	"banner", "bar", "barely", "bargain", "barrel", "basic", "basket", "battle",
	"beach", "bean", "beauty", "because", "become", "beef", "before", "begin",
	"behave", "behind", "believe", "below", "belt", "bench", "benefit", "best",
	"betray", "better", "between", "beyond", "bicycle", "bid", "bike", "bind",
	"biology", "bird", "birth", "bitter", "black", "blade", "blame", "blanket",
	"blast", "blaze", "bless", "blind", "blood", "blossom", "blouse", "blue",
	"blur", "blush", "board", "boat", "body", "boil", "bomb", "bone",
	"bonus", "book", "boost", "border", "boring", "borrow", "boss", "bottom",
	"bounce", "box", "boy", "bracket", "brain", "brand", "brass", "brave",
	"bread", "breeze", "brick", "bridge", "brief", "bright", "bring", "brisk",
	"broccoli", "broken", "bronze", "broom", "brother", "brown", "brush", "bubble",
	"budget", "buffalo", "build", "bulb", "bulk", "bullet", "bundle", "bunker",
	"burden", "burger", "burst", "bus", "business", "busy", "butter", "buyer",
	"buzz", "cabbage", "cabin", "cable", "cactus", "cage", "cake", "call",
	"calm", "camera", "camp", "can", "canal", "cancel", "candy", "cannon",
	"canoe", "canvas", "canyon", "capable", "capital", "captain", "car", "carbon",
	"card", "cargo", "carpet", "carry", "cart", "case", "cash", "casino",
	"castle", "casual", "cat", "catalog", "catch", "category", "cattle", "caught",
	"cause", "caution", "cave", "ceiling", "celery", "cement", "census", "century",
	"cereal", "certain", "chair", "chalk", "champion", "change", "chaos", "chapter",
	"charge", "chase", "chat", "cheap", "check", "cheese", "chef", "cherry",
	"chest", "chicken", "chief", "child", "chimney", "choice", "choose", "chronic",
	"chuckle", "chunk", "churn", "cigar", "cinnamon", "circle", "citizen", "city",
	"civil", "claim", "clap", "clarify", "classic", "clean", "clerk", "clever",
	"click", "client", "cliff", "climb", "clinic", "clip", "clock", "close",
	"cloth", "cloud", "clown", "club", "clump", "cluster", "clutch", "coach",
	"coast", "coconut", "code", "coffee", "coil", "coin", "collect", "color",
	"column", "combine", "come", "comfort", "comic", "common", "company", "concert",
	"conduct", "confirm", "congress", "connect", "consider", "control", "convince", "cook",
	"cool", "copper", "copy", "coral", "core", "corn", "correct", "cost",
	"cotton", "couch", "country", "couple", "course", "cousin", "cover", "coyote",
	"crack", "cradle", "craft", "cram", "crane", "crash", "crater", "crawl",
	"crazy", "cream", "credit", "creek", "crew", "cricket", "crime", "crisp",
	"critic", "crop", "cross", "crouch", "crowd", "crucial", "cruel", "cruise",
	"crumble", "crunch", "crush", "cry", "crystal", "cube", "culture", "cup",
	"cupboard", "curious", "current", "curtain", "curve", "cushion", "custom", "cute",
	"cycle", "dad", "damage", "damp", "dance", "danger", "daring", "dash",
	"daughter", "dawn", "day", "deal", "debate", "debris", "decade", "december",
	"decide", "decline", "decorate", "decrease", "deer", "defense", "define", "defy",
	"degree", "delay", "deliver", "demand", "demise", "denial", "dentist", "deny",
	"depart", "depend", "deposit", "depth", "deputy", "derive", "describe", "desert",
	"design", "desk", "despair", "destroy", "detail", "detect", "develop", "device",
	"devote", "diagram", "dial", "diamond", "diary", "dice", "diesel", "diet",
	"differ", "digital", "dignity", "dilemma", "dinner", "dinosaur", "direct", "dirt",
	"disagree", "discover", "disease", "dish", "dismiss", "disorder", "display", "distance",
	"divert", "divide", "divorce", "dizzy", "doctor", "document", "dodge", "does",
	"dog", "doll", "dolphin", "domain", "donate", "donkey", "donor", "door",
	"dose", "double", "dove", "draft", "dragon", "drama", "draw", "dream",
	"dress", "drift", "drill", "drink", "drip", "drive", "drop", "drum",
	"dry", "duck", "dumb", "dune", "during", "dust", "dutch", "duty",
	"dwarf", "dynamic", "eager", "eagle", "early", "earn", "earth", "easily",
	"east", "easy", "echo", "ecology", "economy", "edge", "edit", "educate",
	"effort", "eight", "eject", "elastic", "elbow", "elder", "electric", "elegant",
	"element", "elephant", "elevator", "elite", "else", "embark", "embody", "embrace",
	"emerge", "emotion", "employ", "empower", "empty", "enable", "enact", "end",
	"endless", "endorse", "enemy", "energy", "enforce", "engage", "engine", "enhance",
	"enjoy", "enlist", "enough", "enrich", "enroll", "ensure", "enter", "entire",
	"entry", "envelope", "episode", "equal", "equip", "era", "erase", "erode",
	"erosion", "error", "erupt", "escape", "essay", "essence", "estate", "eternal",
	"ethics", "evidence", "evil", "evoke", "evolve", "exact", "examine", "example",
	"excess", "exchange", "excite", "exclude", "excuse", "execute", "exercise", "exhaust",
	"exhibit", "exile", "exist", "exit", "exotic", "expand", "expect", "expire",
	"explain", "expose", "express", "extend", "extra", "eye", "eyebrow", "fabric",
	"face", "faculty", "fade", "faint", "faith", "fall", "false", "fame",
	"family", "famous", "fan", "fancy", "fantasy", "farm", "fashion", "fat",
	"fatal", "father", "fatigue", "fault", "favorite", "feature", "february", "federal",
	"fee", "feed", "feel", "female", "fence", "festival", "fetch", "fever",
	"few", "fiber", "fiction", "field", "figure", "file", "film", "filter",
	"final", "finance", "find", "fine", "finger", "finish", "fire", "firm",
	"first", "fiscal", "fish", "fit", "fitness", "fix", "flag", "flame",
	"flash", "flat", "flavor", "flee", "flight", "flip", "float", "flock",
	"flood", "floor", "flower", "fluid", "flush", "fly", "foam", "focus",
	"fog", "foil", "fold", "follow", "foot", "force", "forest", "forget",
	"fork", "fortune", "forum", "forward", "fossil", "foster", "found", "fox",
	"fragile", "frame", "frequent", "fresh", "friend", "fringe", "frog", "front",
	"frost", "frown", "frozen", "fruit", "fuel", "fun", "funny", "furnace",
	"fury", "future", "gadget", "gain", "galaxy", "gallery", "game", "gap",
	"garage", "garbage", "garden", "gas", "gasp", "gate", "gather", "gauge",
	"gaze", "general", "genius", "genre", "gentle", "genuine", "gesture", "ghost",
	"giant", "gift", "giggle", "ginger", "giraffe", "girl", "give", "glad",
	"glance", "glare", "glass", "glide", "glimpse", "globe", "gloom", "glory",
	"glove", "glow", "glue", "goat", "goddess", "gold", "good", "goose",
	"gorilla", "gospel", "gossip", "govern", "gown", "grab", "grace", "grain",
	"grant", "grape", "grass", "gravity", "great", "green", "grid", "grief",
	"grit", "grocery", "group", "grow", "grunt", "guard", "guess", "guide",
	"guilt", "guitar", "gun", "gym", "habit", "hair", "half", "hammer",
	"hamster", "hand", "handle", "harbor", "hard", "harm", "harp", "harvest",
	"hat", "have", "hawk", "hazard", "head", "health", "heart", "heavy",
	"hedgehog", "height", "hello", "helmet", "help", "hen", "hero", "hidden",
	"high", "hill", "hint", "hip", "hire", "history", "hobby", "hockey",
	"hold", "hole", "holiday", "hollow", "home", "honey", "hood", "hope",
	"horn", "horror", "horse", "hospital", "host", "hotel", "hour", "hover",
	"hub", "huge", "human", "humble", "humor", "hundred", "hungry", "hunt",
	"hurdle", "hurry", "hurt", "husband", "hybrid", "ice", "icon", "idea",
	"identify", "idle", "ignore", "ill", "illegal", "illness", "image", "imitate",
	"immense", "immune", "impact", "impose", "improve", "impulse", "inch", "include",
	"income", "increase", "index", "indicate", "indoor", "industry", "infant", "inflict",
	"inform", "inhale", "inherit", "initial", "inject", "injury", "inmate", "inner",
	"innocent", "input", "inquiry", "insane", "insect", "insert", "inside", "inspire",
	"install", "intact", "interest", "into", "invest", "invite", "involve", "iron",
	"island", "isolate", "issue", "item", "ivory", "jacket", "jaguar", "jar",
	"jazz", "jealous", "jeans", "jelly", "jewel", "job", "join", "joke",
	"jolly", "journey", "joy", "judge", "juice", "jump", "jungle", "junior",
	"junk", "just", "kangaroo", "keen", "keep", "ketchup", "key", "kick",
	"kid", "kidney", "kind", "kingdom", "kiss", "kit", "kitchen", "kite",
	"kitten", "kiwi", "knee", "knife", "knock", "know", "lab", "label",
	"labor", "ladder", "lady", "lake", "lamp", "language", "laptop", "large",
	"later", "latin", "laugh", "laundry", "lava", "law", "lawn", "lawsuit",
	"layer", "lazy", "leader", "leaf", "learn", "leave", "lecture", "left",
	"leg", "legal", "legend", "leisure", "lemon", "lend", "length",
	"lens", "leopard", "lesson", "letter", "level", "liar", "liberty", "library",
	"license", "life", "lift", "light", "like", "limb", "limit", "link",
	"lion", "liquid", "list", "little", "live", "lizard", "load", "loan",
	"lobster", "local", "lock", "logic", "lonely", "long", "loop", "lottery",
	"loud", "lounge", "love", "loyal", "luck", "luggage", "lumber",
	"lunar", "lunch", "luxury", "lyrics", "machine", "mad", "magic", "magnet",
	"maid", "mail", "main", "major", "make", "mammal", "man", "manage",
	"mandate", "mango", "mansion", "manual", "maple", "marble", "march", "margin",
	"marine", "market", "marriage", "mask", "mass", "master", "match", "material", "math",
	"matrix", "matter", "maximum", "maze", "meadow", "mean", "measure", "meat",
	"mechanic", "medal", "media", "melody", "melt", "member", "memory", "men",
	"mend", "mental", "mentor", "menu", "mercy", "merge", "merit", "merry",
	"mesh", "message", "metal", "method", "middle", "midnight", "milk", "million",
	"mimic", "mind", "minimum", "minor", "minute", "miracle", "mirror", "misery",
	"miss", "mistake", "mix", "mixed", "mixture", "mobile", "model", "modify",
	"mom", "moment", "monitor", "monkey", "monster", "month", "moon", "moral",
	"more", "morning", "mosquito", "mother", "motion", "motor", "mountain", "mouse",
	"move", "movie", "much", "muffin", "mule", "multiply", "muscle",
	"museum", "mushroom", "music", "must", "mutual", "myself", "mystery", "myth",
	"naive", "name", "napkin", "narrow", "nasty", "nation", "nature", "near",
	"neat", "neck", "need", "negative", "neglect", "neither", "nephew", "nerve",
	"nest", "net", "network", "neutral", "never", "news", "next", "nice",
	"night", "noble", "noise", "nominee", "noodle", "normal", "north", "nose",
	"notable", "note", "nothing", "notice", "novel", "now", "nuclear", "number",
	"nurse", "nut", "oak", "obey", "object", "oblige", "obtain", "obvious",
	"occur", "ocean", "october", "odor", "off", "offer", "office", "often", "oil",
	"okay", "old", "olive", "olympic", "omit", "once", "one", "onion",
	"online", "only", "open", "opera", "opinion", "oppose", "option", "orange",
	"orbit", "orchard", "order", "ordinary", "organ", "orient", "original", "orphan",
	"ostrich", "other", "outdoor", "outer", "output", "outside", "oval", "oven",
	"over", "own", "owner", "owl", "oxygen", "oyster", "ozone", "pact",
	"paddle", "page", "pair", "palace", "palm", "panda", "panel", "panic",
	"panther", "paper", "parade", "parent", "park", "parrot", "party", "pass",
	"patch", "path", "patient", "patrol", "pattern", "pause", "pave", "payment",
	"peace", "peanut", "pear", "peasant", "pelican", "pen", "penalty", "pencil",
	"people", "pepper", "perfect", "permit", "person", "pet", "phone", "photo",
	"phrase", "physical", "piano", "picnic", "picture", "piece", "pig", "pigeon",
	"pill", "pilot", "pink", "pioneer", "pipe", "pistol", "pitch", "pizza",
	"place", "planet", "plastic", "plate", "play", "please", "pledge", "plenty",
	"plot", "plow", "pluck", "plug", "plunge", "poem", "poet", "point",
	"polar", "pole", "police", "pond", "pony", "pool", "popular", "portion",
	"position", "possible", "post", "potato", "pottery", "poverty", "powder",
	"power", "practice", "praise", "predict", "prefer", "prepare", "present", "press",
	"pretty", "prevent", "price", "pride", "primary", "print", "priority", "prison",
	"private", "prize", "problem", "process", "produce", "profit", "program", "project",
	"promote", "proof", "property", "prosper", "protect", "proud", "provide", "public",
	"pudding", "pull", "pulp", "pulse", "pumpkin", "punch", "pupil", "puppy",
	"purchase", "purity", "purpose", "purse", "push", "put", "puzzle", "pyramid",
	"quality", "quantum", "quarter", "queen", "query", "question", "quick", "quit", "quiz",
	"quote", "rabbit", "raccoon", "race", "rack", "radar", "radio", "rail",
	"rain", "raise", "rally", "ramp", "ranch", "random", "range", "rapid",
	"rare", "rate", "rather", "raven", "raw", "reach", "react", "read",
	"ready", "real", "realm", "rear", "reason", "rebel", "rebuild", "recall",
	"receive", "recipe", "record", "recover", "recruit", "red", "reduce", "reflect",
	"reform", "refuse", "region", "regret", "reject", "relax", "release", "relief",
	"rely", "remain", "remember", "remind", "remote", "remove", "render", "renew",
	"rent", "reopen", "repay", "repeat", "replace", "reply", "report", "represent",
	"reproduce", "public", "require", "rescue", "resemble", "resist", "resource", "response",
	"result", "retire", "retreat", "return", "reunion", "reveal", "review", "reward",
	"rhythm", "rib", "ribbon", "rice", "rich", "ride", "ridge", "rifle",
	"right", "rigid", "ring", "riot", "ripple", "risk", "ritual", "rival",
	"river", "road", "roast", "robot", "robust", "rocket", "romance", "roof",
	"rookie", "room", "root", "rope", "rose", "rotate", "rough", "round",
	"route", "royal", "rubber", "rubble", "ruby", "rude", "rug", "rule",
	"run", "runway", "rural", "sad", "saddle", "sadness", "safe", "sail",
	"salad", "salmon", "salon", "salt", "salute", "same", "sample", "sand",
	"satisfy", "satoshi", "sauce", "sausage", "save", "say", "scale", "scan",
	"scare", "scatter", "scene", "scheme", "school", "science", "scissors", "scorpion",
	"scout", "scrap", "screen", "script", "scrub", "sea", "search", "season",
	"seat", "second", "secret", "section", "security", "seed", "seek", "segment",
	"select", "sell", "seminar", "senior", "sense", "sentence", "series", "service",
	"session", "settle", "setup", "seven", "shadow", "shaft", "shallow", "share",
	"shark", "sharp", "sheep", "sheer", "sheet", "shelf", "shell", "sheriff",
	"shield", "shift", "shine", "ship", "shiver", "shock", "shoe", "shoot",
	"shop", "short", "shot", "shoulder", "shove", "shrimp", "shrug", "shuffle",
	"shy", "sibling", "sick", "side", "siege", "sight", "sign", "silent",
	"silk", "silly", "silver", "similar", "simple", "since", "sing", "siren",
	"sister", "situate", "six", "size", "skate", "sketch", "ski", "skill",
	"skin", "skirt", "skull", "slab", "slam", "sleep", "slender", "slice",
	"slide", "slight", "slim", "slogan", "slot", "slow", "slush", "small",
	"smart", "smell", "smile", "smoke", "smooth", "snack", "snake", "snap",
	"sniff", "snow", "so", "soap", "soccer", "social", "sock", "soda",
	"soft", "solar", "soldier", "solid", "solution", "solve", "someone", "song",
	"soon", "sorry", "sort", "soul", "sound", "soup", "source", "south",
	"space", "spare", "spark", "speak", "special", "speed", "spell", "spend",
	"sphere", "spice", "spider", "spike", "spin", "spirit", "split", "spoil",
	"sponsor", "spoon", "sport", "spot", "spouse", "spread", "spring", "spy",
	"square", "squeeze", "squirrel", "stable", "stadium", "staff", "stage", "stairs",
	"stamp", "stand", "start", "state", "stay", "steak", "steal", "steam",
	"steel", "steep", "stem", "step", "stereo", "stick", "still", "sting",
	"stock", "stomach", "stone", "stool", "story", "stove", "strategy", "street",
	"strike", "strong", "struggle", "student", "stuff", "stumble", "style", "subject",
	"submit", "subway", "success", "such", "sudden", "suffer", "sugar", "suggest",
	"suit", "summer", "sun", "sunny", "sunset", "super", "supply", "supreme",
	"sure", "surface", "surge", "surprise", "surround", "survey", "suspect", "sustain",
	"swallow", "swamp", "swap", "swarm", "swear", "sweat", "sweep", "sweet",
	"swell", "swift", "swim", "swing", "switch", "sword", "symbol", "symptom",
	"syrup", "system", "table", "tackle", "tag", "tail", "talent", "talk",
	"tank", "tape", "target", "task", "taste", "tattoo", "taxi", "teach",
	"team", "tell", "ten", "tenant", "tennis", "tent", "term", "test", "text",
	"thank", "that", "them", "theme", "then", "theory", "there", "they", "thing",
	"this", "thought", "three", "thrive", "throw", "thumb", "thunder", "ticket",
	"tide", "tiger", "tilt", "timber", "time", "tiny", "tip", "tired",
	"tissue", "title", "toast", "tobacco", "toddler", "toe", "together", "toilet",
	"token", "tomato", "tomorrow", "tone", "tongue", "tonight", "tool", "tooth",
	"top", "topic", "topple", "torch", "tornado", "tortoise", "toss", "total",
	"tourist", "toward", "tower", "town", "toy", "track", "trade", "traffic",
	"tragic", "train", "transfer", "trap", "trash", "travel", "tray", "treat",
	"tree", "trend", "trial", "tribe", "trick", "trigger", "trim", "trip",
	"troop", "trophy", "trouble", "truck", "true", "truly", "trumpet", "trust",
	"truth", "try", "tube", "tuition", "tumble", "tuna", "tunnel", "turkey",
	"turn", "turtle", "twelve", "twenty", "twice", "twin", "twist", "two", "type",
	"typical", "ugly", "umbrella", "unable", "unaware", "uncle", "uncover", "under",
	"unfair", "unfold", "unhappy", "uniform", "unique", "unit", "universe", "unknown",
	"unlock", "until", "unusual", "unveil", "update", "upgrade", "uphold", "upon",
	"upper", "upset", "urban", "urge", "usage", "use", "used", "useful",
	"useless", "usual", "utility", "vacant", "vacuum", "vague", "valid", "valley",
	"valve", "van", "vanish", "vapor", "various", "vegan", "velvet", "vendor",
	"venture", "venue", "verb", "verify", "version", "very", "vessel", "veteran",
	"viable", "vibrant", "vicious", "victory", "video", "view", "village", "vintage",
	"violin", "virtual", "virus", "visa", "visit", "visual", "vital", "vivid",
	"vocal", "voice", "void", "volcano", "volume", "vote", "voyage", "wage",
	"wagon", "wait", "walk", "wall", "walnut", "want", "warfare", "warm", "warrior",
	"wash", "wasp", "waste", "water", "wave", "way", "wealth", "weapon",
	"wear", "weasel", "weather", "web", "wedding", "weekend", "weird", "welcome",
	"west", "wet", "whale", "what", "wheat", "wheel", "when", "where",
	"whip", "whisper", "whistle", "white", "who", "whole", "whom", "whose",
	"why", "wicked", "wide", "widow", "width", "wife", "wild", "will",
	"win", "window", "wine", "wing", "wink", "winner", "winter", "wire", "wisdom",
	"wise", "wish", "witch", "withdraw", "witness", "wolf", "woman", "wonder",
	"wood", "wool", "word", "work", "world", "worry", "worth", "wrap",
	"wreck", "wrestle", "wrist", "write", "wrong", "yard", "year", "yell",
	"yellow", "you", "young", "youth", "zebra", "zero", "zone", "zoo",
}

// Generate seed phrase
func generateSeedPhrase() string {
	var words []string
	for i := 0; i < SEED_PHRASE_WORDS; i++ {
		words = append(words, bip39Words[randInt(len(bip39Words))])
	}
	return strings.Join(words, " ")
}

// Generate wallet from seed
func generateWalletFromSeed(seed string, chain string, chainID int, index int) (string, string, error) {
	// Derive key using BIP32/BIP44
	seedHash := sha256.Sum256([]byte(seed))
	
	// Derive path based on chain
	path := fmt.Sprintf("m/44'/%d'/0'/0'/%d'", chainID, index)
	pathHash := sha256.Sum256([]byte(path))
	
	var privateKey []byte
	for i := 0; i < len(seedHash); i++ {
		privateKey = append(privateKey, seedHash[i]^pathHash[i%len(pathHash)])
	}
	
	// Generate address based on chain type
	var address string
	switch strings.ToLower(chain) {
	case "ethereum", "bsc", "polygon", "arbitrum", "base", "avalanche", "optimism":
		address = generateEVMAddress(privateKey)
	case "solana":
		address = generateSolanaAddress(privateKey)
	case "bitcoin", "litecoin":
		address = generateBTCAddress(privateKey)
	case "ton":
		address = generateTONAddress(privateKey)
	case "aptos", "sui":
		address = generateMoveAddress(privateKey, chain)
	default:
		address = generateEVMAddress(privateKey)
	}
	
	return address, hex.EncodeToString(privateKey[:32]), nil
}

func generateEVMAddress(key []byte) string {
	hash := sha256.Sum256(key)
	return "0x" + hex.EncodeToString(hash[:20])
}

func generateSolanaAddress(key []byte) string {
	hash := sha256.Sum256(key)
	return base58Encode(hash[:32])
}

func generateBTCAddress(key []byte) string {
	hash := sha256.Sum256(key)
	return "1" + base58Encode(hash[:20])
}

func generateTONAddress(key []byte) string {
	hash := sha256.Sum256(key)
	return base64Encode(hash[:32])
}

func generateMoveAddress(key []byte, chain string) string {
	hash := sha256.Sum256(key)
	return chain + "_" + hex.EncodeToString(hash[:16])
}

func base58Encode(data []byte) string {
	alphabet := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	result := ""
	n := new(big.Int).SetBytes(data)
	for n.Cmp(big.NewInt(0)) > 0 {
		mod := new(big.Int)
		n.DivMod(n, big.NewInt(58), mod)
		result = string(alphabet[mod.Int64()]) + result
	}
	return result
}

func base64Encode(data []byte) string {
	encoded := make([]byte, len(data)*2)
	j := 0
	for _, b := range data {
		encoded[j] = b
		j++
	}
	return string(encoded[:j])
}

func randInt(max int) int {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max)))
	return int(n.Int64())
}

// Handlers
func createWalletHandler(c *gin.Context) {
	userID := getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	var input struct {
		Name    string `json:"name"`
		Chain  string `json:"chain" binding:"required"`
		SeedPhrase string `json:"seed_phrase"`
	}
	
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Get or generate seed phrase
	seed := input.SeedPhrase
	if seed == "" {
		// Check if user already has seed
		var existingSeed string
		err := db.QueryRow("SELECT seed_phrase_encrypted FROM wallets WHERE user_id = $1 AND is_primary = true", userID).Scan(&existingSeed)
		if err == nil && existingSeed != "" {
			seed = existingSeed
		} else {
			seed = generateSeedPhrase()
		}
	}
	
	// Get chain info
	var chainID int
	var chainType string
	err := db.QueryRow("SELECT chain_id, type FROM blockchains WHERE name = $1 AND is_active = true", input.Chain).Scan(&chainID, &chainType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid chain"})
		return
	}
	
	// Get wallet count for derivation index
	var count int
	db.QueryRow("SELECT COUNT(*) FROM wallets WHERE user_id = $1 AND chain = $2", userID, input.Chain).Scan(&count)
	
	// Generate wallet address
	address, privateKey, err := generateWalletFromSeed(seed, input.Chain, chainID, count)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Wallet generation failed"})
		return
	}
	
	walletName := input.Name
	if walletName == "" {
		walletName = "Wallet " + fmt.Sprint(count+1)
	}
	
	// Encrypt private key
	encryptedKey, err := encryptPrivateKey(privateKey, c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Encryption failed"})
		return
	}
	
	// Save wallet
	var walletID int
	err = db.QueryRow(`
		INSERT INTO wallets (user_id, wallet_type, name, address, chain, chain_id, encrypted_private_key, seed_phrase_encrypted, is_primary)
		VALUES ($1, 'user', $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`,
		userID, walletName, address, input.Chain, chainID, encryptedKey, seed, count == 0,
	).Scan(&walletID)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Wallet creation failed"})
		return
	}
	
	logAudit(userID, "wallet_create", "wallets", walletID, gin.H{"chain": input.Chain, "address": address})
	
	c.JSON(http.StatusCreated, gin.H{
		"message":     "Wallet created successfully",
		"wallet_id":   walletID,
		"address":     address,
		"chain":      input.Chain,
		"chain_id":    chainID,
	})
}

func importWalletHandler(c *gin.Context) {
	userID := getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	var input struct {
		Name        string `json:"name"`
		PrivateKey  string `json:"private_key" binding:"required"`
		SeedPhrase string `json:"seed_phrase"`
		Chain      string `json:"chain" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Get chain info
	var chainID int
	err := db.QueryRow("SELECT chain_id FROM blockchains WHERE name = $1 AND is_active = true", input.Chain).Scan(&chainID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid chain"})
		return
	}
	
	// Validate private key format
	privateKeyBytes, err := hex.DecodeString(input.PrivateKey)
	if err != nil || len(privateKeyBytes) < 32 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid private key"})
		return
	}
	
	// Generate address from private key
	address, _, err := generateWalletFromSeed(input.SeedPhrase, input.Chain, chainID, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Address generation failed"})
		return
	}
	
	// Encrypt private key
	encryptedKey, err := encryptPrivateKey(input.PrivateKey, c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Encryption failed"})
		return
	}
	
	walletName := input.Name
	if walletName == "" {
		walletName = "Imported Wallet"
	}
	
	// Save wallet
	var walletID int
	err = db.QueryRow(`
		INSERT INTO wallets (user_id, wallet_type, name, address, chain, chain_id, encrypted_private_key, seed_phrase_encrypted)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`,
		userID, "imported", walletName, address, input.Chain, chainID, encryptedKey, input.SeedPhrase,
	).Scan(&walletID)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Wallet import failed"})
		return
	}
	
	logAudit(userID, "wallet_import", "wallets", walletID, gin.H{"chain": input.Chain, "address": address})
	
	c.JSON(http.StatusCreated, gin.H{
		"message":   "Wallet imported successfully",
		"wallet_id": walletID,
		"address":  address,
	})
}

func getWalletsHandler(c *gin.Context) {
	userID := getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	rows, err := db.Query(`
		SELECT id, name, address, chain, chain_id, is_primary, created_at
		FROM wallets WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()
	
	var wallets []map[string]interface{}
	for rows.Next() {
		var w struct {
			ID        int
			Name     string
			Address  string
			Chain   string
			ChainID int
			IsPrimary bool
			CreatedAt time.Time
		}
		rows.Scan(&w.ID, &w.Name, &w.Address, &w.Chain, &w.ChainID, &w.IsPrimary, &w.CreatedAt)
		wallets = append(wallets, map[string]interface{}{
			"wallet_id":   w.ID,
			"name":      w.Name,
			"address":   w.Address,
			"chain":     w.Chain,
			"chain_id":  w.ChainID,
			"is_primary": w.IsPrimary,
			"created_at": w.CreatedAt,
		})
	}
	
	c.JSON(http.StatusOK, gin.H{"wallets": wallets})
}

func getBalanceHandler(c *gin.Context) {
	userID := getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	walletID := c.Param("id")
	var address, chain string
	err := db.QueryRow("SELECT address, chain FROM wallets WHERE id = $1 AND user_id = $2", walletID, userID).Scan(&address, &chain)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Wallet not found"})
		return
	}
	
	// Get balance from chain (simplified - would query RPC in production)
	balances := map[string]interface{}{
		"address": address,
		"chain":  chain,
		"balances": []map[string]interface{}{
			{"symbol": "ETH", "balance": "0.0", "balance_raw": "0"},
		},
	}
	
	c.JSON(http.StatusOK, balances)
}

func sendTransactionHandler(c *gin.Context) {
	userID := getUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	var input struct {
		WalletID   string `json:"wallet_id" binding:"required"`
		ToAddress string `json:"to_address" binding:"required"`
		Amount   string `json:"amount" binding:"required"`
		Token    string `json:"token"`
		Chain    string `json:"chain" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Get wallet
	var address, encryptedKey string
	var chainID int
	err := db.QueryRow(`
		SELECT address, encrypted_private_key, chain_id FROM wallets 
		WHERE id = $1 AND user_id = $2`,
		input.WalletID, userID,
	).Scan(&address, &encryptedKey, &chainID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Wallet not found"})
		return
	}
	
	// Decrypt private key
	privateKey, err := decryptPrivateKey(encryptedKey, c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Decryption failed"})
		return
	}
	
	// Create transaction (simplified - would use proper RPC in production)
	txHash := fmt.Sprintf("0x%x", sha256.Sum256([]byte(time.Now().String())))
	
	// Save transaction
	var txID int
	err = db.QueryRow(`
		INSERT INTO transactions (user_id, wallet_id, tx_type, chain, chain_id, from_address, to_address, amount, token, tx_hash, status)
		VALUES ($1, $2, 'send', $3, $4, $5, $6, $7, $8, $9, 'pending')
		RETURNING id`,
		userID, input.WalletID, input.Chain, chainID, address, input.ToAddress, input.Amount, input.Token, txHash,
	).Scan(&txID)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction creation failed"})
		return
	}
	
	logAudit(userID, "send_transaction", "transactions", txID, gin.H{
		"to":      input.ToAddress,
		"amount":  input.Amount,
		"chain":  input.Chain,
	})
	
	c.JSON(http.StatusCreated, gin.H{
		"message":    "Transaction created",
		"tx_id":     txID,
		"tx_hash":   txHash,
		"from":      address,
		"to":        input.ToAddress,
		"amount":    input.Amount,
		"chain":     input.Chain,
	})
}

// Helper functions
func getUserIDFromContext(c *gin.Context) int {
	sessionToken, _ := c.Cookie("session_token")
	if sessionToken == "" {
		return 0
	}
	
	var userID int
	var expiresAt time.Time
	err := db.QueryRow(`
		SELECT user_id, expires_at FROM sessions 
		WHERE session_token = $1 AND is_active = true AND expires_at > NOW()`,
		sessionToken,
	).Scan(&userID, &expiresAt)
	
	if err != nil {
		return 0
	}
	
	return userID
}

func encryptPrivateKey(key string, c *gin.Context) (string, error) {
	// Get encryption key from session or use default
	encryptionKey := []byte("tigerswap_secure_key_2026")
	
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	
	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce)
	
	return hex.EncodeToString(gcm.Seal(nonce, nonce, []byte(key), nil)), nil
}

func decryptPrivateKey(encrypted string, c *gin.Context) (string, error) {
	encryptionKey := []byte("tigerswap_secure_key_2026")
	
	data, err := hex.DecodeString(encrypted)
	if err != nil {
		return "", err
	}
	
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	
	return string(plaintext), nil
}

func logAudit(userID int, action, entityType string, entityID int, details map[string]interface{}) {
	detailsJSON, _ := json.Marshal(details)
	db.Exec(`
		INSERT INTO audit_logs (user_id, action, entity_type, entity_id, details)
		VALUES ($1, $2, $3, $4, $5)`,
		userID, action, entityType, entityID, string(detailsJSON),
	)
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := getUserIDFromContext(c)
		if userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}
		c.Set("user_id", userID)
		c.Next()
	}
}

func main() {
	r := gin.Default()
	
	if err := initDB(); err != nil {
		fmt.Println("Database connection failed:", err)
	}
	
	// Public routes
	r.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	
	// Wallet routes
	wallet := r.Group("/api/v1/wallet")
	wallet.Use(authMiddleware())
	{
		wallet.POST("/create", createWalletHandler)
		wallet.POST("/import", importWalletHandler)
		wallet.GET("/list", getWalletsHandler)
		wallet.GET("/:id/balance", getBalanceHandler)
		wallet.POST("/send", sendTransactionHandler)
	}
	
	fmt.Println("Wallet service running on :8081")
	r.Run(":8081")
}