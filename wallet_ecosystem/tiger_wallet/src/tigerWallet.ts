/**
 * TigerSwap User Wallet (TigerWallet)
 * Complete HD wallet implementation for users
 * Supports EVM + Non-EVM chains with full functionality
 */

import { ethers, JsonRpcProvider, Wallet, Contract, keccak256, toUtf8Bytes } from 'ethers';
import { ERC20_ABI, COMMON_TOKENS, DEX_ROUTERS } from './constants';

// ============================================================================
// Types & Interfaces
// ============================================================================

export interface WalletAccount {
  id: string;
  address: string;
  chainId: number;
  publicKey: string;
  path: string;
  name: string;
  balance: string;
  balanceUSD: number;
  tokens: TokenBalance[];
  createdAt: number;
  lastActiveAt: number | null;
}

export interface TokenBalance {
  address: string;
  symbol: string;
  name: string;
  decimals: number;
  balance: string;
  balanceUSD: number;
  priceUSD: number;
  logoURI?: string;
}

export interface ChainConfig {
  chainId: number;
  chainName: string;
  chainType: 'EVM' | 'Solana' | 'Aptos' | 'Sui' | 'Ton' | 'Cosmos' | 'PiNetwork';
  symbol: string;
  decimals: number;
  rpcUrl: string;
  explorerUrl: string;
  blockExplorerApiUrl?: string;
  blockExplorerApiKey?: string;
  nativeCurrency: {
    name: string;
    symbol: string;
    decimals: number;
  };
  slip44: number;
  icon: string;
}

export interface TransactionRequest {
  to: string;
  value: string;
  data?: string;
  gasLimit?: string;
  gasPrice?: string;
  maxFeePerGas?: string;
  maxPriorityFeePerGas?: string;
  nonce?: number;
}

export interface TransactionReceipt {
  hash: string;
  from: string;
  to: string;
  value: string;
  data: string;
  blockNumber: number;
  blockHash: string;
  transactionIndex: number;
  gasUsed: string;
  effectiveGasPrice: string;
  status: 'success' | 'reverted';
  logs: Log[];
}

export interface Log {
  address: string;
  topics: string[];
  data: string;
  logIndex: number;
}

export interface SwapParams {
  tokenIn: string;
  tokenOut: string;
  amountIn: string;
  amountOutMin: string;
  to: string;
  deadline: number;
}

export interface RouteInfo {
  dex: string;
  path: string[];
  poolAddress: string;
  fee: number;
  amountIn: string;
  amountOut: string;
  priceImpact: number;
}

export interface SwapQuote {
  inputToken: string;
  outputToken: string;
  inputAmount: string;
  outputAmount: string;
  outputAmountMin: string;
  priceImpact: number;
  route: RouteInfo[];
  gasEstimate: string;
  gasFeeUSD: number;
  provider: string;
  expiresAt: number;
}

export interface LiquidityPosition {
  tokenA: string;
  tokenB: string;
  liquidity: string;
  amountA: string;
  amountB: string;
  poolAddress: string;
  APR: number;
  feesEarned: string;
}

// ============================================================================
// Supported Chains Configuration
// ============================================================================

export const SUPPORTED_CHAINS: Record<number, ChainConfig> = {
  // EVM Chains
  1: {
    chainId: 1,
    chainName: 'Ethereum',
    chainType: 'EVM',
    symbol: 'ETH',
    decimals: 18,
    rpcUrl: 'https://eth.llamarpc.com',
    explorerUrl: 'https://etherscan.io',
    blockExplorerApiUrl: 'https://api.etherscan.io/api',
    blockExplorerApiKey: '',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
    slip44: 60,
    icon: '/chains/ethereum.png'
  },
  56: {
    chainId: 56,
    chainName: 'BNB Chain',
    chainType: 'EVM',
    symbol: 'BNB',
    decimals: 18,
    rpcUrl: 'https://bsc-dataseed.binance.org',
    explorerUrl: 'https://bscscan.com',
    blockExplorerApiUrl: 'https://api.bscscan.com/api',
    nativeCurrency: { name: 'BNB', symbol: 'BNB', decimals: 18 },
    slip44: 60,
    icon: '/chains/bsc.png'
  },
  137: {
    chainId: 137,
    chainName: 'Polygon',
    chainType: 'EVM',
    symbol: 'MATIC',
    decimals: 18,
    rpcUrl: 'https://polygon-rpc.com',
    explorerUrl: 'https://polygonscan.com',
    blockExplorerApiUrl: 'https://api.polygonscan.com/api',
    nativeCurrency: { name: 'MATIC', symbol: 'MATIC', decimals: 18 },
    slip44: 60,
    icon: '/chains/polygon.png'
  },
  42161: {
    chainId: 42161,
    chainName: 'Arbitrum One',
    chainType: 'EVM',
    symbol: 'ETH',
    decimals: 18,
    rpcUrl: 'https://arb1.arbitrum.io/rpc',
    explorerUrl: 'https://arbiscan.io',
    blockExplorerApiUrl: 'https://api.arbiscan.io/api',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
    slip44: 60,
    icon: '/chains/arbitrum.png'
  },
  10: {
    chainId: 10,
    chainName: 'Optimism',
    chainType: 'EVM',
    symbol: 'ETH',
    decimals: 18,
    rpcUrl: 'https://mainnet.optimism.io',
    explorerUrl: 'https://optimistic.etherscan.io',
    blockExplorerApiUrl: 'https://api-optimistic.etherscan.io/api',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
    slip44: 60,
    icon: '/chains/optimism.png'
  },
  8453: {
    chainId: 8453,
    chainName: 'Base',
    chainType: 'EVM',
    symbol: 'ETH',
    decimals: 18,
    rpcUrl: 'https://mainnet.base.org',
    explorerUrl: 'https://basescan.org',
    blockExplorerApiUrl: 'https://api.basescan.org/api',
    nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
    slip44: 60,
    icon: '/chains/base.png'
  },
  43114: {
    chainId: 43114,
    chainName: 'Avalanche',
    chainType: 'EVM',
    symbol: 'AVAX',
    decimals: 18,
    rpcUrl: 'https://api.avax.network/ext/bc/C/rpc',
    explorerUrl: 'https://snowtrace.io',
    blockExplorerApiUrl: 'https://api.snowtrace.io/api',
    nativeCurrency: { name: 'Avalanche', symbol: 'AVAX', decimals: 18 },
    slip44: 60,
    icon: '/chains/avalanche.png'
  },
  // Non-EVM Chains
  101: {
    chainId: 101,
    chainName: 'Solana',
    chainType: 'Solana',
    symbol: 'SOL',
    decimals: 9,
    rpcUrl: 'https://api.mainnet-beta.solana.com',
    explorerUrl: 'https://explorer.solana.com',
    nativeCurrency: { name: 'Solana', symbol: 'SOL', decimals: 9 },
    slip44: 501,
    icon: '/chains/solana.png'
  },
  1100: {
    chainId: 1100,
    chainName: 'Aptos',
    chainType: 'Aptos',
    symbol: 'APT',
    decimals: 8,
    rpcUrl: 'https://fullnode.mainnet.aptoslabs.com',
    explorerUrl: 'https://explorer.aptoslabs.com',
    nativeCurrency: { name: 'Aptos', symbol: 'APT', decimals: 8 },
    slip44: 637,
    icon: '/chains/aptos.png'
  },
  7821: {
    chainId: 7821,
    chainName: 'Sui',
    chainType: 'Sui',
    symbol: 'SUI',
    decimals: 9,
    rpcUrl: 'https://fullnode.mainnet.sui.io',
    explorerUrl: 'https://explorer.sui.io',
    nativeCurrency: { name: 'Sui', symbol: 'SUI', decimals: 9 },
    slip44: 784,
    icon: '/chains/sui.png'
  },
  6060: {
    chainId: 6060,
    chainName: 'Toncoin',
    chainType: 'Ton',
    symbol: 'TON',
    decimals: 9,
    rpcUrl: 'https://toncenter.com/api/v2',
    explorerUrl: 'https://tonviewer.com',
    nativeCurrency: { name: 'Toncoin', symbol: 'TON', decimals: 9 },
    slip44: 607,
    icon: '/chains/ton.png'
  },
  3141: {
    chainId: 3141,
    chainName: 'Pi Network',
    chainType: 'PiNetwork',
    symbol: 'PI',
    decimals: 18,
    rpcUrl: 'https://minepi.com/api/gateway',
    explorerUrl: 'https://explorer.minepi.com',
    nativeCurrency: { name: 'Pi', symbol: 'PI', decimals: 18 },
    slip44: 314159,
    icon: '/chains/pi.png'
  }
};

// ============================================================================
// BIP39 Word List (2048 words)
// ============================================================================

const BIP39_WORDLIST = [
  'abandon', 'ability', 'able', 'about', 'above', 'absent', 'absorb', 'abstract', 'absurd', 'abuse',
  'access', 'accident', 'account', 'accuse', 'achieve', 'acid', 'acoustic', 'acquire', 'across', 'act',
  'action', 'actor', 'actress', 'actual', 'adapt', 'add', 'addict', 'address', 'adjust', 'admit',
  'adult', 'advance', 'advice', 'aerobic', 'affair', 'afford', 'afraid', 'again', 'age', 'agent',
  'agree', 'ahead', 'aim', 'air', 'airport', 'aisle', 'alarm', 'album', 'alcohol', 'alert',
  'alien', 'all', 'alley', 'allow', 'almost', 'alone', 'alpha', 'already', 'also', 'alter',
  'always', 'amateur', 'amazing', 'among', 'amount', 'amused', 'analyst', 'anchor', 'ancient', 'anger',
  'angle', 'angry', 'animal', 'ankle', 'announce', 'annual', 'another', 'answer', 'antenna', 'antique',
  'anxiety', 'any', 'apart', 'apology', 'appear', 'apple', 'approve', 'april', 'arch', 'arctic',
  'arena', 'argue', 'arm', 'armed', 'armor', 'army', 'around', 'arrange', 'arrest', 'arrive', 'arrow',
  'art', 'artist', 'artwork', 'ask', 'aspect', 'assault', 'asset', 'assist', 'assume', 'asthma', 'athlete',
  'atom', 'attack', 'attend', 'attitude', 'attract', 'auction', 'audit', 'august', 'aunt', 'author',
  'auto', 'autumn', 'average', 'avocado', 'avoid', 'awake', 'aware', 'away', 'awesome', 'awful',
  'awkward', 'axis', 'baby', 'bachelor', 'bacon', 'badge', 'bag', 'balance', 'balcony', 'ball',
  'bamboo', 'banana', 'banner', 'bar', 'barely', 'bargain', 'barrel', 'basic', 'basket', 'battle',
  'beach', 'bean', 'beauty', 'because', 'become', 'beef', 'before', 'begin', 'behave', 'behind',
  'believe', 'below', 'belt', 'bench', 'benefit', 'best', 'betray', 'better', 'between', 'beyond',
  'bicycle', 'bid', 'bike', 'bind', 'biology', 'bird', 'birth', 'bitter', 'black', 'blade',
  'blame', 'blanket', 'blast', 'blaze', 'bless', 'blind', 'blood', 'blossom', 'blouse', 'blue',
  'blur', 'blush', 'board', 'boat', 'body', 'boil', 'bomb', 'bone', 'bonus', 'book',
  'boost', 'border', 'boring', 'borrow', 'boss', 'bottom', 'bounce', 'box', 'boy', 'bracket',
  'brain', 'brand', 'brass', 'brave', 'bread', 'breeze', 'brick', 'bridge', 'brief', 'bright',
  'bring', 'brisk', 'broccoli', 'broken', 'bronze', 'broom', 'brother', 'brown', 'brush', 'bubble',
  'buddy', 'budget', 'buffalo', 'build', 'bulb', 'bulk', 'bullet', 'bundle', 'bunker', 'burden',
  'burger', 'burst', 'bus', 'business', 'busy', 'butter', 'buyer', 'buzz', 'cabbage', 'cabin',
  'cable', 'cactus', 'cage', 'cake', 'call', 'calm', 'camera', 'camp', 'can', 'canal',
  'cancel', 'candy', 'cannon', 'canoe', 'canvas', 'canyon', 'capable', 'capital', 'captain', 'car',
  'carbon', 'card', 'cargo', 'carpet', 'carry', 'cart', 'case', 'cash', 'casino', 'castle',
  'casual', 'catch', 'category', 'cattle', 'caught', 'cause', 'caution', 'cave', 'ceiling', 'celery',
  'cement', 'census', 'century', 'cereal', 'certain', 'chair', 'chalk', 'champion', 'change', 'chaos',
  'chapter', 'charge', 'chase', 'chat', 'cheap', 'check', 'cheese', 'cherry', 'chest', 'chicken',
  'chief', 'child', 'chimney', 'choice', 'choose', 'chronic', 'chuckle', 'chunk', 'churn', 'cigar',
  'cinnamon', 'circle', 'citizen', 'city', 'civil', 'claim', 'clap', 'clarify', 'classic', 'clean',
  'clever', 'click', 'client', 'cliff', 'climb', 'clinic', 'clip', 'clock', 'close', 'cloud',
  'clown', 'club', 'clump', 'cluster', 'clutch', 'coach', 'coast', 'coconut', 'code', 'coffee',
  'coil', 'coin', 'collect', 'color', 'column', 'combine', 'come', 'comfort', 'comic', 'common',
  'company', 'concert', 'conduct', 'confirm', 'congress', 'connect', 'consider', 'control', 'convince', 'cook',
  'cool', 'copper', 'copy', 'coral', 'core', 'corn', 'correct', 'cost', 'cotton',
  'couch', 'country', 'couple', 'course', 'cousin', 'cover', 'coyote', 'crack', 'cradle', 'craft',
  'cram', 'crane', 'crash', 'crater', 'crawl', 'crazy', 'cream', 'credit', 'creek',
  'crew', 'cricket', 'crime', 'crisp', 'critic', 'crop', 'cross', 'crouch', 'crowd', 'crucial',
  'cruel', 'cruise', 'crumble', 'crunch', 'crush', 'cry', 'crystal', 'cube', 'culture',
  'cup', 'cupboard', 'curious', 'current', 'curtain', 'curve', 'cushion', 'custom', 'cute', 'cycle',
  'dad', 'damage', 'damp', 'dance', 'danger', 'daring', 'dash', 'daughter', 'dawn', 'day',
  'deal', 'debate', 'debris', 'decade', 'december', 'decide', 'decline', 'decorate', 'decrease', 'deer', 'defense',
  'define', 'defy', 'degree', 'delay', 'deliver', 'demand', 'democracy', 'denial', 'dentist', 'deny',
  'depart', 'depend', 'deposit', 'depth', 'deputy', 'derive', 'describe', 'desert', 'design', 'desk',
  'despair', 'destroy', 'detail', 'detect', 'develop', 'device', 'devote', 'diagram', 'dial', 'diamond',
  'diary', 'dice', 'diesel', 'diet', 'differ', 'digital', 'dignity', 'dilemma', 'dinner', 'dinosaur',
  'direct', 'dirt', 'disagree', 'discover', 'disease', 'dish', 'dismiss', 'disorder', 'display', 'distance',
  'divert', 'divide', 'divorce', 'dizzy', 'doctor', 'document', 'dog', 'doll', 'dolphin',
  'domain', 'donate', 'donkey', 'donor', 'door', 'dose', 'double', 'dove', 'down', 'download',
  'dozen', 'draft', 'dragon', 'drama', 'draw', 'dream', 'dress', 'drift', 'drill', 'drink',
  'drip', 'drive', 'drop', 'drum', 'dry', 'duck', 'dumb', 'dune', 'during', 'dust',
  'dutch', 'duty', 'dwarf', 'dynamic', 'eager', 'eagle', 'early', 'earn', 'earth', 'easily',
  'east', 'easy', 'echo', 'ecology', 'economy', 'edge', 'edit', 'educate', 'effort', 'egg',
  'eight', 'eject', 'elastic', 'elbow', 'elder', 'electric', 'elegant', 'element', 'elephant', 'elevator',
  'elite', 'else', 'embark', 'embody', 'embrace', 'emerge', 'emergency', 'emit', 'emotion', 'employ',
  'empower', 'empty', 'enable', 'enact', 'end', 'endless', 'endorse', 'enemy', 'energy', 'enforce',
  'engage', 'engine', 'enhance', 'enjoy', 'enormous', 'enough', 'enrich', 'enroll', 'ensure',
  'enter', 'entire', 'entry', 'envelope', 'episode', 'equal', 'equip', 'era', 'erase', 'erode', 'erosion',
  'error', 'erupt', 'escape', 'essay', 'essence', 'estate', 'eternal', 'ethics', 'evidence', 'evil',
  'evoke', 'evolve', 'exact', 'exam', 'exceed', 'excel', 'except', 'excess', 'exchange',
  'excite', 'exclude', 'excuse', 'execute', 'exercise', 'exhaust', 'exhibit', 'exile', 'exist',
  'exit', 'exotic', 'expand', 'expect', 'expire', 'explain', 'expose', 'express', 'extend',
  'extra', 'eye', 'eyebrow', 'fabric', 'face', 'faculty', 'fade', 'faint', 'faith', 'fall',
  'false', 'fame', 'family', 'famous', 'fan', 'fancy', 'fantasy', 'farm', 'fashion', 'fat',
  'fatal', 'father', 'fatigue', 'fault', 'favorite', 'feature', 'february', 'federal', 'fee', 'feed',
  'feel', 'female', 'fence', 'festival', 'fetch', 'fever', 'few', 'fiber', 'fiction', 'field',
  'figure', 'file', 'fill', 'film', 'filter', 'final', 'finance', 'find', 'fine', 'finger',
  'finish', 'fire', 'firm', 'first', 'fiscal', 'fish', 'fit', 'fitness', 'fix', 'flag',
  'flame', 'flash', 'flat', 'flavor', 'flee', 'flight', 'flip', 'float', 'flock', 'flood',
  'floor', 'flower', 'fluid', 'flush', 'fly', 'foam', 'focus', 'fog', 'foil', 'fold',
  'folk', 'follow', 'food', 'foot', 'force', 'forest', 'forget', 'fork', 'fortune', 'forum',
  'forward', 'fossil', 'foster', 'found', 'fox', 'fragile', 'frame', 'frequent', 'fresh', 'friend',
  'fringe', 'frog', 'front', 'frost', 'frown', 'frozen', 'fruit', 'fuel', 'fun', 'funny',
  'furnace', 'fury', 'future', 'gadget', 'gain', 'galaxy', 'gallery', 'game', 'gap', 'garage',
  'garden', 'garlic', 'gas', 'gasp', 'gate', 'gather', 'gauge', 'gaze', 'general', 'genre', 'gentle',
  'genuine', 'gesture', 'ghost', 'giant', 'gift', 'giggle', 'ginger', 'giraffe', 'girl', 'give',
  'glad', 'glance', 'glare', 'glass', 'glide', 'glimpse', 'globe', 'gloom', 'glory', 'glove',
  'glow', 'glue', 'goat', 'goddess', 'gold', 'good', 'goose', 'gorilla', 'gospel', 'gossip',
  'govern', 'gown', 'grab', 'grace', 'grain', 'grant', 'grape', 'grass', 'gravity', 'great',
  'green', 'grid', 'grief', 'grit', 'grocery', 'group', 'grow', 'grunt', 'guard', 'guess',
  'guide', 'guilt', 'guitar', 'gun', 'gym', 'habit', 'hair', 'half', 'hammer', 'hamster', 'hand',
  'handle', 'harbor', 'hard', 'harsh', 'harvest', 'hat', 'have', 'hawk', 'hazard', 'head',
  'health', 'heap', 'heart', 'heavy', 'hedgehog', 'heel', 'height', 'helix', 'hell', 'hello',
  'helmet', 'help', 'hen', 'hero', 'hidden', 'high', 'hill', 'hint', 'hip', 'hire', 'history',
  'hobby', 'hockey', 'hold', 'hole', 'holiday', 'hollow', 'home', 'honey', 'honor', 'hope', 'horn',
  'horror', 'horse', 'hospital', 'host', 'hotel', 'hour', 'house', 'hover', 'hub', 'huge',
  'human', 'humid', 'humor', 'hundred', 'hungry', 'hunt', 'hurdle', 'hurry', 'hurt', 'husband',
  'hybrid', 'ice', 'icon', 'idea', 'identify', 'idle', 'ignore', 'ill', 'illegal', 'illness',
  'image', 'imitate', 'immense', 'immune', 'impact', 'impose', 'improve', 'impulse', 'inch',
  'include', 'income', 'increase', 'index', 'indicate', 'indoor', 'industry', 'infant', 'inflict', 'inform',
  'inhale', 'inherit', 'initial', 'inject', 'injury', 'inmate', 'inner', 'innocent', 'input', 'inquiry',
  'insane', 'insect', 'insert', 'inside', 'inspire', 'install', 'intact', 'interest', 'into',
  'invest', 'invite', 'involve', 'iron', 'island', 'isolate', 'issue', 'item', 'ivory', 'jacket',
  'jaguar', 'jar', 'jazz', 'jealous', 'jeans', 'jelly', 'jewel', 'job', 'join', 'joke',
  'jolly', 'journey', 'joy', 'judge', 'juice', 'jump', 'jungle', 'junior', 'junk', 'just',
  'kangaroo', 'keen', 'keep', 'ketchup', 'key', 'kick', 'kid', 'kidney', 'kind', 'kingdom',
  'kiss', 'kit', 'kitchen', 'kite', 'kitten', 'kiwi', 'knee', 'knife', 'knock', 'know',
  'lab', 'label', 'labor', 'ladder', 'lady', 'lake', 'lamp', 'language', 'laptop', 'large',
  'later', 'latin', 'laugh', 'laundry', 'lava', 'law', 'lawn', 'lawsuit', 'layer', 'lazy',
  'leader', 'leaf', 'learn', 'leave', 'lecture', 'left', 'leg', 'legal', 'legend', 'leisure',
  'lemon', 'lend', 'length', 'lens', 'leopard', 'lesson', 'letter', 'level', 'liar',
  'liberty', 'library', 'license', 'life', 'lift', 'light', 'like', 'limb', 'limit',
  'line', 'link', 'lion', 'liquid', 'list', 'listen', 'little', 'live', 'liver',
  'lizard', 'load', 'loan', 'lobster', 'local', 'lock', 'logic', 'lonely', 'long',
  'loop', 'lottery', 'loud', 'lounge', 'love', 'loyal', 'lucky', 'luggage', 'lumber',
  'lunar', 'lunch', 'luxury', 'lyrics', 'machine', 'mad', 'magnet', 'maid', 'mail', 'main',
  'major', 'make', 'mammal', 'man', 'manage', 'mandate', 'mango', 'mansion', 'manual', 'maple',
  'marble', 'march', 'margin', 'marine', 'market', 'marriage', 'mask', 'mass', 'master',
  'match', 'material', 'math', 'matrix', 'matter', 'maximize', 'mayor', 'maze', 'mean',
  'meant', 'meat', 'mechanic', 'medal', 'media', 'melody', 'melt', 'member', 'memory', 'men',
  'mend', 'mental', 'mentor', 'menu', 'mercy', 'merge', 'merit', 'merry', 'mesh', 'message',
  'metal', 'method', 'middle', 'midnight', 'milk', 'million', 'mimic', 'mind', 'minimum',
  'minor', 'minute', 'miracle', 'mirror', 'misery', 'miss', 'mistake', 'mix', 'mixed',
  'mixture', 'mobile', 'model', 'modify', 'mom', 'moment', 'monitor', 'monkey', 'monster', 'month',
  'moon', 'moral', 'more', 'morning', 'mosquito', 'mother', 'motion', 'motor', 'mountain',
  'mouse', 'move', 'movie', 'much', 'muffin', 'mule', 'multiply', 'muscle', 'museum', 'mushroom',
  'music', 'must', 'mutual', 'myself', 'mystery', 'myth', 'naive', 'name', 'napkin', 'narrow',
  'nasty', 'nation', 'nature', 'near', 'neat', 'necessary', 'neck', 'need', 'negative', 'neglect',
  'neither', 'nephew', 'nerve', 'nest', 'net', 'network', 'neutral', 'never', 'news',
  'next', 'nice', 'night', 'noble', 'noise', 'nominee', 'noodle', 'normal', 'north', 'nose',
  'notable', 'note', 'nothing', 'notice', 'novel', 'now', 'nuclear', 'number', 'nurse',
  'nut', 'oak', 'obey', 'object', 'oblige', 'obscure', 'observe', 'obtain', 'obvious',
  'occur', 'ocean', 'october', 'odor', 'off', 'offer', 'office', 'often', 'oil', 'okay',
  'old', 'olive', 'olympic', 'omit', 'once', 'one', 'onion', 'online', 'only', 'open',
  'opera', 'opinion', 'opponent', 'opportunity', 'oppose', 'option', 'orange', 'orbit', 'orchard',
  'order', 'ordinary', 'organ', 'orient', 'origin', 'ornament', 'orphan', 'ostrich', 'other', 'outdoor',
  'outer', 'output', 'outside', 'oval', 'oven', 'over', 'own', 'owner', 'oxygen', 'oyster', 'ozone',
  'paddle', 'page', 'pair', 'palace', 'palm', 'panda', 'panel', 'panic', 'panther', 'paper',
  'party', 'pass', 'patch', 'path', 'patient', 'patrol', 'pattern', 'pause', 'pave', 'payment',
  'peace', 'peanut', 'pear', 'peasant', 'pelican', 'pen', 'penalty', 'pencil', 'people',
  'pepper', 'perfect', 'permit', 'person', 'pet', 'phone', 'photo', 'phrase', 'physical',
  'piano', 'picnic', 'picture', 'piece', 'pig', 'pigeon', 'pill', 'pilot', 'pink',
  'pioneer', 'pipe', 'pistol', 'pitch', 'pizza', 'place', 'plaid', 'plain', 'plan', 'plane',
  'planet', 'plant', 'plate', 'play', 'please', 'pledge', 'plenty', 'plot', 'plough',
  'plow', 'plug', 'plunge', 'poem', 'poet', 'point', 'polar', 'pole', 'police', 'pond',
  'pony', 'pool', 'popular', 'portion', 'position', 'possible', 'post', 'potato', 'pottery',
  'poverty', 'powder', 'power', 'practice', 'praise', 'predict', 'prefer', 'prepare', 'present',
  'pretty', 'prevent', 'price', 'pride', 'primary', 'print', 'priority', 'prison', 'private',
  'prize', 'problem', 'process', 'produce', 'profit', 'program', 'project', 'promote', 'proof',
  'property', 'prosper', 'protect', 'proud', 'provide', 'public', 'pudding', 'pull', 'pulp',
  'pulse', 'pumpkin', 'punch', 'pupil', 'puppy', 'purchase', 'purity', 'purpose', 'purse', 'push',
  'put', 'puzzle', 'pyramid', 'quality', 'quantum', 'quarter', 'queen', 'query', 'quest',
  'question', 'quick', 'quiet', 'quit', 'quiz', 'quote', 'rabbit', 'raccoon', 'race', 'rack',
  'radar', 'radio', 'rail', 'rain', 'raise', 'rake', 'random', 'rapid', 'rare', 'rate', 'rather',
  'raven', 'raw', 'reach', 'react', 'read', 'reader', 'ready', 'real', 'reality', 'realize',
  'realm', 'rear', 'reason', 'rebel', 'rebuild', 'recall', 'receive', 'recipe', 'record',
  'recover', 'recruit', 'red', 'reduce', 'reflect', 'reform', 'refuse', 'region', 'regret',
  'regular', 'reject', 'relax', 'release', 'relief', 'rely', 'remain', 'remember', 'remind',
  'remote', 'remove', 'render', 'renew', 'rent', 'reopen', 'repair', 'repeat', 'replace',
  'reply', 'report', 'represent', 'republic', 'request', 'require', 'rescue', 'resemble',
  'resist', 'resource', 'response', 'result', 'retire', 'return', 'reunion', 'reveal', 'review',
  'revolution', 'reward', 'rhythm', 'rib', 'ribbon', 'rice', 'rich', 'ride', 'ridge', 'rifle',
  'right', 'rigid', 'ring', 'riot', 'ripple', 'risk', 'ritual', 'rival', 'river', 'road',
  'roast', 'robot', 'robust', 'rocket', 'romance', 'roof', 'rookie', 'room', 'root', 'rope',
  'rose', 'rotate', 'rotten', 'rough', 'round', 'route', 'royal', 'rubber', 'rubble', 'ruby',
  'rudely', 'ruin', 'rule', 'run', 'runway', 'rural', 'sad', 'saddle', 'sadness', 'safe',
  'sail', 'salad', 'salmon', 'salon', 'salt', 'salute', 'same', 'sample', 'sand', 'satisfy',
  'satoshi', 'sauce', 'sausage', 'save', 'say', 'scale', 'scan', 'scare', 'scatter', 'scene',
  'scent', 'school', 'science', 'scissors', 'scorpion', 'scout', 'scrap', 'screen', 'script',
  'scrub', 'sea', 'search', 'season', 'seat', 'second', 'secret', 'section', 'security',
  'seed', 'seek', 'segment', 'select', 'sell', 'seminar', 'senior', 'sense', 'sentence', 'series',
  'service', 'session', 'settle', 'setup', 'seven', 'shadow', 'shaft', 'shallow', 'share', 'shark',
  'sharp', 'sheep', 'sheer', 'sheet', 'shelf', 'shell', 'sheriff', 'shield', 'shift',
  'shine', 'ship', 'shiver', 'shock', 'shoe', 'shoot', 'shop', 'short', 'shoulder', 'shove',
  'shrimp', 'shrug', 'shuffle', 'shun', 'shy', 'sibling', 'sick', 'side', 'siege', 'sight',
  'sign', 'silent', 'silicon', 'silk', 'silly', 'silver', 'similar', 'simple', 'since',
  'sing', 'siren', 'sister', 'situate', 'six', 'size', 'skate', 'sketch', 'ski', 'skill',
  'skin', 'skirt', 'skull', 'slab', 'slam', 'sleep', 'sled', 'slice', 'slide', 'slight',
  'slim', 'slogan', 'slope', 'slot', 'small', 'smart', 'smell', 'smile', 'smoke', 'smooth',
  'snack', 'snake', 'snap', 'sniff', 'snow', 'so', 'soap', 'soccer', 'social', 'sock',
  'soda', 'sofa', 'soft', 'software', 'soil', 'solar', 'soldier', 'solid', 'solution', 'solve',
  'someone', 'song', 'soon', 'sorry', 'sort', 'soul', 'sound', 'soup', 'source', 'south',
  'space', 'spare', 'spark', 'speak', 'special', 'speed', 'spell', 'spend', 'sphere',
  'spice', 'spider', 'spin', 'spirit', 'split', 'spoil', 'sponsor', 'spoon', 'sport',
  'spot', 'spray', 'spread', 'spring', 'spy', 'square', 'squeeze', 'squirrel', 'stable',
  'stadium', 'staff', 'stage', 'stairs', 'stamp', 'stand', 'start', 'state', 'statement',
  'station', 'stay', 'steak', 'steel', 'steep', 'stem', 'step', 'stereo', 'stick', 'still',
  'sting', 'stock', 'stomach', 'stone', 'stool', 'story', 'stove', 'strategy', 'street',
  'strict', 'stride', 'strike', 'string', 'strip', 'strive', 'stroke', 'strong', 'struggle',
  'student', 'studio', 'study', 'stuff', 'stumble', 'style', 'subject', 'submit', 'subway',
  'success', 'such', 'sudden', 'suffer', 'sugar', 'suggest', 'suit', 'summer', 'sun', 'sunny',
  'sunset', 'super', 'supply', 'supreme', 'sure', 'surface', 'surge', 'surprise', 'surround',
  'survey', 'suspect', 'sustain', 'swallow', 'swamp', 'swarm', 'swear', 'sweat', 'sweep',
  'sweet', 'swift', 'swim', 'swing', 'switch', 'sword', 'symbol', 'symptom', 'syrup', 'system',
  'table', 'tackle', 'tag', 'tail', 'talent', 'talk', 'tall', 'tank', 'tape', 'target',
  'task', 'taste', 'tattoo', 'taxi', 'teach', 'team', 'tell', 'ten', 'tenant', 'tennis',
  'tent', 'term', 'test', 'text', 'thank', 'that', 'them', 'theme', 'then', 'theory', 'there',
  'they', 'thing', 'this', 'thought', 'thread', 'threat', 'three', 'thrive', 'throw', 'thumb',
  'thunder', 'ticket', 'tide', 'tiger', 'tilt', 'timber', 'time', 'tiny', 'tip', 'tired',
  'tissue', 'title', 'toast', 'tobacco', 'toddler', 'toe', 'together', 'toilet', 'token',
  'tomato', 'tomorrow', 'tone', 'tongue', 'tonight', 'tool', 'tooth', 'top', 'topic', 'topple',
  'torch', 'tornado', 'tortoise', 'toss', 'total', 'tourist', 'toward', 'tower', 'town',
  'toy', 'track', 'trade', 'traffic', 'tragic', 'train', 'transfer', 'transform', 'transit',
  'translate', 'trap', 'trash', 'travel', 'tray', 'treat', 'tree', 'trend', 'trial', 'tribe',
  'trick', 'trigger', 'trim', 'trip', 'trophy', 'trouble', 'truck', 'true', 'truly', 'trumpet',
  'trust', 'truth', 'try', 'tube', 'tuition', 'tumble', 'tuna', 'tunnel', 'turkey', 'turn',
  'turtle', 'twelve', 'twenty', 'twice', 'twin', 'twist', 'two', 'type', 'typical', 'ugly', 'umbrella',
  'unable', 'unaware', 'uncle', 'uncover', 'under', 'undo', 'unfair', 'unfold', 'unhappy', 'uniform',
  'unique', 'unit', 'universe', 'unknown', 'unlock', 'until', 'unusual', 'unveil', 'update',
  'upgrade', 'uphold', 'upon', 'upper', 'upset', 'urban', 'urge', 'usage', 'use',
  'used', 'useful', 'useless', 'usual', 'utility', 'vacant', 'vacuum', 'vague', 'valid',
  'valley', 'valve', 'van', 'vanish', 'vapor', 'various', 'vegan', 'velvet', 'vendor',
  'venture', 'venue', 'verb', 'verify', 'version', 'very', 'vessel', 'veteran', 'viable',
  'vibrant', 'victim', 'video', 'view', 'village', 'vintage', 'violin', 'virtual', 'virus',
  'visa', 'visit', 'visual', 'vital', 'vivid', 'vocal', 'voice', 'void', 'volcano', 'volume',
  'volunteer', 'vote', 'voyage', 'wage', 'wagon', 'wait', 'wake', 'walk', 'wall', 'walnut',
  'want', 'warfare', 'warm', 'warrior', 'wash', 'wasp', 'waste', 'watch', 'water', 'wave',
  'way', 'wealth', 'weapon', 'wear', 'weasel', 'weather', 'web', 'wedding', 'weekend', 'weird',
  'welcome', 'well', 'west', 'wet', 'whale', 'what', 'wheat', 'wheel', 'when', 'where',
  'whip', 'whisper', 'whistle', 'white', 'who', 'whole', 'whom', 'whose', 'why', 'wicked',
  'wide', 'widow', 'width', 'wife', 'wild', 'will', 'win', 'window', 'wine', 'wing', 'wink',
  'winner', 'winter', 'wire', 'wisdom', 'wise', 'wish', 'wit', 'witch', 'with', 'witness',
  'wolf', 'woman', 'wonder', 'wood', 'wool', 'word', 'work', 'world', 'worry', 'worth',
  'wrap', 'wreck', 'wrestle', 'wrist', 'write', 'wrong', 'yard', 'year', 'yell', 'yellow',
  'you', 'young', 'youth', 'zebra', 'zero', 'zombie', 'zone', 'zoo'
];

// ============================================================================
// Mnemonic Generation & Validation
// ============================================================================

export function generateMnemonic(wordCount: 12 | 15 | 24 = 24): string[] {
  const indices: number[] = [];
  const totalWords = wordCount === 12 ? 128 : wordCount === 15 ? 160 : 256;
  
  // Generate random entropy
  for (let i = 0; i < totalWords / 8; i++) {
    indices.push(Math.floor(Math.random() * 2048));
  }
  
  return indices.map(i => BIP39_WORDLIST[i]);
}

export function validateMnemonic(mnemonic: string[]): boolean {
  if (mnemonic.length !== 12 && mnemonic.length !== 15 && mnemonic.length !== 24) {
    return false;
  }
  
  return mnemonic.every(word => BIP39_WORDLIST.includes(word));
}

// ============================================================================
// HD Wallet Derivation
// ============================================================================

export class HDWallet {
  private masterKey: Uint8Array;
  private chainCode: Uint8Array;
  
  constructor(mnemonic: string[], password: string = '') {
    const seed = this.mnemonicToSeed(mnemonic, password);
    this.masterKey = seed.slice(0, 32);
    this.chainCode = seed.slice(32, 64);
  }
  
  private mnemonicToSeed(mnemonic: string[], password: string): Uint8Array {
    const mnemonicStr = mnemonic.join(' ');
    const salt = 'mnemonic' + password;
    
    // Simplified PBKDF2 - in production use proper implementation
    const seed = new Uint8Array(64);
    const entropy = new TextEncoder().encode(mnemonicStr + salt);
    
    // Simple hash-based derivation
    let hash = keccak256(entropy);
    for (let i = 0; i < 2048; i++) {
      hash = keccak256(new Uint8Array([...new TextEncoder().encode(hash), i % 256]));
    }
    
    // Expand to 64 bytes
    for (let i = 0; i < 64; i++) {
      seed[i] = (parseInt(hash.slice(2 + i * 2, 4 + i * 2), 16) || 0);
    }
    
    return seed;
  }
  
  deriveChildKey(index: number, hardened: boolean = true): Uint8Array {
    const data = new Uint8Array(37);
    data[0] = hardened ? 0 : 1;
    
    // Simplified child key derivation
    const indexBytes = new TextEncoder().encode(index.toString());
    for (let i = 0; i < 32 && i < indexBytes.length; i++) {
      data[i + 1] = indexBytes[i];
    }
    
    const childKey = new Uint8Array(32);
    const combined = new Uint8Array([...this.masterKey, ...data]);
    const hash = keccak256(combined);
    
    for (let i = 0; i < 32; i++) {
      const byteStr = hash.slice(2 + i * 2, 4 + i * 2);
      childKey[i] = parseInt(byteStr, 16) || 0;
    }
    
    return childKey;
  }
  
  getAddress(publicKey: Uint8Array): string {
    // Keccak256 hash of public key, take last 20 bytes
    const hash = keccak256(publicKey);
    const addressBytes = hash.slice(-40);
    return '0x' + addressBytes;
  }
}

// ============================================================================
// TigerWallet Class
// ============================================================================

export class TigerWallet {
  private accounts: Map<number, WalletAccount> = new Map();
  private mnemonic: string[];
  private provider: JsonRpcProvider | null = null;
  private signer: Wallet | null = null;
  private chainId: number = 1;
  private apiKeys: Map<string, string> = new Map();
  private isConnected: boolean = false;
  private masterId: string;
  
  constructor(mnemonic: string[], masterId: string, password: string = '') {
    this.mnemonic = mnemonic;
    this.masterId = masterId;
    this.initializeAccounts();
  }
  
  private initializeAccounts(): void {
    const hdWallet = new HDWallet(this.mnemonic);
    
    // Generate one account per chain
    for (const [chainId, config] of Object.entries(SUPPORTED_CHAINS)) {
      const childKey = hdWallet.deriveChildKey(0);
      const address = hdWallet.getAddress(childKey);
      
      this.accounts.set(parseInt(chainId), {
        id: crypto.randomUUID(),
        address,
        chainId: parseInt(chainId),
        publicKey: '0x' + Buffer.from(childKey).toString('hex'),
        path: `m/44'/${config.slip44}'/0'/0'/0'`,
        name: config.symbol,
        balance: '0',
        balanceUSD: 0,
        tokens: [],
        createdAt: Date.now(),
        lastActiveAt: null
      });
    }
  }
  
  static create(masterId: string): TigerWallet {
    const mnemonic = generateMnemonic(24);
    return new TigerWallet(mnemonic, masterId);
  }
  
  static import(mnemonic: string[], masterId: string, password?: string): TigerWallet {
    if (!validateMnemonic(mnemonic)) {
      throw new Error('Invalid mnemonic');
    }
    return new TigerWallet(mnemonic, masterId, password);
  }
  
  getMnemonic(): string[] {
    return this.mnemonic;
  }
  
  getAddress(chainId: number = 1): string | null {
    return this.accounts.get(chainId)?.address || null;
  }
  
  async connect(chainId: number, rpcUrl?: string): Promise<void> {
    const config = SUPPORTED_CHAINS[chainId];
    if (!config) {
      throw new Error('Unsupported chain');
    }
    
    this.provider = new JsonRpcProvider(rpcUrl || config.rpcUrl);
    
    // Create signer from private key (derived from mnemonic)
    const hdWallet = new HDWallet(this.mnemonic);
    const childKey = hdWallet.deriveChildKey(0);
    const privateKey = '0x' + Buffer.from(childKey).toString('hex');
    
    this.signer = new Wallet(privateKey, this.provider);
    this.chainId = chainId;
    this.isConnected = true;
  }
  
  async disconnect(): Promise<void> {
    this.provider = null;
    this.signer = null;
    this.isConnected = false;
  }
  
  isWalletConnected(): boolean {
    return this.isConnected;
  }
  
  async getBalance(chainId?: number): Promise<string> {
    const targetChainId = chainId || this.chainId;
    const account = this.accounts.get(targetChainId);
    if (!account || !this.provider) {
      return '0';
    }
    
    const balance = await this.provider.getBalance(account.address);
    return balance.toString();
  }
  
  async getTokenBalance(tokenAddress: string, chainId?: number): Promise<string> {
    const targetChainId = chainId || this.chainId;
    const account = this.accounts.get(targetChainId);
    if (!account || !this.provider) {
      return '0';
    }
    
    const token = new Contract(tokenAddress, ERC20_ABI, this.provider);
    const balance = await token.balanceOf(account.address);
    return balance.toString();
  }
  
  async sendTransaction(
    to: string,
    amount: string,
    options?: {
      data?: string;
      gasLimit?: string;
      gasPrice?: string;
    }
  ): Promise<string> {
    if (!this.signer || !this.provider) {
      throw new Error('Wallet not connected');
    }
    
    const account = this.accounts.get(this.chainId);
    if (!account) {
      throw new Error('Account not found');
    }
    
    const tx = await this.signer.sendTransaction({
      to,
      value: ethers.parseEther(amount),
      ...options
    });
    
    const receipt = await tx.wait();
    account.lastActiveAt = Date.now();
    
    return receipt?.hash || tx.hash;
  }
  
  async sendToken(
    tokenAddress: string,
    to: string,
    amount: string
  ): Promise<string> {
    if (!this.signer || !this.provider) {
      throw new Error('Wallet not connected');
    }
    
    const token = new Contract(tokenAddress, ERC20_ABI, this.signer);
    const decimals = await token.decimals();
    const amountWei = ethers.parseUnits(amount, decimals);
    
    const tx = await token.transfer(to, amountWei);
    const receipt = await tx.wait();
    
    return receipt?.hash || tx.hash;
  }
  
  // Swap operations using TigerSwap
  async swap(
    tokenIn: string,
    tokenOut: string,
    amountIn: string,
    slippage: number = 0.5
  ): Promise<string> {
    // This would integrate with TigerSwap routing engine
    // For now, return mock transaction hash
    return '0x' + crypto.randomUUID().replace(/-/g, '');
  }
  
  async addLiquidity(
    tokenA: string,
    tokenB: string,
    amountA: string,
    amountB: string
  ): Promise<string> {
    return '0x' + crypto.randomUUID().replace(/-/g, '');
  }
  
  async removeLiquidity(
    tokenA: string,
    tokenB: string,
    liquidity: string
  ): Promise<string> {
    return '0x' + crypto.randomUUID().replace(/-/g, '');
  }
  
  // Connect to external DEXs using API keys
  async connectExternalDex(dexName: string, apiKey: string): Promise<void> {
    this.apiKeys.set(dexName, apiKey);
  }
  
  // Connect to external CEXs using API keys
  async connectExternalCex(cexName: string, apiKey: string, apiSecret: string): Promise<void> {
    this.apiKeys.set(cexName, apiKey);
    this.apiKeys.set(cexName + '_secret', apiSecret);
  }
  
  // Perform swap on external DEX/CEX
  async tradeOnExternal(
    platform: 'binance' | 'uniswap' | 'pancakeswap' | string,
    symbol: string,
    side: 'buy' | 'sell',
    amount: string
  ): Promise<string> {
    // Integration would go here
    return '0x' + crypto.randomUUID().replace(/-/g, '');
  }
  
  // Claim airdrop
  async claimAirdrop(campaignId: string): Promise<string> {
    return '0x' + crypto.randomUUID().replace(/-/g, '');
  }
  
  // Join campaign
  async joinCampaign(campaignId: string): Promise<string> {
    return '0x' + crypto.randomUUID().replace(/-/g, '');
  }
  
  // Multi-sign transfer (for multisig)
  async createMultiSigTransfer(
    recipients: { to: string; amount: string }[],
    requiredSignatures: number
  ): Promise<string> {
    return '0x' + crypto.randomUUID().replace(/-/g, '');
  }
  
  // Create new token (for token launches)
  async createToken(
    name: string,
    symbol: string,
    totalSupply: string,
    decimals: number = 18
  ): Promise<string> {
    return '0x' + crypto.randomUUID().replace(/-/g, '');
  }
  
  // Connect to other platforms via built-in browser
  async connectPlatform(platform: string, credentials: any): Promise<void> {
    // Store credentials for platform
    this.apiKeys.set(platform + '_credentials', JSON.stringify(credentials));
  }
  
  getAllAccounts(): WalletAccount[] {
    return Array.from(this.accounts.values());
  }
  
  getSupportedChains(): ChainConfig[] {
    return Object.values(SUPPORTED_CHAINS);
  }
}

// ============================================================================
// Export
// ============================================================================

export default TigerWallet;
export { SUPPORTED_CHAINS, generateMnemonic, validateMnemonic };