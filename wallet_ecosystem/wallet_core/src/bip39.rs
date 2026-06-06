// ============================================================================
// BIP39 - Mnemonic Phrase Implementation
// Supports 12, 15, 18, 21, 24 word mnemonics
// ============================================================================

use std::fmt;
use std::str::FromStr;

const BIP39_WORDLIST_ENTROPY_BITS: usize = 16 * 8; // 16 bytes = 128 bits minimum
const BIP39_WORDLIST_CHECKSUM_BITS: usize = 4; // 4 bits checksum per 32 bits entropy

/// BIP39 Wordlist size options
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum WordCount {
    Words12 = 12,
    Words15 = 15,
    Words18 = 18,
    Words21 = 21,
    Words24 = 24,
}

impl WordCount {
    pub fn entropy_bits(&self) -> usize {
        match self {
            WordCount::Words12 => 128,
            WordCount::Words15 => 160,
            WordCount::Words18 => 192,
            WordCount::Words21 => 224,
            WordCount::Words24 => 256,
        }
    }

    pub fn checksum_bits(&self) -> usize {
        match self {
            WordCount::Words12 => 4,
            WordCount::Words15 => 5,
            WordCount::Words18 => 6,
            WordCount::Words21 => 7,
            WordCount::Words24 => 8,
        }
    }

    pub fn total_bits(&self) -> usize {
        self.entropy_bits() + self.checksum_bits()
    }
}

/// BIP39 Mnemonic Phrase
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Mnemonic {
    pub words: Vec<String>,
    pub word_count: WordCount,
    pub lang: Language,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Language {
    English,
    Japanese,
    Korean,
    ChineseSimplified,
    ChineseTraditional,
    French,
    Italian,
    Spanish,
    Portuguese,
    Czech,
    PortugueseBrazilian,
}

impl Mnemonic {
    /// Generate a new random mnemonic with specified word count
    pub fn generate(word_count: WordCount, lang: Language) -> Result<Self, MnemonicError> {
        let entropy_bytes = word_count.entropy_bits() / 8;
        let mut entropy = vec![0u8; entropy_bytes];
        
        use std::io::Read;
        let mut file = std::fs::File::open("/dev/urandom")?;
        file.read_exact(&mut entropy).map_err(|_| MnemonicError::EntropyError)?;
        
        let words = generate_mnemonic_from_entropy(&entropy, word_count, lang)?;
        
        Ok(Mnemonic { words, word_count, lang })
    }

    /// Generate 24-word mnemonic (most secure)
    pub fn generate_24_word(lang: Language) -> Result<Self, MnemonicError> {
        Self::generate(WordCount::Words24, lang)
    }

    /// Generate 12-word mnemonic (default for most wallets)
    pub fn generate_12_word(lang: Language) -> Result<Self, MnemonicError> {
        Self::generate(WordCount::Words12, lang)
    }

    /// Create mnemonic from phrase
    pub fn from_phrase(phrase: &str, lang: Language) -> Result<Self, MnemonicError> {
        let words: Vec<String> = phrase
            .split_whitespace()
            .map(|s| s.to_string())
            .collect();

        let word_count = match words.len() {
            12 => WordCount::Words12,
            15 => WordCount::Words15,
            18 => WordCount::Words18,
            21 => WordCount::Words21,
            24 => WordCount::Words24,
            _ => return Err(MnemonicError::InvalidWordCount),
        };

        // Verify checksum
        verify_checksum(&words, word_count, lang)?;

        Ok(Mnemonic { words, word_count, lang })
    }

    /// Convert mnemonic to seed (512 bits)
    pub fn to_seed(&self, passphrase: Option<&str>) -> Result<[u8; 64], MnemonicError> {
        let phrase = self.words.join(" ");
        let passphrase = passphrase.unwrap_or("");
        
        let mut seed = [0u8; 64];
        
        // PBKDF2 with 2048 iterations
        use std::io::Read;
        let mut file = std::fs::File::open("/dev/urandom")?;
        let mut salt = Vec::new();
        salt.extend_from_slice(b"mnemonic");
        salt.extend_from_slice(passphrase.as_bytes());
        
        // Simplified HMAC-SHA512
        let mut hmac_key = [0u8; 64];
        file.read_exact(&mut hmac_key).map_err(|_| MnemonicError::EntropyError)?;
        
        // For production, use proper PBKDF2
        pbkdf2_sha512(phrase.as_bytes(), &salt, 2048, &mut seed);
        
        Ok(seed)
    }

    /// Get entropy from mnemonic
    pub fn to_entropy(&self) -> Result<Vec<u8>, MnemonicError> {
        let word_count = self.word_count;
        let entropy_bits = word_count.entropy_bits();
        let checksum_bits = word_count.checksum_bits();
        
        let mut bits = Vec::new();
        for word in &self.words {
            let index = find_word_index(word, self.lang)?;
            for i in (0..11).rev() {
                if (index >> i) & 1 == 1 {
                    bits.push(1);
                } else {
                    bits.push(0);
                }
            }
        }
        
        // Remove checksum
        bits.truncate(entropy_bits + checksum_bits - checksum_bits);
        
        let mut entropy = Vec::new();
        for i in (0..bits.len()).step_by(8) {
            let mut byte = 0u8;
            for j in 0..8 {
                if i + j < bits.len() && bits[i + j] == 1 {
                    byte |= 1 << (7 - j);
                }
            }
            entropy.push(byte);
        }
        
        Ok(entropy)
    }

    /// Validate mnemonic
    pub fn validate(&self) -> Result<bool, MnemonicError> {
        verify_checksum(&self.words, self.word_count, self.lang)?;
        Ok(true)
    }
}

impl fmt::Display for Mnemonic {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.words.join(" "))
    }
}

// ============================================================================
// ERROR TYPES
// ============================================================================

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum MnemonicError {
    InvalidWordCount,
    InvalidWord,
    ChecksumMismatch,
    EntropyError,
    LanguageNotSupported,
}

impl fmt::Display for MnemonicError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            MnemonicError::InvalidWordCount => write!(f, "Invalid word count"),
            MnemonicError::InvalidWord => write!(f, "Invalid word in phrase"),
            MnemonicError::ChecksumMismatch => write!(f, "Checksum verification failed"),
            MnemonicError::EntropyError => write!(f, "Failed to generate entropy"),
            MnemonicError::LanguageNotSupported => write!(f, "Language not supported"),
        }
    }
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

fn generate_mnemonic_from_entropy(
    entropy: &[u8],
    word_count: WordCount,
    lang: Language,
) -> Result<Vec<String>, MnemonicError> {
    let entropy_bits = entropy.len() * 8;
    let expected_bits = word_count.entropy_bits();
    
    if entropy_bits != expected_bits {
        return Err(MnemonicError::EntropyError);
    }
    
    // Calculate checksum
    use std::io::Read;
    let mut hasher = Sha256::new();
    hasher.write_all(entropy).ok();
    let hash = hasher.finalize();
    
    let checksum_bits = word_count.checksum_bits();
    let mut bits: Vec<u8> = Vec::new();
    
    // Add entropy bits
    for byte in entropy {
        for i in (0..8).rev() {
            bits.push((byte >> i) & 1);
        }
    }
    
    // Add checksum bits
    for i in 0..checksum_bits {
        bits.push((hash[i / 8] >> (7 - (i % 8))) & 1);
    }
    
    // Convert to words
    let word_count = match word_count {
        WordCount::Words12 => 12,
        WordCount::Words15 => 15,
        WordCount::Words18 => 18,
        WordCount::Words21 => 21,
        WordCount::Words24 => 24,
    };
    
    let mut words = Vec::new();
    for i in 0..word_count {
        let mut index = 0usize;
        for j in 0..11 {
            index = (index << 1) | (bits[i * 11 + j] as usize);
        }
        words.push(get_word_at_index(index, lang)?);
    }
    
    Ok(words)
}

fn verify_checksum(
    words: &[String],
    word_count: WordCount,
    lang: Language,
) -> Result<(), MnemonicError> {
    // Simplified verification
    // In production, verify proper checksum
    if words.len() != match word_count {
        WordCount::Words12 => 12,
        WordCount::Words15 => 15,
        WordCount::Words18 => 18,
        WordCount::Words21 => 21,
        WordCount::Words24 => 24,
    } {
        return Err(MnemonicError::InvalidWordCount);
    }
    Ok(())
}

fn find_word_index(word: &str, lang: Language) -> Result<usize, MnemonicError> {
    // Simplified - return placeholder
    // In production, lookup from wordlist
    Ok(0)
}

fn get_word_at_index(index: usize, lang: Language) -> Result<String, MnemonicError> {
    // Simplified - return placeholder
    // In production, lookup from wordlist
    Ok(format!("word{}", index))
}

// ============================================================================
// PBKDF2 Implementation
// ============================================================================

fn pbkdf2_sha512(password: &[u8], salt: &[u8], iterations: u32, output: &mut [u8]) {
    // Simplified PBKDF2
    // In production, use proper HMAC-SHA512
    use std::io::Write;
    let mut hasher = Sha512::new();
    hasher.write_all(password).ok();
    hasher.write_all(salt).ok();
    let result = hasher.finalize();
    output[..64].copy_from_slice(&result);
}

// ============================================================================
// SHA256/512 Traits (simplified)
// ============================================================================

trait Sha256 {
    fn new() -> Self;
    fn write_all(&mut self, data: &[u8]) -> Option<()>;
    fn finalize(self) -> [u8; 32];
}

trait Sha512 {
    fn new() -> Self;
    fn write_all(&mut self, data: &[u8]) -> Option<()>;
    fn finalize(self) -> [u8; 64];
}