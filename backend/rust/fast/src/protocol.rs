//! WebSocket framing and protocol helpers used by the TigerSwap engine.
//!
//! `WSFrame` performs real RFC 6455 masking and unmasking, payload-length
//! encoding and streaming decode. Frames are validated on decode so a malformed
//! frame is reported instead of panicking.

use std::fmt;

/// Opcode for a WebSocket frame.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(u8)]
pub enum OpCode {
    Continuation = 0x0,
    Text = 0x1,
    Binary = 0x2,
    Close = 0x8,
    Ping = 0x9,
    Pong = 0xA,
}

impl OpCode {
    pub fn from_u8(value: u8) -> Option<Self> {
        match value {
            0x0 => Some(OpCode::Continuation),
            0x1 => Some(OpCode::Text),
            0x2 => Some(OpCode::Binary),
            0x8 => Some(OpCode::Close),
            0x9 => Some(OpCode::Ping),
            0xA => Some(OpCode::Pong),
            _ => None,
        }
    }

    /// Control frames must not be fragmented.
    pub fn is_control(self) -> bool {
        matches!(self, OpCode::Close | OpCode::Ping | OpCode::Pong)
    }
}

/// High-level message type.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum MessageType {
    Text,
    Binary,
    Ping,
    Pong,
    Close,
}

impl MessageType {
    pub fn opcode(self) -> OpCode {
        match self {
            MessageType::Text => OpCode::Text,
            MessageType::Binary => OpCode::Binary,
            MessageType::Ping => OpCode::Ping,
            MessageType::Pong => OpCode::Pong,
            MessageType::Close => OpCode::Close,
        }
    }

    pub fn from_opcode(opcode: OpCode) -> Option<Self> {
        match opcode {
            OpCode::Text | OpCode::Continuation => Some(MessageType::Text),
            OpCode::Binary => Some(MessageType::Binary),
            OpCode::Ping => Some(MessageType::Ping),
            OpCode::Pong => Some(MessageType::Pong),
            OpCode::Close => Some(MessageType::Close),
        }
    }
}

/// A WebSocket message payload.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct WSMessage {
    pub msg_type: MessageType,
    pub data: Vec<u8>,
}

impl WSMessage {
    pub fn text(data: impl Into<String>) -> Self {
        Self {
            msg_type: MessageType::Text,
            data: data.into().into_bytes(),
        }
    }

    pub fn binary(data: impl Into<Vec<u8>>) -> Self {
        Self {
            msg_type: MessageType::Binary,
            data: data.into(),
        }
    }

    pub fn ping() -> Self {
        Self {
            msg_type: MessageType::Ping,
            data: Vec::new(),
        }
    }

    pub fn pong() -> Self {
        Self {
            msg_type: MessageType::Pong,
            data: Vec::new(),
        }
    }

    pub fn close(code: u16) -> Self {
        Self {
            msg_type: MessageType::Close,
            data: code.to_be_bytes().to_vec(),
        }
    }
}

/// Errors raised while decoding a WebSocket frame.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum FrameError {
    /// The buffer did not contain a complete frame.
    Incomplete,
    /// The opcode is not defined by RFC 6455.
    UnknownOpcode(u8),
    /// A control frame exceeded the 125-byte limit or was fragmented.
    InvalidControlFrame,
    /// A client frame was not masked, or a server frame was.
    InvalidMasking,
    /// The payload length used the 64-bit form when a shorter form would do.
    NonMinimalLength,
    /// The payload extended beyond the advertised frame length.
    LengthMismatch,
}

impl fmt::Display for FrameError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            FrameError::Incomplete => write!(f, "incomplete websocket frame"),
            FrameError::UnknownOpcode(op) => write!(f, "unknown websocket opcode {op}"),
            FrameError::InvalidControlFrame => write!(f, "invalid websocket control frame"),
            FrameError::InvalidMasking => write!(f, "invalid websocket masking"),
            FrameError::NonMinimalLength => write!(f, "non-minimal websocket length encoding"),
            FrameError::LengthMismatch => write!(f, "websocket payload length mismatch"),
        }
    }
}

impl std::error::Error for FrameError {}

/// A single WebSocket frame.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct WSFrame {
    pub fin: bool,
    pub opcode: u8,
    pub masked: bool,
    pub masking_key: [u8; 4],
    pub payload: Vec<u8>,
}

impl WSFrame {
    /// Builds a frame, masking the payload when `mask` is set (client to server).
    pub fn new(message: &WSMessage, mask: bool) -> Self {
        let payload = message.data.clone();
        let masking_key = if mask {
            random_mask()
        } else {
            [0, 0, 0, 0]
        };

        let masked_payload = if mask {
            apply_mask(&payload, &masking_key)
        } else {
            payload
        };

        Self {
            fin: true,
            opcode: message.msg_type.opcode() as u8,
            masked: mask,
            masking_key,
            payload: masked_payload,
        }
    }

    /// Encodes the frame to its wire representation.
    pub fn encode(&self) -> Vec<u8> {
        let length = self.payload.len();
        let mut out = Vec::with_capacity(length + 14);

        let first = if self.fin { 0x80 } else { 0x00 } | (self.opcode & 0x0F);
        out.push(first);

        let mask_bit = if self.masked { 0x80 } else { 0x00 };
        if length < 126 {
            out.push(mask_bit | length as u8);
        } else if length <= u16::MAX as usize {
            out.push(mask_bit | 126);
            out.extend_from_slice(&(length as u16).to_be_bytes());
        } else {
            out.push(mask_bit | 127);
            out.extend_from_slice(&(length as u64).to_be_bytes());
        }

        if self.masked {
            out.extend_from_slice(&self.masking_key);
        }

        out.extend_from_slice(&self.payload);
        out
    }

    /// Decodes one frame from `buffer`, returning the frame and bytes consumed.
    pub fn decode(buffer: &[u8]) -> Result<(Self, usize), FrameError> {
        if buffer.len() < 2 {
            return Err(FrameError::Incomplete);
        }

        let fin = buffer[0] & 0x80 != 0;
        let opcode = buffer[0] & 0x0F;
        if OpCode::from_u8(opcode).is_none() {
            return Err(FrameError::UnknownOpcode(opcode));
        }

        let masked = buffer[1] & 0x80 != 0;
        let length_marker = buffer[1] & 0x7F;

        let mut offset = 2usize;
        let payload_len = match length_marker {
            126 => {
                if buffer.len() < offset + 2 {
                    return Err(FrameError::Incomplete);
                }
                let value = u16::from_be_bytes([buffer[offset], buffer[offset + 1]]);
                offset += 2;
                if value < 126 {
                    return Err(FrameError::NonMinimalLength);
                }
                value as usize
            }
            127 => {
                if buffer.len() < offset + 8 {
                    return Err(FrameError::Incomplete);
                }
                let mut bytes = [0u8; 8];
                bytes.copy_from_slice(&buffer[offset..offset + 8]);
                offset += 8;
                let value = u64::from_be_bytes(bytes);
                if value <= u16::MAX as u64 {
                    return Err(FrameError::NonMinimalLength);
                }
                value as usize
            }
            other => other as usize,
        };

        if let Some(code) = OpCode::from_u8(opcode) {
            if code.is_control() && (!fin || payload_len > 125) {
                return Err(FrameError::InvalidControlFrame);
            }
        }

        let mut masking_key = [0u8; 4];
        if masked {
            if buffer.len() < offset + 4 {
                return Err(FrameError::Incomplete);
            }
            masking_key.copy_from_slice(&buffer[offset..offset + 4]);
            offset += 4;
        }

        if buffer.len() < offset + payload_len {
            return Err(FrameError::Incomplete);
        }

        let raw = &buffer[offset..offset + payload_len];
        let payload = if masked {
            apply_mask(raw, &masking_key)
        } else {
            raw.to_vec()
        };

        Ok((
            Self {
                fin,
                opcode,
                masked,
                masking_key,
                payload,
            },
            offset + payload_len,
        ))
    }

    /// The message this frame carries.
    pub fn message(&self) -> Option<WSMessage> {
        let opcode = OpCode::from_u8(self.opcode)?;
        let msg_type = MessageType::from_opcode(opcode)?;
        Some(WSMessage {
            msg_type,
            data: self.payload.clone(),
        })
    }

    /// Validates a server-side frame: client frames must be masked.
    pub fn validate_client_frame(&self) -> Result<(), FrameError> {
        if !self.masked {
            return Err(FrameError::InvalidMasking);
        }
        if let Some(code) = OpCode::from_u8(self.opcode) {
            if code.is_control() && (!self.fin || self.payload.len() > 125) {
                return Err(FrameError::InvalidControlFrame);
            }
        }
        Ok(())
    }
}

/// XORs a payload with the 4-byte masking key.
pub fn apply_mask(payload: &[u8], key: &[u8; 4]) -> Vec<u8> {
    payload
        .iter()
        .enumerate()
        .map(|(index, byte)| byte ^ key[index % 4])
        .collect()
}

/// Generates a masking key from the operating system CSPRNG.
fn random_mask() -> [u8; 4] {
    let mut key = [0u8; 4];
    if getrandom(&mut key).is_err() {
        // Fall back to a time-derived key only if the OS RNG is unavailable.
        let nanos = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .map(|d| d.as_nanos() as u64)
            .unwrap_or(0);
        key.copy_from_slice(&nanos.to_le_bytes()[..4]);
    }
    key
}

fn getrandom(buffer: &mut [u8]) -> std::io::Result<()> {
    #[cfg(unix)]
    {
        use std::io::Read;
        let mut file = std::fs::File::open("/dev/urandom")?;
        file.read_exact(buffer)?;
        Ok(())
    }
    #[cfg(not(unix))]
    {
        // Non-Unix targets fall back to the timestamp path in `random_mask`.
        let _ = buffer;
        Err(std::io::Error::new(
            std::io::ErrorKind::Unsupported,
            "no system random source",
        ))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn encodes_short_text_frame() {
        let frame = WSFrame::new(&WSMessage::text("hi"), true);
        let bytes = frame.encode();

        assert_eq!(bytes[0], 0x81); // FIN + text
        assert_eq!(bytes[1] & 0x7F, 2);
        assert_eq!(bytes[1] & 0x80, 0x80); // masked
        assert_eq!(bytes.len(), 2 + 4 + 2);
    }

    #[test]
    fn round_trips_masked_text() {
        let original = WSMessage::text("TigerSwap order book");
        let frame = WSFrame::new(&original, true);
        let bytes = frame.encode();

        let (decoded, consumed) = WSFrame::decode(&bytes).unwrap();
        assert_eq!(consumed, bytes.len());
        assert_eq!(decoded.message().unwrap(), original);
    }

    #[test]
    fn round_trips_unmasked_binary() {
        let original = WSMessage::binary(vec![0, 1, 2, 250, 255]);
        let frame = WSFrame::new(&original, false);
        let bytes = frame.encode();

        let (decoded, _) = WSFrame::decode(&bytes).unwrap();
        assert!(!decoded.masked);
        assert_eq!(decoded.message().unwrap(), original);
    }

    #[test]
    fn round_trips_medium_payload() {
        let payload = vec![0xABu8; 500];
        let frame = WSFrame::new(&WSMessage::binary(payload.clone()), true);
        let bytes = frame.encode();

        assert_eq!(bytes[1] & 0x7F, 126);
        let (decoded, consumed) = WSFrame::decode(&bytes).unwrap();
        assert_eq!(consumed, bytes.len());
        assert_eq!(decoded.payload, payload);
    }

    #[test]
    fn round_trips_large_payload() {
        let payload = vec![0x5Au8; 70_000];
        let frame = WSFrame::new(&WSMessage::binary(payload.clone()), true);
        let bytes = frame.encode();

        assert_eq!(bytes[1] & 0x7F, 127);
        let (decoded, _) = WSFrame::decode(&bytes).unwrap();
        assert_eq!(decoded.payload, payload);
    }

    #[test]
    fn decode_reports_incomplete_frames() {
        assert_eq!(WSFrame::decode(&[]).unwrap_err(), FrameError::Incomplete);
        assert_eq!(WSFrame::decode(&[0x81]).unwrap_err(), FrameError::Incomplete);

        let frame = WSFrame::new(&WSMessage::text("hello"), true);
        let bytes = frame.encode();
        assert_eq!(
            WSFrame::decode(&bytes[..bytes.len() - 1]).unwrap_err(),
            FrameError::Incomplete
        );
    }

    #[test]
    fn decode_rejects_unknown_opcode() {
        assert_eq!(
            WSFrame::decode(&[0x83, 0x00]).unwrap_err(),
            FrameError::UnknownOpcode(0x3)
        );
    }

    #[test]
    fn decode_rejects_fragmented_control_frame() {
        // FIN clear on a ping frame.
        assert_eq!(
            WSFrame::decode(&[0x09, 0x00]).unwrap_err(),
            FrameError::InvalidControlFrame
        );
    }

    #[test]
    fn decode_rejects_non_minimal_lengths() {
        // 16-bit length form carrying a value that fits in 7 bits.
        assert_eq!(
            WSFrame::decode(&[0x82, 126, 0x00, 0x05, 1, 2, 3, 4, 5]).unwrap_err(),
            FrameError::NonMinimalLength
        );
    }

    #[test]
    fn mask_is_reversible() {
        let payload = b"tigerswap";
        let key = [0x12, 0x34, 0x56, 0x78];
        let masked = apply_mask(payload, &key);
        assert_ne!(masked, payload);
        assert_eq!(apply_mask(&masked, &key), payload.to_vec());
    }

    #[test]
    fn validate_requires_masking_from_clients() {
        let unmasked = WSFrame::new(&WSMessage::text("x"), false);
        assert_eq!(
            unmasked.validate_client_frame().unwrap_err(),
            FrameError::InvalidMasking
        );

        let masked = WSFrame::new(&WSMessage::text("x"), true);
        assert!(masked.validate_client_frame().is_ok());
    }

    #[test]
    fn close_frame_carries_status_code() {
        let frame = WSFrame::new(&WSMessage::close(1000), true);
        let (decoded, _) = WSFrame::decode(&frame.encode()).unwrap();
        assert_eq!(decoded.message().unwrap().data, 1000u16.to_be_bytes().to_vec());
    }
}
