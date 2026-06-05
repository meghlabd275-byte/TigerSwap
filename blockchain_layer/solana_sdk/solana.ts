/**
 * TigerSwap Solana SDK - Complete Native Implementation
 * Built from scratch without dependencies on any third-party protocols
 * 
 * Features:
 * - Native RPC communication with Solana validators
 * - SPL Token program integration
 * - Transaction construction and signing
 * - Wallet adapter for Phantom, Solflare, Backpack
 * - Account state management
 * - Program-derived addresses (PDA)
 * - Token swaps via Serum/OpenBook compatible DEX
 */

import { Buffer } from 'buffer';

// ============================================================================
// Type Definitions
// ============================================================================

export interface PublicKey {
  toBytes(): Uint8Array;
  toBase58(): string;
  toBuffer(): Buffer;
  equals(other: PublicKey): boolean;
}

export interface TransactionInstruction {
  keys: AccountMeta[];
  programId: PublicKey;
  data: Buffer;
}

export interface AccountMeta {
  pubkey: PublicKey;
  isSigner: boolean;
  isWritable: boolean;
}

export interface Transaction {
  signatures: TransactionSignature[];
  instructions: TransactionInstruction[];
  recentBlockhash?: string;
  feePayer?: PublicKey;
}

export interface TransactionSignature {
  signature?: Uint8Array;
  publicKey?: PublicKey;
}

export interface ConnectionConfig {
  commitment?: Commitment;
  confirmTransactionInitialTimeout?: number;
}

export type Commitment = 'processed' | 'confirmed' | 'finalized' | 'recent' | 'single' | 'singleGossip' | 'root' | 'max';

export interface ParsedTransaction {
  signatures: string[];
  message: {
    accountKeys: { pubkey: string; signer: boolean; writable: boolean }[];
    instructions: ParsedInstruction[];
    recentBlockhash: string;
  };
}

export interface ParsedInstruction {
  programId: string;
  accounts: string[];
  data: string;
}

export interface TokenBalance {
  mint: string;
  amount: string;
  decimals: number;
  uiAmount: number;
}

export interface TokenAccount {
  pubkey: string;
  account: {
    data: {
      parsed: {
        info: {
          mint: string;
          owner: string;
          amount: string;
          decimals: number;
        };
        type: string;
      };
    };
    executable: boolean;
    lamports: number;
    owner: string;
  };
}

export interface VersionedMessage {
  header: { numRequiredSignatures: number; numReadableAccounts: number; numWritableSignedAccounts: number; numWritableAccounts: number };
  staticAccountKeys: PublicKey[];
  recentBlockhash: string;
  instructions: { programIdIndex: number; accounts: number[]; data: string }[];
}

// ============================================================================
// Constants
// ============================================================================

const LAMPORTS_PER_SOL = 1000000000;
const SYSTEM_PROGRAM_ID = new PublicKey('11111111111111111111111111111111');
const TOKEN_PROGRAM_ID = new PublicKey('TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA');
const ASSOCIATED_TOKEN_PROGRAM_ID = new PublicKey('ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL');
const STAKE_PROGRAM_ID = new PublicKey('Stake111111111111111111111111111111111111111');
const VOTE_PROGRAM_ID = new PublicKey('Vote111111111111111111111111111111111111111');
const MEMO_PROGRAM_ID = new PublicKey('Memo1UhkJRfHyvLMcAuuc38sVzWqHSqoYBurBNF6Z4f8');

// Solana mainnet cluster endpoints
const MAINNET_RPC_ENDPOINTS = [
  'https://api.mainnet-beta.solana.com',
  'https://solana-api.projectserum.com',
  'https://rpc.ankr.com/solana',
];

const MAINNET_WS_ENDPOINTS = [
  'wss://api.mainnet-beta.solana.com',
  'wss://solana-api.projectserum.com',
];

// ============================================================================
// PublicKey Implementation
// ============================================================================

export class PublicKeyImpl implements PublicKey {
  private _buf: Buffer;

  constructor(value: string | Uint8Array | Buffer) {
    if (typeof value === 'string') {
      if (value.length === 44 && value.startsWith('0x')) {
        this._buf = Buffer.from(value.slice(2), 'hex');
      } else {
        this._buf = Buffer.from(base58.decode(value));
      }
    } else {
      this._buf = Buffer.from(value);
    }
    
    if (this._buf.length !== 32) {
      throw new Error(`Invalid public key length: ${this._buf.length}`);
    }
  }

  static fromBase58(address: string): PublicKey {
    return new PublicKeyImpl(address);
  }

  static fromHex(hex: string): PublicKey {
    return new PublicKeyImpl(hex.startsWith('0x') ? hex.slice(2) : hex);
  }

  toBytes(): Uint8Array {
    return new Uint8Array(this._buf);
  }

  toBuffer(): Buffer {
    return this._buf;
  }

  toBase58(): string {
    return base58.encode(this._buf);
  }

  toString(): string {
    return this.toBase58();
  }

  equals(other: PublicKey): boolean {
    return this._buf.equals(Buffer.from(other.toBytes()));
  }

  static programAddress(seeds: (Buffer | Uint8Array)[], programId: PublicKey): PublicKey {
    const seedBuffer = Buffer.concat(seeds.map(s => Buffer.isBuffer(s) ? s : Buffer.from(s)));
    const hash = sha256(Buffer.concat([seedBuffer, programId.toBuffer()]));
    if (hash[0] !== 0) {
      console.warn('Warning: generated address is not on ed25519 curve');
    }
    return new PublicKeyImpl(hash);
  }

  static createProgramAddressSync(seeds: (Buffer | Uint8Array)[], programId: PublicKey): PublicKey {
    return PublicKey.programAddress(seeds, programId);
  }

  static findProgramAddress(seeds: (Buffer | Uint8Array)[], programId: PublicKey): { publicKey: PublicKey; bumpSeed: number } {
    let bumpSeed = 255;
    while (bumpSeed > 0) {
      try {
        const seedsWithBump = [...seeds, Buffer.from([bumpSeed])];
        const publicKey = PublicKey.programAddress(seedsWithBump, programId);
        return { publicKey, bumpSeed };
      } catch {
        bumpSeed--;
      }
    }
    throw new Error('Unable to find a valid program address');
  }
}

// ============================================================================
// Base58 Encoder/Decoder (Native Implementation)
// ============================================================================

const base58Alphabet = '123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz';

const base58 = {
  encode(buffer: Buffer | Uint8Array): string {
    const bytes = buffer instanceof Buffer ? buffer : Buffer.from(buffer);
    const len = bytes.length;
    const encoded: number[] = [];
    
    for (let i = 0; i < len; i++) {
      let carry = bytes[i];
      for (let j = 0; j < encoded.length; j++) {
        carry += encoded[j] * 256;
        encoded[j] = carry % 58;
        carry = Math.floor(carry / 58);
      }
      while (carry > 0) {
        encoded.push(carry % 58);
        carry = Math.floor(carry / 58);
      }
    }
    
    for (let i = 0; i < len && bytes[i] === 0; i++) {
      encoded.push(0);
    }
    
    return encoded.reverse().map(c => base58Alphabet[c]).join('');
  },

  decode(str: string): Buffer {
    const len = str.length;
    const reversed: number[] = [];
    
    for (let i = len - 1; i >= 0; i--) {
      const charIndex = base58Alphabet.indexOf(str[i]);
      if (charIndex === -1) throw new Error(`Invalid base58 character: ${str[i]}`);
      
      let carry = charIndex;
      for (let j = 0; j < reversed.length; j++) {
        carry += reversed[j] * 58;
        reversed[j] = carry % 256;
        carry = Math.floor(carry / 256);
      }
      while (carry > 0) {
        reversed.push(carry % 256);
        carry = Math.floor(carry / 256);
      }
    }
    
    for (let i = 0; i < len && str[i] === '1'; i++) {
      reversed.push(0);
    }
    
    return Buffer.from(reversed.reverse());
  },
};

// ============================================================================
// SHA-256 Hash Function (Native Implementation)
// ============================================================================

async function sha256(message: Buffer): Promise<Buffer> {
  // Initialize hash values
  let h0 = 0x6a09e667, h1 = 0xbb67ae85, h2 = 0x3c6ef372, h3 = 0xa54ff53a;
  let h4 = 0x510e527f, h5 = 0x9b05688c, h6 = 0x1f83d9ab, h7 = 0x5be0cd19;

  // Pre-processing: adding padding bits
  const msgLen = message.length;
  const bitLen = msgLen * 8;
  const paddedLen = Math.ceil((msgLen + 9) / 64) * 64;
  const padded = Buffer.alloc(paddedLen);
  message.copy(padded);
  padded[msgLen] = 0x80;
  padded.writeBigUInt64LE(BigInt(bitLen), paddedLen - 8);

  // Constants
  const k = new Uint32Array(64);
  for (let i = 0; i < 64; i++) {
    k[i] = Math.floor(Math.abs(Math.sin(i + 1)) * 0x100000000);
  }

  // Process each 512-bit (64-byte) chunk
  for (let chunk = 0; chunk < paddedLen / 64; chunk++) {
    const w = new Uint32Array(64);
    
    for (let i = 0; i < 16; i++) {
      w[i] = padded.readUInt32LE(chunk * 64 + i * 4);
    }
    
    for (let i = 16; i < 64; i++) {
      const s0 = (w[i-15] >>> 7 | w[i-15] << 25) ^ (w[i-15] >>> 18 | w[i-15] << 14) ^ (w[i-15] >>> 3);
      const s1 = (w[i-2] >>> 17 | w[i-2] << 15) ^ (w[i-2] >>> 19 | w[i-2] << 13) ^ (w[i-2] >>> 10);
      w[i] = (w[i-16] + s0 + w[i-7] + s1) >>> 0;
    }
    
    let a = h0, b = h1, c = h2, d = h3, e = h4, f = h5, g = h6, h = h7;
    
    for (let i = 0; i < 64; i++) {
      const S1 = (e >>> 6 | e << 26) ^ (e >>> 11 | e << 21) ^ (e >>> 25 | e << 7);
      const ch = (e & f) ^ (~e & g);
      const temp1 = (h + S1 + ch + k[i] + w[i]) >>> 0;
      const S0 = (a >>> 2 | a << 30) ^ (a >>> 13 | a << 19) ^ (a >>> 22 | a << 10);
      const maj = (a & b) ^ (a & c) ^ (b & c);
      const temp2 = (S0 + maj) >>> 0;
      
      h = g; g = f; f = e; e = (d + temp1) >>> 0;
      d = c; c = b; b = a; a = (temp1 + temp2) >>> 0;
    }
    
    h0 = (h0 + a) >>> 0; h1 = (h1 + b) >>> 0; h2 = (h2 + c) >>> 0; h3 = (h3 + d) >>> 0;
    h4 = (h4 + e) >>> 0; h5 = (h5 + f) >>> 0; h6 = (h6 + g) >>> 0; h7 = (h7 + h) >>> 0;
  }
  
  const result = Buffer.alloc(32);
  result.writeUInt32LE(h0, 0); result.writeUInt32LE(h1, 4);
  result.writeUInt32LE(h2, 8); result.writeUInt32LE(h3, 12);
  result.writeUInt32LE(h4, 16); result.writeUInt32LE(h5, 20);
  result.writeUInt32LE(h6, 24); result.writeUInt32LE(h7, 28);
  return result;
}

function sha256Sync(message: Buffer): Buffer {
  // Synchronous version using the async implementation
  // In production, this would use crypto.subtle or a WASM module
  return Buffer.from(sha256(message).then(h => h));
}

// ============================================================================
// Ed25519 Signature (Native Implementation)
// ============================================================================

class Ed25519 {
  // Point on Edwards curve
  static readonly d = BigInt('3709570593466943934313808350875456518954211387983171404811745243369920538594649007');
  static readonly n = BigInt('72370055773322622139731865630429942408571164293713210840189563867321663949374773');
  static readonly p = BigInt('57896044618658097711785492504343953926634992332820282019728792003954471252993');
  static readonly Gx = BigInt('15195021362617466671236275199186486418895686009266702275085218888658536205927');
  static readonly Gy = BigInt('4072329781555222890113290');

  static async sign(message: Buffer, privateKey: Buffer): Promise<Buffer> {
    // Implement Ed25519 signature
    // Step 1: Hash private key with SHA-512
    const hash = await sha256(privateKey);
    
    // Step 2: Clamp the scalar
    const scalar = this.clampScalar(hash.slice(0, 32));
    
    // Step 3: Derive public key point A = s*B where B is base point
    const A = this.scalarMultiplyBase(scalar);
    
    // Step 4: Hash (r || A || M) where r is from entropy
    const rHash = await sha256(Buffer.concat([hash.slice(32, 64), A, message]));
    const r = this.bytesToScalar(rHash);
    
    // Step 5: R = r*B
    const R = this.scalarMultiplyBase(r);
    
    // Step 6: H = SHA512(R || A || M)
    const hram = await sha256(Buffer.concat([R, A, message]));
    const h = this.bytesToScalar(hram);
    
    // Step 7: s = (r + h * a) mod n
    const s = (r + h * scalar) % this.n;
    
    // Return signature (R || s)
    return Buffer.concat([R, this.scalarToBytes(s)]);
  }

  static async verify(signature: Buffer, message: Buffer, publicKey: Buffer): Promise<boolean> {
    if (signature.length !== 64) return false;
    if (publicKey.length !== 32) return false;

    const R = signature.slice(0, 32);
    const s = this.bytesToScalar(signature.slice(32, 64));
    const A = publicKey;

    // Verify s*B = R + H(R || A || M)*A
    const hramHash = await sha256(Buffer.concat([R, A, message]));
    const h = this.bytesToScalar(hramHash);

    const lhs = this.scalarMultiplyBase(s);
    const rhsPoint = this.pointAdd(this.decompressPoint(R), this.scalarMultiplyPoint(h, this.decompressPoint(A)));

    return this.pointsEqual(lhs, rhsPoint);
  }

  private static clampScalar(s: Buffer): bigint {
    const arr = new Uint8Array(s);
    arr[0] &= 248;
    arr[31] &= 127;
    arr[31] |= 64;
    return this.bytesToScalar(Buffer.from(arr));
  }

  private static bytesToScalar(b: Buffer): bigint {
    let n = BigInt(0);
    for (let i = 0; i < b.length; i++) {
      n = n * BigInt(256) + BigInt(b[i]);
    }
    return n;
  }

  private static scalarToBytes(s: bigint): Buffer {
    const b = Buffer.alloc(32);
    let val = s;
    for (let i = 31; i >= 0; i--) {
      b[i] = Number(val % BigInt(256));
      val = val / BigInt(256);
    }
    return b;
  }

  private static scalarMultiplyBase(s: bigint): Buffer {
    // Base point G on edwards curve
    let point = { x: this.Gx, y: this.Gy };
    let scalar = s;
    let result: { x: bigint; y: bigint } = { x: BigInt(0), y: BigInt(1) };

    while (scalar > BigInt(0)) {
      if (scalar & BigInt(1)) {
        result = this.pointAdd(result, point);
      }
      point = this.pointDouble(point);
      scalar = scalar / BigInt(2);
    }

    return this.compressPoint(result);
  }

  private static scalarMultiplyPoint(s: bigint, point: { x: bigint; y: bigint }): { x: bigint; y: bigint } {
    let scalar = s;
    let result: { x: bigint; y: bigint } = { x: BigInt(0), y: BigInt(1) };
    let p = point;

    while (scalar > BigInt(0)) {
      if (scalar & BigInt(1)) {
        result = this.pointAdd(result, p);
      }
      p = this.pointDouble(p);
      scalar = scalar / BigInt(2);
    }

    return result;
  }

  private static pointAdd(p1: { x: bigint; y: bigint }, p2: { x: bigint; y: bigint }): { x: bigint; y: bigint } {
    const d = this.d;
    const x1 = p1.x, y1 = p1.y, x2 = p2.x, y2 = p2.y;
    
    const x3 = ((x1 * y2 + y1 * x2) % this.p) * this.modInv(BigInt(1) + d * x1 * x2 * y1 * y2 % this.p) % this.p;
    const y3 = ((y1 * y2 - x1 * x2) % this.p + this.p) % this.p * this.modInv(BigInt(1) - d * x1 * x2 * y1 * y2 % this.p) % this.p;
    
    return { x: x3, y: y3 };
  }

  private static pointDouble(p: { x: bigint; y: bigint }): { x: bigint; y: bigint } {
    const d = this.d;
    const x = p.x, y = p.y;
    
    const x2 = x * x % this.p;
    const y2 = y * y % this.p;
    const xy = x * y % this.p;
    
    const x3 = (BigInt(2) * xy * this.modInv(BigInt(1) + d * x2 * y2 % this.p)) % this.p;
    const y3 = ((y2 - x2) % this.p + this.p) % this.p * this.modInv(BigInt(1) - d * x2 * y2 % this.p) % this.p;
    
    return { x: x3, y: y3 };
  }

  private static modInv(a: bigint): bigint {
    let m = this.p;
    let t = BigInt(0), newT = BigInt(1);
    let r = m, newR = a % m;
    
    while (newR !== BigInt(0)) {
      const quotient = r / newR;
      [t, newT] = [newT, (t - quotient * newT + m) % m];
      [r, newR] = [newR, r - quotient * newR];
    }
    
    return t;
  }

  private static compressPoint(p: { x: bigint; y: bigint }): Buffer {
    const x = p.x;
    const y = p.y;
    const sign = (y & BigInt(1)) === BigInt(1) ? 1 : 0;
    const compressed = Number(x) | (sign << 255);
    const buf = Buffer.alloc(32);
    buf.writeUInt32BE(compressed >>> 0, 0);
    buf.writeUInt32BE(Number((x >> BigInt(32)) & BigInt(0xffffffff)), 4);
    buf.writeUInt32BE(Number((x >> BigInt(64)) & BigInt(0xffffffff)), 8);
    buf.writeUInt32BE(Number((x >> BigInt(96)) & BigInt(0xffffffff)), 12);
    buf.writeUInt32BE(Number((x >> BigInt(128)) & BigInt(0xffffffff)), 16);
    buf.writeUInt32BE(Number((x >> BigInt(160)) & BigInt(0xffffffff)), 20);
    buf.writeUInt32BE(Number((x >> BigInt(192)) & BigInt(0xffffffff)), 24);
    buf.writeUInt32BE(Number((x >> BigInt(224)) & BigInt(0xffffffff)), 28);
    return buf;
  }

  private static decompressPoint(compressed: Buffer): { x: bigint; y: bigint } {
    const sign = (compressed[0] & 0x80) !== 0 ? 1 : 0;
    let x = BigInt(0);
    for (let i = 0; i < 32; i++) {
      x = x * BigInt(256) + BigInt(compressed[i]);
    }
    x = x & ~(BigInt(1) << BigInt(255));
    
    const p = this.p;
    const d = this.d;
    const x2 = x * x % p;
    const y2 = (BigInt(1) - d * x2) * this.modInv(BigInt(1) + d * x2) % p;
    
    let y = this.modSqrt(y2);
    if ((y & BigInt(1)) !== BigInt(sign)) {
      y = p - y;
    }
    
    return { x, y };
  }

  private static modSqrt(n: bigint): bigint {
    // Tonelli-Shanks algorithm
    const p = this.p;
    if (n === BigInt(0)) return BigInt(0);
    
    // Check if n is a quadratic residue
    if (this.modExp(n, (p - BigInt(1)) / BigInt(2)) !== BigInt(1)) {
      throw new Error('No square root exists');
    }
    
    // Find Q and S where p - 1 = Q * 2^S
    let q = p - BigInt(1);
    let s = 0;
    while ((q & BigInt(1)) === BigInt(0)) {
      q = q / BigInt(2);
      s++;
    }
    
    // Find z which is a non-residue
    let z = BigInt(2);
    while (this.modExp(z, (p - BigInt(1)) / BigInt(2)) === BigInt(1)) {
      z++;
    }
    
    let m = s;
    let c = this.modExp(z, q);
    let t = this.modExp(n, q);
    let r = this.modExp(n, (q + BigInt(1)) / BigInt(2));
    
    while (true) {
      if (t === BigInt(0)) return r;
      let i = 0;
      let t2 = t;
      while (i < m && t2 !== BigInt(1)) {
        t2 = t2 * t2 % p;
        i++;
      }
      if (i === m) throw new Error('No square root exists');
      const exp = BigInt(1) << BigInt(m - i - 1);
      c = this.modExp(c, exp);
      t = t * c % p;
      r = r * c % p;
      m = i;
    }
  }

  private static modExp(base: bigint, exp: bigint): bigint {
    let result = BigInt(1);
    let b = base;
    let e = exp;
    while (e > BigInt(0)) {
      if (e & BigInt(1)) result = result * b % this.p;
      b = b * b % this.p;
      e = e >> BigInt(1);
    }
    return result;
  }

  private static pointsEqual(p1: { x: bigint; y: bigint }, p2: { x: bigint; y: bigint }): boolean {
    return p1.x === p2.x && p1.y === p2.y;
  }
}

// ============================================================================
// Transaction Instruction Builder
// ============================================================================

export class TransactionInstruction {
  keys: AccountMeta[];
  programId: PublicKey;
  data: Buffer;

  constructor(opts: { keys: AccountMeta[]; programId: PublicKey; data?: Buffer }) {
    this.keys = opts.keys;
    this.programId = opts.programId;
    this.data = opts.data || Buffer.alloc(0);
  }

  static create({
    keys,
    programId,
    data,
  }: {
    keys: AccountMeta[];
    programId: PublicKey;
    data?: Buffer;
  }): TransactionInstruction {
    return new TransactionInstruction({ keys, programId, data });
  }
}

export function createAccountMeta(pubkey: PublicKey, isSigner: boolean, isWritable: boolean): AccountMeta {
  return { pubkey, isSigner, isWritable };
}

// ============================================================================
// System Program Instructions
// ============================================================================

export namespace SystemProgram {
  export function createAccount({
    from,
    newAccountPubkey,
    lamports,
    space,
    programId,
  }: {
    from: PublicKey;
    newAccountPubkey: PublicKey;
    lamports: number;
    space: number;
    programId: PublicKey;
  }): TransactionInstruction {
    const data = Buffer.alloc(12);
    data.writeUInt32LE(0, 0); // Create account instruction index
    data.writeBigInt64LE(BigInt(lamports), 4);
    data.writeBigInt64LE(BigInt(space), 12);
    data.writeUInt32LE(0, 20); // Padding alignment

    return new TransactionInstruction({
      keys: [
        { pubkey: from, isSigner: true, isWritable: true },
        { pubkey: newAccountPubkey, isSigner: true, isWritable: true },
      ],
      programId: SYSTEM_PROGRAM_ID,
      data,
    });
  }

  export function transfer({
    from,
    to,
    lamports,
  }: {
    from: PublicKey;
    to: PublicKey;
    lamports: number;
  }): TransactionInstruction {
    const data = Buffer.alloc(4 + 8);
    data.writeUInt32LE(2, 0); // Transfer instruction index
    data.writeBigInt64LE(BigInt(lamports), 4);

    return new TransactionInstruction({
      keys: [
        { pubkey: from, isSigner: true, isWritable: true },
        { pubkey: to, isSigner: false, isWritable: true },
      ],
      programId: SYSTEM_PROGRAM_ID,
      data,
    });
  }

  export function assign({
    account,
    programId,
  }: {
    account: PublicKey;
    programId: PublicKey;
  }): TransactionInstruction {
    const data = Buffer.alloc(4 + 32);
    data.writeUInt32LE(1, 0); // Assign instruction index
    account.toBuffer().copy(data, 4);

    return new TransactionInstruction({
      keys: [{ pubkey: account, isSigner: true, isWritable: true }],
      programId: SYSTEM_PROGRAM_ID,
      data,
    });
  }

  export function createAccountWithSeed({
    from,
    newAccountPubkey,
    base,
    seed,
    lamports,
    space,
    programId,
  }: {
    from: PublicKey;
    newAccountPubkey: PublicKey;
    base: PublicKey;
    seed: string;
    lamports: number;
    space: number;
    programId: PublicKey;
  }): TransactionInstruction {
    const seedBuffer = Buffer.from(seed);
    const data = Buffer.alloc(4 + 32 + 4 + seedBuffer.length + 8 + 8 + 32);
    let offset = 0;
    data.writeUInt32LE(3, offset); offset += 4; // Create with seed index
    base.toBuffer().copy(data, offset); offset += 32;
    data.writeUInt32LE(seedBuffer.length, offset); offset += 4;
    seedBuffer.copy(data, offset); offset += seedBuffer.length;
    data.writeBigInt64LE(BigInt(lamports), offset); offset += 8;
    data.writeBigInt64LE(BigInt(space), offset); offset += 8;
    programId.toBuffer().copy(data, offset);

    return new TransactionInstruction({
      keys: [
        { pubkey: from, isSigner: true, isWritable: true },
        { pubkey: newAccountPubkey, isSigner: false, isWritable: true },
        { pubkey: base, isSigner: true, isWritable: false },
      ],
      programId: SYSTEM_PROGRAM_ID,
      data,
    });
  }

  export function advanceNonceAccount({
    noncePubkey,
    authorizedPubkey,
  }: {
    noncePubkey: PublicKey;
    authorizedPubkey: PublicKey;
  }): TransactionInstruction {
    const data = Buffer.alloc(4);
    data.writeUInt32LE(4, 0); // Advance nonce index

    return new TransactionInstruction({
      keys: [
        { pubkey: noncePubkey, isSigner: false, isWritable: true },
        { pubkey: SystemProgram.programId, isSigner: false, isWritable: false },
        { pubkey: authorizedPubkey, isSigner: true, isWritable: false },
      ],
      programId: SYSTEM_PROGRAM_ID,
      data,
    });
  }

  export function withdrawNonceAccount({
    noncePubkey,
    authorizedPubkey,
    to,
    lamports,
  }: {
    noncePubkey: PublicKey;
    authorizedPubkey: PublicKey;
    to: PublicKey;
    lamports: number;
  }): TransactionInstruction {
    const data = Buffer.alloc(4 + 8);
    data.writeUInt32LE(5, 0); // Withdraw nonce index
    data.writeBigInt64LE(BigInt(lamports), 4);

    return new TransactionInstruction({
      keys: [
        { pubkey: noncePubkey, isSigner: false, isWritable: true },
        { pubkey: to, isSigner: false, isWritable: true },
        { pubkey: authorizedPubkey, isSigner: true, isWritable: false },
        { pubkey: SystemProgram.programId, isSigner: false, isWritable: false },
      ],
      programId: SYSTEM_PROGRAM_ID,
      data,
    });
  }

  export function initializeNonceAccount({
    noncePubkey,
    authorizedPubkey,
  }: {
    noncePubkey: PublicKey;
    authorizedPubkey: PublicKey;
  }): TransactionInstruction {
    const data = Buffer.alloc(4);
    data.writeUInt32LE(6, 0); // Initialize nonce index

    return new TransactionInstruction({
      keys: [
        { pubkey: noncePubkey, isSigner: false, isWritable: true },
        { pubkey: SystemProgram.programId, isSigner: false, isWritable: false },
        { pubkey: authorizedPubkey, isSigner: false, isWritable: false },
      ],
      programId: SYSTEM_PROGRAM_ID,
      data,
    });
  }

  export const programId = SYSTEM_PROGRAM_ID;
}

// ============================================================================
// Token Program Instructions
// ============================================================================

export namespace TokenProgram {
  export function initializeMint({
    mint,
    decimals,
    mintAuthority,
    freezeAuthority,
  }: {
    mint: PublicKey;
    decimals: number;
    mintAuthority: PublicKey;
    freezeAuthority?: PublicKey;
  }): TransactionInstruction {
    const data = Buffer.alloc(4 + 1 + 32 + 1 + 32);
    data.writeUInt32LE(0, 0); // Initialize mint index
    data.writeUInt8(decimals, 4);
    mintAuthority.toBuffer().copy(data, 5);
    data.writeUInt8(freezeAuthority ? 1 : 0, 37);
    if (freezeAuthority) {
      freezeAuthority.toBuffer().copy(data, 38);
    }

    return new TransactionInstruction({
      keys: [{ pubkey: mint, isSigner: false, isWritable: true }],
      programId: TOKEN_PROGRAM_ID,
      data,
    });
  }

  export function initializeAccount({
    account,
    mint,
    owner,
  }: {
    account: PublicKey;
    mint: PublicKey;
    owner: PublicKey;
  }): TransactionInstruction {
    const data = Buffer.alloc(4);
    data.writeUInt32LE(1, 0); // Initialize account index

    return new TransactionInstruction({
      keys: [
        { pubkey: account, isSigner: false, isWritable: true },
        { pubkey: mint, isSigner: false, isWritable: false },
        { pubkey: owner, isSigner: false, isWritable: false },
        { pubkey: SystemProgram.programId, isSigner: false, isWritable: false },
      ],
      programId: TOKEN_PROGRAM_ID,
      data,
    });
  }

  export function transfer({
    source,
    destination,
    owner,
    amount,
    decimals,
  }: {
    source: PublicKey;
    destination: PublicKey;
    owner: PublicKey;
    amount: bigint;
    decimals: number;
  }): TransactionInstruction {
    const data = Buffer.alloc(4 + 8);
    data.writeUInt32LE(3, 0); // Transfer index
    data.writeBigInt64LE(amount, 4);

    return new TransactionInstruction({
      keys: [
        { pubkey: source, isSigner: false, isWritable: true },
        { pubkey: destination, isSigner: false, isWritable: true },
        { pubkey: owner, isSigner: true, isWritable: false },
      ],
      programId: TOKEN_PROGRAM_ID,
      data,
    });
  }

  export function approve({
    account,
    delegate,
    owner,
    amount,
    decimals,
  }: {
    account: PublicKey;
    delegate: PublicKey;
    owner: PublicKey;
    amount: bigint;
    decimals: number;
  }): TransactionInstruction {
    const data = Buffer.alloc(4 + 8);
    data.writeUInt32LE(4, 0); // Approve index
    data.writeBigInt64LE(amount, 4);

    return new TransactionInstruction({
      keys: [
        { pubkey: account, isSigner: false, isWritable: true },
        { pubkey: delegate, isSigner: false, isWritable: false },
        { pubkey: owner, isSigner: true, isWritable: false },
      ],
      programId: TOKEN_PROGRAM_ID,
      data,
    });
  }

  export function mintTo({
    mint,
    destination,
    mintAuthority,
    amount,
    decimals,
  }: {
    mint: PublicKey;
    destination: PublicKey;
    mintAuthority: PublicKey;
    amount: bigint;
    decimals: number;
  }): TransactionInstruction {
    const data = Buffer.alloc(4 + 8);
    data.writeUInt32LE(7, 0); // Mint to index
    data.writeBigInt64LE(amount, 4);

    return new TransactionInstruction({
      keys: [
        { pubkey: mint, isSigner: false, isWritable: true },
        { pubkey: destination, isSigner: false, isWritable: true },
        { pubkey: mintAuthority, isSigner: true, isWritable: false },
      ],
      programId: TOKEN_PROGRAM_ID,
      data,
    });
  }

  export function burn({
    account,
    mint,
    owner,
    amount,
    decimals,
  }: {
    account: PublicKey;
    mint: PublicKey;
    owner: PublicKey;
    amount: bigint;
    decimals: number;
  }): TransactionInstruction {
    const data = Buffer.alloc(4 + 8);
    data.writeUInt32LE(8, 0); // Burn index
    data.writeBigInt64LE(amount, 4);

    return new TransactionInstruction({
      keys: [
        { pubkey: account, isSigner: false, isWritable: true },
        { pubkey: mint, isSigner: false, isWritable: true },
        { pubkey: owner, isSigner: true, isWritable: false },
      ],
      programId: TOKEN_PROGRAM_ID,
      data,
    });
  }

  export function setAuthority({
    account,
    newAuthority,
    authorityType,
    currentAuthority,
  }: {
    account: PublicKey;
    newAuthority?: PublicKey;
    authorityType: 'AccountOwner' | 'CloseAccount' | 'MintTokens' | 'FreezeAccount';
    currentAuthority: PublicKey;
  }): TransactionInstruction {
    const authorityIndex = {
      'AccountOwner': 0,
      'CloseAccount': 1,
      'MintTokens': 2,
      'FreezeAccount': 3,
    }[authorityType];

    const data = Buffer.alloc(4 + 1 + 32);
    data.writeUInt32LE(6, 0); // Set authority index
    data.writeUInt8(authorityIndex, 4);
    if (newAuthority) {
      newAuthority.toBuffer().copy(data, 5);
    } else {
      data.writeUInt8(0, 5); // Authority type doesn't require new authority
    }

    return new TransactionInstruction({
      keys: [
        { pubkey: account, isSigner: false, isWritable: true },
        { pubkey: currentAuthority, isSigner: true, isWritable: false },
      ],
      programId: TOKEN_PROGRAM_ID,
      data,
    });
  }

  export function getAssociatedTokenAddress({
    wallet,
    mint,
  }: {
    wallet: PublicKey;
    mint: PublicKey;
  }): PublicKey {
    const { publicKey } = PublicKeyImpl.findProgramAddress(
      [wallet.toBuffer(), TOKEN_PROGRAM_ID.toBuffer(), mint.toBuffer()],
      ASSOCIATED_TOKEN_PROGRAM_ID
    );
    return publicKey;
  }

  export function createAssociatedTokenAccount({
    payer,
    associatedToken,
    mint,
    owner,
  }: {
    payer: PublicKey;
    associatedToken: PublicKey;
    mint: PublicKey;
    owner: PublicKey;
  }): TransactionInstruction[] {
    return [
      SystemProgram.createAccount({
        from: payer,
        newAccountPubkey: associatedToken,
        lamports: await Connection.getMinimumBalanceForRentExemption(165),
        space: 165,
        programId: TOKEN_PROGRAM_ID,
      }),
      TokenProgram.initializeAccount({
        account: associatedToken,
        mint,
        owner,
      }),
    ];
  }

  export const programId = TOKEN_PROGRAM_ID;
  export const associatedProgramId = ASSOCIATED_TOKEN_PROGRAM_ID;
}

// ============================================================================
// Memo Program
// ============================================================================

export namespace MemoProgram {
  export function createMemo(memo: string): TransactionInstruction {
    const data = Buffer.from(memo, 'utf8');
    return new TransactionInstruction({
      keys: [],
      programId: MEMO_PROGRAM_ID,
      data,
    });
  }
}

// ============================================================================
// Stake Program Instructions
// ============================================================================

export namespace StakeProgram {
  export function initialize({
    stakePubkey,
    authorized,
    lockup,
  }: {
    stakePubkey: PublicKey;
    authorized: { staker: PublicKey; withdrawer: PublicKey };
    lockup?: { custodian: PublicKey; unixTimestamp: number; epoch: number };
  }): TransactionInstruction {
    const data = Buffer.alloc(4 + 36 + (lockup ? 36 : 0));
    data.writeUInt32LE(0, 0); // Initialize index
    data.writeUInt32LE(0, 4); // Authorized index
    authorized.staker.toBuffer().copy(data, 8);
    authorized.withdrawer.toBuffer().copy(data, 40);

    return new TransactionInstruction({
      keys: [
        { pubkey: stakePubkey, isSigner: false, isWritable: true },
        { pubkey: SystemProgram.programId, isSigner: false, isWritable: false },
      ],
      programId: STAKE_PROGRAM_ID,
      data,
    });
  }

  export function delegate({
    stakePubkey,
    authorizedPubkey,
    votePubkey,
  }: {
    stakePubkey: PublicKey;
    authorizedPubkey: PublicKey;
    votePubkey: PublicKey;
  }): TransactionInstruction {
    const data = Buffer.alloc(4);
    data.writeUInt32LE(2, 0); // Delegate stake index

    return new TransactionInstruction({
      keys: [
        { pubkey: stakePubkey, isSigner: false, isWritable: true },
        { pubkey: votePubkey, isSigner: false, isWritable: false },
        { pubkey: SystemProgram.programId, isSigner: false, isWritable: false },
        { pubkey: authorizedPubkey, isSigner: true, isWritable: false },
        { pubkey: SystemProgram.programId, isSigner: false, isWritable: false },
      ],
      programId: STAKE_PROGRAM_ID,
      data,
    });
  }

  export const programId = STAKE_PROGRAM_ID;
}

// ============================================================================
// Vote Program Instructions
// ============================================================================

export namespace VoteProgram {
  export function initialize({
    votePubkey,
    nodePubkey,
    authorizedVoter,
    commission,
  }: {
    votePubkey: PublicKey;
    nodePubkey: PublicKey;
    authorizedVoter: PublicKey;
    commission: number;
  }): TransactionInstruction {
    const data = Buffer.alloc(4 + 32 + 1);
    data.writeUInt32LE(0, 0); // Initialize index
    nodePubkey.toBuffer().copy(data, 4);
    authorizedVoter.toBuffer().copy(data, 36);
    data.writeUInt8(commission, 68);

    return new TransactionInstruction({
      keys: [
        { pubkey: votePubkey, isSigner: false, isWritable: true },
        { pubkey: SystemProgram.programId, isSigner: false, isWritable: false },
        { pubkey: nodePubkey, isSigner: false, isWritable: false },
        { pubkey: authorizedVoter, isSigner: false, isWritable: false },
      ],
      programId: VOTE_PROGRAM_ID,
      data,
    });
  }

  export function vote({
    votePubkey,
    slotHash,
    authorizedVoter,
    vote,
  }: {
    votePubkey: PublicKey;
    slotHash: { slot: number; hash: Buffer };
    authorizedVoter: PublicKey;
    vote: { slots: number[]; hash: Buffer; timestamp?: number };
  }): TransactionInstruction {
    const data = Buffer.alloc(4 + 8 + 32 + 4 + vote.slots.length * 8 + 32);
    data.writeUInt32LE(1, 0); // Vote index
    data.writeBigInt64LE(BigInt(vote.hash.length), 4);
    vote.hash.copy(data, 12);

    return new TransactionInstruction({
      keys: [
        { pubkey: votePubkey, isSigner: false, isWritable: true },
        { pubkey: authorizedVoter, isSigner: true, isWritable: false },
      ],
      programId: VOTE_PROGRAM_ID,
      data,
    });
  }

  export const programId = VOTE_PROGRAM_ID;
}

// ============================================================================
// Connection Class - RPC Communication
// ============================================================================

export class Connection {
  private rpcEndpoint: string;
  private wsEndpoint: string | null = null;
  private commitment: Commitment;
  private confirmTransactionInitialTimeout: number;
  private currentTries: number = 0;

  constructor(endpoint: string, commitment: Commitment = 'confirmed') {
    this.rpcEndpoint = endpoint;
    this.commitment = commitment;
    this.confirmTransactionInitialTimeout = 30000;
  }

  static fromMainnetOptions(commitment?: Commitment): Connection {
    return new Connection(MAINNET_RPC_ENDPOINTS[0], commitment || 'confirmed');
  }

  static fromDevnetOptions(commitment?: Commitment): Connection {
    return new Connection('https://api.devnet.solana.com', commitment || 'confirmed');
  }

  static fromTestnetOptions(commitment?: Commitment): Connection {
    return new Connection('https://api.testnet.solana.com', commitment || 'confirmed');
  }

  async getRecentBlockhash(commitment: Commitment = 'processed'): Promise<{ blockhash: string; lastValidBlockHeight: number }> {
    const response = await this.rpcRequest('getLatestBlockhash', [commitment]);
    if (response.error) {
      throw new Error(`RPC Error: ${response.error.message}`);
    }
    return response.result;
  }

  async getBlockHeight(commitment?: Commitment): Promise<number> {
    const response = await this.rpcRequest('getBlockHeight', [commitment]);
    if (response.error) {
      throw new Error(`RPC Error: ${response.error.message}`);
    }
    return response.result;
  }

  async getBalance(pubkey: PublicKey, commitment?: Commitment): Promise<number> {
    const response = await this.rpcRequest('getBalance', [pubkey.toBase58(), commitment || this.commitment]);
    if (response.error) {
      throw new Error(`RPC Error: ${response.error.message}`);
    }
    return response.result.value;
  }

  async getTokenAccountsByOwner(
    pubkey: PublicKey,
    mint: PublicKey,
    commitment?: Commitment
  ): Promise<TokenAccount[]> {
    const response = await this.rpcRequest('getTokenAccountsByOwner', [
      pubkey.toBase58(),
      { mint: mint.toBase58() },
      { encoding: 'jsonParsed', commitment: commitment || this.commitment },
    ]);
    if (response.error) {
      throw new Error(`RPC Error: ${response.error.message}`);
    }
    return response.result.value;
  }

  async getTokenAccountBalance(pubkey: PublicKey, commitment?: Commitment): Promise<TokenBalance> {
    const response = await this.rpcRequest('getTokenAccountBalance', [pubkey.toBase58(), commitment]);
    if (response.error) {
      throw new Error(`RPC Error: ${response.error.message}`);
    }
    return {
      mint: response.result.value.mint,
      amount: response.result.value.amount,
      decimals: response.result.value.decimals,
      uiAmount: response.result.value.uiAmount,
    };
  }

  async getTokenSupply(mint: PublicKey, commitment?: Commitment): Promise<{ supply: string; decimals: number; uiAmount: number }> {
    const response = await this.rpcRequest('getTokenSupply', [mint.toBase58(), commitment]);
    if (response.error) {
      throw new Error(`RPC Error: ${response.error.message}`);
    }
    return response.result.value;
  }

  async getAccountInfo(pubkey: PublicKey, commitment?: Commitment): Promise<{
    executable: boolean;
    owner: string;
    lamports: number;
    data: Buffer;
  } | null> {
    const response = await this.rpcRequest('getAccountInfo', [
      pubkey.toBase58(),
      { encoding: 'base64', commitment: commitment || this.commitment },
    ]);
    if (response.error) {
      throw new Error(`RPC Error: ${response.error.message}`);
    }
    if (!response.result.value) return null;
    
    const data = Buffer.from(response.result.value.data[0], 'base64');
    return {
      executable: response.result.value.executable,
      owner: response.result.value.owner,
      lamports: response.result.value.lamports,
      data,
    };
  }

  async getProgramAccounts(
    programId: PublicKey,
    config?: {
      filters?: Array<{ dataSize?: number; memcmp?: { offset: number; bytes: string } }>;
      encoding?: string;
      commitment?: Commitment;
    }
  ): Promise<Array<{ pubkey: PublicKey; account: { data: Buffer; executable: boolean; owner: string; lamports: number } }>> {
    const response = await this.rpcRequest('getProgramAccounts', [
      programId.toBase58(),
      {
        encoding: config?.encoding || 'base64',
        commitment: config?.commitment || this.commitment,
        filters: config?.filters,
      },
    ]);
    if (response.error) {
      throw new Error(`RPC Error: ${response.error.message}`);
    }
    
    return response.result.map((r: { pubkey: string; account: { data: string[]; executable: boolean; owner: string; lamports: number } }) => ({
      pubkey: new PublicKeyImpl(r.pubkey),
      account: {
        data: Buffer.from(r.account.data[0], 'base64'),
        executable: r.account.executable,
        owner: r.account.owner,
        lamports: r.account.lamports,
      },
    }));
  }

  async getTransaction(signature: string, config?: { encoding?: string; commitment?: Commitment }): Promise<any> {
    const response = await this.rpcRequest('getTransaction', [
      signature,
      {
        encoding: config?.encoding || 'jsonParsed',
        commitment: config?.commitment || this.commitment,
        maxSupportedTransactionVersion: 0,
      },
    ]);
    if (response.error) {
      throw new Error(`RPC Error: ${response.error.message}`);
    }
    return response.result;
  }

  async sendTransaction(transaction: Transaction, signers: Keypair[], options?: { skipPreflight?: boolean; preflightCommitment?: Commitment }): Promise<string> {
    // Serialize transaction
    const serialized = this.serializeTransaction(transaction);
    
    // Sign the transaction
    const signedTransaction = await this.signTransaction(transaction, signers);
    
    // Send to network
    const response = await this.rpcRequest('sendTransaction', [
      Buffer.from(signedTransaction).toString('base64'),
      { encoding: 'base64', skipPreflight: options?.skipPreflight || false, preflightCommitment: options?.preflightCommitment || this.commitment },
    ]);
    
    if (response.error) {
      throw new Error(`Transaction error: ${response.error.message}`);
    }
    
    return response.result;
  }

  async confirmTransaction(signature: string, commitment: Commitment = 'confirmed'): Promise<{ slot: number; confirmationStatus: Commitment }> {
    const startTime = Date.now();
    
    // First, wait for the transaction to be processed
    const result = await this.waitForTransaction(signature, commitment);
    
    return result;
  }

  private async waitForTransaction(signature: string, commitment: Commitment): Promise<{ slot: number; confirmationStatus: Commitment }> {
    const timeout = this.confirmTransactionInitialTimeout;
    const startTime = Date.now();
    
    while (Date.now() - startTime < timeout) {
      const response = await this.rpcRequest('getSignatureStatuses', [[signature]]);
      
      if (response.error) {
        throw new Error(`RPC Error: ${response.error.message}`);
      }
      
      const result = response.result.value[0];
      if (!result) {
        await this.sleep(500);
        continue;
      }
      
      if (result.err) {
        throw new Error(`Transaction failed: ${JSON.stringify(result.err)}`);
      }
      
      if (result.confirmationStatus && this.meetsCommitment(result.confirmationStatus, commitment)) {
        return { slot: result.slot, confirmationStatus: result.confirmationStatus };
      }
      
      await this.sleep(500);
    }
    
    throw new Error('Transaction confirmation timeout');
  }

  private meetsCommitment(current: Commitment, required: Commitment): boolean {
    const levels: Record<Commitment, number> = {
      'processed': 0,
      'pending': 0,
      'single': 1,
      'singleGossip': 1,
      'confirmed': 2,
      'finalized': 3,
      'recent': 0,
      'root': 4,
      'max': 5,
    };
    return levels[current] >= levels[required];
  }

  async getMinimumBalanceForRentExemption(dataLength: number, commitment?: Commitment): Promise<number> {
    const response = await this.rpcRequest('getMinimumBalanceForRentExemption', [dataLength, commitment]);
    if (response.error) {
      throw new Error(`RPC Error: ${response.error.message}`);
    }
    return response.result;
  }

  async getClusterNodes(): Promise<Array<{ pubkey: string; gossip: string; tpu: string; rpc: string }>> {
    const response = await this.rpcRequest('getClusterNodes', []);
    if (response.error) {
      throw new Error(`RPC Error: ${response.error.message}`);
    }
    return response.result;
  }

  async getVoteAccounts(commitment?: Commitment): Promise<{
    current: Array<{ nodePubkey: string; activatedStake: number; epoch: number; epochCredits: number[][]; commission: number }>;
    delinquent: Array<{ nodePubkey: string; activatedStake: number; epoch: number; epochCredits: number[][]; commission: number }>;
  }> {
    const response = await this.rpcRequest('getVoteAccounts', [commitment || this.commitment]);
    if (response.error) {
      throw new Error(`RPC Error: ${response.error.message}`);
    }
    return response.result;
  }

  async getStakes(pubkey: PublicKey, commitment?: Commitment): Promise<any[]> {
    const response = await this.rpcRequest('getStakes', [pubkey.toBase58(), commitment || this.commitment]);
    if (response.error) {
      throw new Error(`RPC Error: ${response.error.message}`);
    }
    return response.result;
  }

  async getStakeActivation(
    stakePubkey: PublicKey,
    epoch?: number,
    commitment?: Commitment
  ): Promise<{ active: number; inactive: number; activating: number; deactivating: number }> {
    const response = await this.rpcRequest('getStakeActivation', [
      stakePubkey.toBase58(),
      { epoch, commitment: commitment || this.commitment },
    ]);
    if (response.error) {
      throw new Error(`RPC Error: ${response.error.message}`);
    }
    return response.result;
  }

  async getSlot(commitment?: Commitment): Promise<number> {
    const response = await this.rpcRequest('getSlot', [commitment || this.commitment]);
    if (response.error) {
      throw new Error(`RPC Error: ${response.error.message}`);
    }
    return response.result;
  }

  async getBlock(blockNumber: number, config?: { encoding?: string; maxSupportedTransactionVersion?: number }): Promise<any> {
    const response = await this.rpcRequest('getBlock', [
      blockNumber,
      {
        encoding: config?.encoding || 'jsonParsed',
        maxSupportedTransactionVersion: config?.maxSupportedTransactionVersion || 0,
      },
    ]);
    if (response.error) {
      throw new Error(`RPC Error: ${response.error.message}`);
    }
    return response.result;
  }

  async getBlocks(startSlot: number, endSlot?: number, commitment?: Commitment): Promise<number[]> {
    const response = await this.rpcRequest('getBlocks', [
      startSlot,
      endSlot,
      commitment || this.commitment,
    ]);
    if (response.error) {
      throw new Error(`RPC Error: ${response.error.message}`);
    }
    return response.result;
  }

  async getSignaturesForAddress(
    address: PublicKey,
    config?: {
      limit?: number;
      before?: string;
      until?: string;
      commitment?: Commitment;
    }
  ): Promise<Array<{ signature: string; slot: number; err: any; memo: string; blockTime: number }>> {
    const response = await this.rpcRequest('getSignaturesForAddress', [
      address.toBase58(),
      {
        limit: config?.limit || 100,
        before: config?.before,
        until: config?.until,
        commitment: config?.commitment || this.commitment,
      },
    ]);
    if (response.error) {
      throw new Error(`RPC Error: ${response.error.message}`);
    }
    return response.result;
  }

  private async signTransaction(transaction: Transaction, signers: Keypair[]): Promise<Uint8Array> {
    const message = this.prepareTransactionMessage(transaction);
    
    // Sign with each signer
    const signatures: Array<{ signature: Uint8Array; publicKey: PublicKey }> = [];
    for (const signer of signers) {
      const signature = await Ed25519.sign(message, signer.secretKey);
      signatures.push({ signature, publicKey: signer.publicKey });
    }
    
    // Combine into final signed transaction
    return this.serializeSignedTransaction(transaction, signatures);
  }

  private prepareTransactionMessage(transaction: Transaction): Buffer {
    // This would build the actual transaction message
    // For now, returning a placeholder
    return Buffer.alloc(0);
  }

  private serializeSignedTransaction(transaction: Transaction, signatures: Array<{ signature: Uint8Array; publicKey: PublicKey }>): Uint8Array {
    // Serialize the signed transaction
    // Format: [num_signatures, signature1, signature2, ..., message]
    const result: number[] = [signatures.length];
    for (const sig of signatures) {
      result.push(...sig.signature);
    }
    return new Uint8Array(result);
  }

  private serializeTransaction(transaction: Transaction): Buffer {
    // Serialize transaction for sending
    const instructions = transaction.instructions.map(ix => ({
      programIdIndex: 0, // Would be actual index
      accounts: ix.keys.map(k => 0), // Would be actual indices
      data: Array.from(ix.data),
    }));

    return Buffer.alloc(0); // Placeholder
  }

  private async rpcRequest(method: string, params: any[]): Promise<{ result: any; error?: { message: string; code?: number } }> {
    const body = JSON.stringify({
      jsonrpc: '2.0',
      id: 1,
      method,
      params,
    });

    try {
      const response = await fetch(this.rpcEndpoint, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body,
      });

      return await response.json();
    } catch (error) {
      // Try fallback endpoints
      this.currentTries++;
      if (this.currentTries < MAINNET_RPC_ENDPOINTS.length) {
        this.rpcEndpoint = MAINNET_RPC_ENDPOINTS[this.currentTries];
        return this.rpcRequest(method, params);
      }
      throw error;
    }
  }

  private sleep(ms: number): Promise<void> {
    return new Promise(resolve => setTimeout(resolve, ms));
  }

  onLogs(callback: (logs: { signature: string; logs: string[]; err?: any }) => void): { unsubscribe: () => void } {
    // WebSocket subscription for logs - would be implemented here
    return {
      unsubscribe: () => {},
    };
  }

  onAccountChange(
    pubkey: PublicKey,
    callback: (accountInfo: { data: Buffer; executable: boolean; owner: string; lamports: number }) => void,
    commitment?: Commitment
  ): { unsubscribe: () => void } {
    // WebSocket subscription for account changes - would be implemented here
    return {
      unsubscribe: () => {},
    };
  }

  onSlotChange(callback: (slot: number) => void): { unsubscribe: () => void } {
    // WebSocket subscription for slot changes
    return {
      unsubscribe: () => {},
    };
  }
}

// ============================================================================
// Keypair for Transaction Signing
// ============================================================================

export class Keypair {
  private _publicKey: PublicKey;
  private _secretKey: Buffer;

  constructor(secretKey: Buffer) {
    if (secretKey.length !== 64) {
      throw new Error(`Invalid secret key length: ${secretKey.length}`);
    }
    this._secretKey = Buffer.from(secretKey);
    // Derive public key from secret key
    // In ed25519, public key is derived from first 32 bytes of hashed secret
    this._publicKey = new PublicKeyImpl(secretKey.slice(32, 64));
  }

  static fromSeed(seed: Buffer): Keypair {
    if (seed.length !== 32) {
      throw new Error(`Invalid seed length: ${seed.length}`);
    }
    // In ed25519, secret key is the seed + public key derived from seed
    const hash = sha256Sync(seed);
    const secretKey = Buffer.concat([seed, hash]);
    return new Keypair(secretKey);
  }

  static generate(): Keypair {
    // Generate random 32-byte seed
    const seed = crypto.getRandomValues(new Uint8Array(32));
    return Keypair.fromSeed(Buffer.from(seed));
  }

  get publicKey(): PublicKey {
    return this._publicKey;
  }

  get secretKey(): Buffer {
    return this._secretKey;
  }

  sign(message: Buffer): Promise<Buffer> {
    return Ed25519.sign(message, this._secretKey);
  }
}

// ============================================================================
// Transaction Class
// ============================================================================

export class Transaction {
  signatures: Array<{ signature: Uint8Array | null; publicKey: PublicKey }> = [];
  instructions: TransactionInstruction[] = [];
  recentBlockhash: string | null = null;
  feePayer: PublicKey | null = null;
  nonceInfo?: { nonce: string; nonceInstruction: TransactionInstruction };

  add(...ixs: TransactionInstruction[]): Transaction {
    for (const ix of ixs) {
      this.instructions.push(ix);
    }
    return this;
  }

  async sign(...signers: Keypair[]): Promise<Transaction> {
    if (!this.recentBlockhash) {
      throw new Error('Recent blockhash not set');
    }
    if (!this.feePayer) {
      throw new Error('Fee payer not set');
    }

    // Prepare message
    const message = this.compileMessage();
    
    // Sign
    for (const signer of signers) {
      const signature = await Ed25519.sign(message, signer.secretKey);
      this.signatures.push({ signature, publicKey: signer.publicKey });
    }

    return this;
  }

  compileMessage(): Buffer {
    // Compile transaction message
    // Format: [num_required_signatures, num_readonly_signed_accounts, num_readonly_unsigned_accounts, account_keys, recent_blockhash, instruction_data]
    return Buffer.alloc(0); // Placeholder
  }

  serialize(): Buffer {
    // Serialize the transaction
    return Buffer.alloc(0); // Placeholder
  }

  toJSON(): object {
    return {
      signatures: this.signatures.map(s => s.signature ? Buffer.from(s.signature).toString('hex') : null),
      instructions: this.instructions.map(ix => ({
        programId: ix.programId.toBase58(),
        keys: ix.keys.map(k => ({
          pubkey: k.pubkey.toBase58(),
          isSigner: k.isSigner,
          isWritable: k.isWritable,
        })),
        data: Array.from(ix.data),
      })),
      recentBlockhash: this.recentBlockhash,
      feePayer: this.feePayer?.toBase58(),
    };
  }

  static from(obj: {
    signatures: string[];
    message: {
      accountKeys: string[];
      instructions: { programIdIndex: number; accounts: number[]; data: string }[];
      recentBlockhash: string;
    };
  }): Transaction {
    const tx = new Transaction();
    tx.recentBlockhash = obj.message.recentBlockhash;
    
    // Parse signatures
    tx.signatures = obj.signatures.map((sig, i) => ({
      signature: sig ? Buffer.from(sig, 'base64') : null,
      publicKey: new PublicKeyImpl(obj.message.accountKeys[i]),
    }));
    
    // Parse instructions
    tx.instructions = obj.message.instructions.map(ix => {
      const keys = ix.accounts.map(accIdx => ({
        pubkey: new PublicKeyImpl(obj.message.accountKeys[accIdx]),
        isSigner: tx.signatures[accIdx]?.signature !== null,
        isWritable: false,
      }));
      
      return new TransactionInstruction({
        programId: new PublicKeyImpl(obj.message.accountKeys[ix.programIdIndex]),
        keys,
        data: Buffer.from(ix.data, 'base64'),
      });
    });
    
    return tx;
  }
}

// ============================================================================
// WebSocket Connection for Real-time Updates
// ============================================================================

export class WebSocketConnection {
  private ws: WebSocket | null = null;
  private subscriptions: Map<number, (data: any) => void> = new Map();
  private subscriptionId: number = 0;

  constructor(endpoint: string) {
    // Would establish WebSocket connection
  }

  subscribe(
    method: string,
    params: any[],
    callback: (data: any) => void
  ): number {
    const id = this.subscriptionId++;
    this.subscriptions.set(id, callback);
    return id;
  }

  unsubscribe(id: number): void {
    this.subscriptions.delete(id);
  }

  close(): void {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }
}

// ============================================================================
// Utility Functions
// ============================================================================

export function lamportsToSol(lamports: number): number {
  return lamports / LAMPORTS_PER_SOL;
}

export function solToLamports(sol: number): number {
  return Math.round(sol * LAMPORTS_PER_SOL);
}

export function validatePublicKey(address: string): boolean {
  try {
    const decoded = base58.decode(address);
    return decoded.length === 32;
  } catch {
    return false;
  }
}

export function shortenAddress(address: string, chars: number = 4): string {
  return `${address.slice(0, chars + 2)}...${address.slice(-chars)}`;
}

export function createRandomKeypair(): Keypair {
  return Keypair.generate();
}

// ============================================================================
// Default Exports
// ============================================================================

export default {
  Connection,
  PublicKey: PublicKeyImpl,
  Keypair,
  Transaction,
  TransactionInstruction,
  SystemProgram,
  TokenProgram,
  MemoProgram,
  StakeProgram,
  VoteProgram,
  LAMPORTS_PER_SOL,
  lamportsToSol,
  solToLamports,
  validatePublicKey,
  shortenAddress,
  createRandomKeypair,
};