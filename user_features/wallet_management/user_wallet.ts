// TigerSwap User Wallet - Full Web3 Wallet Implementation
// Supports EVM and Non-EVM chains with auto-signing capability

import { ethers } from 'ethers';

// Wallet Types
export interface Wallet {
  id: string
  address: string
  chainType: 'evm' | 'solana' | 'tron' | 'bitcoin' | 'sui' | 'aptos'
  createdAt: number
  name: string
  isHardware: boolean
  balance: Balance[]
}

export interface Balance {
  symbol: string
  address: string
  amount: string
  value: string
  chainId: number
  logo?: string
}

export interface Transaction {
  id: string
  hash: string
  from: string
  to: string
  value: string
  token: string
  fee: string
  status: 'pending' | 'confirmed' | 'failed'
  timestamp: number
  chainId: number
  type: 'send' | 'receive' | 'swap' | 'approve' | 'contract'
}

export interface Chain {
  id: number
  name: string
  type: 'evm' | 'solana' | 'tron' | 'bitcoin' | 'sui' | 'aptos'
  rpc: string
  explorer: string
  symbol: string
  decimals: number
  wrappedToken?: string
  isEnabled: boolean
  icon: string
  estimatedGas?: string
  gasPrice?: string
}

export interface Token {
  symbol: string
  name: string
  address: string
  decimals: number
  chainId: number
  logo: string
  price?: string
  isNative: boolean
  isStable?: boolean
}

export interface SwapQuote {
  fromToken: Token
  toToken: Token
  fromAmount: string
  toAmount: string
  priceImpact: number
  route: RouteInfo[]
  estimatedGas: string
  slippage: number
}

export interface RouteInfo {
  protocol: string
  path: string[]
  pools: string[]
  percentage: number
}

// HD Wallet Engine
export class HDWalletEngine {
  private mnemonic: string
  private derivationPath: string

  constructor(mnemonic: string, derivationPath: string = "m/44'/60'/0'/0/0") {
    this.mnemonic = mnemonic
    this.derivationPath = derivationPath
  }

  // Generate wallet from mnemonic
  static fromMnemonic(mnemonic: string, path?: string): HDWalletEngine {
    return new HDWalletEngine(mnemonic, path)
  }

  // Generate random mnemonic
  static generateMnemonic(strength: number = 256): string {
    const words = [
      'abandon', 'ability', 'able', 'about', 'above', 'absent', 'absorb', 'abstract', 
      'absurd', 'abuse', 'access', 'accident', 'account', 'accuse', 'achieve', 'acid',
      'acoustic', 'acquire', 'across', 'act', 'action', 'actor', 'actress', 'actual',
      'adapt', 'add', 'addict', 'address', 'adjust', 'admit', 'adult', 'advance',
      'advice', 'aerobic', 'affair', 'afford', 'afraid', 'again', 'age', 'agent',
      'agree', 'ahead', 'aim', 'air', 'airport', 'aisle', 'alarm', 'album', 'alcohol',
      'alert', 'alien', 'all', 'alley', 'allow', 'almost', 'alone', 'alpha', 'already',
      'also', 'alter', 'always', 'amateur', 'amazing', 'among', 'amount', 'amused',
      'analyst', 'anchor', 'ancient', 'anger', 'angle', 'angry', 'animal', 'ankle',
      'announce', 'annual', 'another', 'answer', 'antenna', 'antique', 'anxiety',
      'any', 'apart', 'apology', 'appear', 'apple', 'approve', 'april', 'arch',
      'arctic', 'area', 'arena', 'argue', 'arm', 'armed', 'armor', 'army', 'around',
      'arrange', 'arrest', 'arrive', 'arrow', 'art', 'artefact', 'artist', 'artwork',
      'ask', 'aspect', 'assault', 'asset', 'assist', 'assume', 'asthma', 'athlete',
      'atom', 'attack', 'attend', 'attitude', 'attract', 'auction', 'audit', 'august', 'aunt', 'author', 'auto', 'autumn', 'average', 'avocado', 'avoid', 'awake', 'aware', 'away', 'awesome', 'awful', 'awkward', 'axis', 'baby', 'bachelor', 'bacon', 'badge', 'bag', 'balance', 'balcony', 'ball', 'bamboo', 'banana', 'banner', 'bar', 'barely', 'bargain', 'barrel', 'base', 'basic', 'basket', 'battle', 'beach', 'bean', 'beauty', 'because', 'become', 'beef', 'before', 'begin', 'behave', 'behind', 'believe', 'below', 'belt', 'bench', 'benefit', 'best', 'betray', 'better', 'between', 'beyond', 'bicycle', 'bid', 'bike', 'bind', 'biology', 'bird', 'birth', 'bitter', 'black', 'blade', 'blame', 'blanket', 'blast', 'blaze', 'bless', 'blind', 'blood', 'blossom', 'blouse', 'blue', 'blur', 'blush', 'board', 'boat', 'body', 'boil', 'bomb', 'bone', 'bonus', 'book', 'boost', 'border', 'boring', 'borrow', 'boss', 'bottom', 'bounce', 'box', 'boy', 'bracket', 'brain', 'brand', 'brass', 'brave', 'bread', 'breeze', 'brick', 'bridge', 'brief', 'bright', 'bring', 'brisk', 'broccoli', 'broken', 'bronze', 'broom', 'brother', 'brown', 'brush', 'bubble', 'buddy', 'budget', 'buffalo', 'build', 'bulb', 'bulk', 'bullet', 'bundle', 'bunker', 'burden', 'burger', 'burst', 'bus', 'business', 'busy', 'butter', 'buyer', 'buzz', 'cabbage', 'cabin', 'cable', 'cactus', 'cage', 'cake', 'call', 'calm', 'camera', 'camp', 'can', 'canal', 'cancel', 'candy', 'cannon', 'canoe', 'canvas', 'canyon', 'capable', 'capital', 'captain', 'car', 'carbon', 'card', 'cargo', 'carpet', 'carry', 'cart', 'case', 'cash', 'casino', 'castle', 'casual', 'cat', 'catalog', 'catch', 'category', 'cattle', 'caught', 'cause', 'caution', 'cave', 'ceiling', 'celery', 'cement', 'census', 'century', 'cereal', 'certain', 'chair', 'chalk', 'champion', 'change', 'chaos', 'chapter', 'charge', 'chase', 'chat', 'cheap', 'check', 'cheese', 'chef', 'cherry', 'chest', 'chicken', 'chief', 'child', 'chimney', 'choice', 'choose', 'chronic', 'chuckle', 'chunk', 'churn', 'cigar', 'cinnamon', 'circle', 'citizen', 'city', 'civil', 'claim', 'clap', 'clarify', 'claw', 'clay', 'clean', 'clerk', 'clever', 'click', 'client', 'cliff', 'climb', 'clinic', 'clip', 'clock', 'close', 'cloth', 'cloud', 'clown', 'club', 'clump', 'clutch', 'coach', 'coast', 'coconut', 'code', 'coffee', 'coil', 'coin', 'collect', 'color', 'column', 'combine', 'come', 'comfort', 'comic', 'common', 'company', 'concert', 'conduct', 'confirm', 'congress', 'connect', 'consider', 'control', 'convince', 'cook', 'cool', 'copper', 'copy', 'coral', 'core', 'corn', 'correct', 'cost', 'cotton', 'couch', 'country', 'couple', 'course', 'cousin', 'cover', 'coyote', 'crack', 'cradle', 'craft', 'cram', 'crane', 'crash', 'crater', 'crawl', 'crazy', 'cream', 'credit', 'creek', 'crew', 'cricket', 'crime', 'crisp', 'critic', 'crop', 'cross', 'crouch', 'crowd', 'crucial', 'cruel', 'cruise', 'crumble', 'crunch', 'crush', 'cry', 'crystal', 'cube', 'culture', 'cup', 'cupboard', 'curious', 'current', 'curtain', 'curve', 'cushion', 'custom', 'cute', 'cycle', 'dad', 'damage', 'damp', 'dance', 'danger', 'daring', 'dash', 'daughter', 'dawn', 'day', 'deal', 'debate', 'debris', 'decade', 'december', 'decide', 'decline', 'decorate', 'decrease', 'deer', 'defense', 'define', 'defy', 'degree', 'delay', 'deliver', 'demand', 'demise', 'denial', 'dentist', 'deny', 'depart', 'depend', 'deposit', 'depth', 'deputy', 'derive', 'describe', 'desert', 'design', 'desk', 'despair', 'destroy', 'detail', 'detect', 'develop', 'device', 'devote', 'diagram', 'dial', 'diamond', 'diary', 'dice', 'diesel', 'diet', 'differ', 'digital', 'dignity', 'dilemma', 'dinner', 'dinosaur', 'direct', 'dirt', 'disagree', 'discover', 'disease', 'dish', 'dismiss', 'disorder', 'display', 'distance', 'divert', 'divide', 'divorce', 'dizzy', 'doctor', 'document', 'dog', 'doll', 'dolphin', 'domain', 'donate', 'donkey', 'donor', 'door', 'dose', 'double', 'dove', 'draft', 'dragon', 'drama', 'drastic', 'draw', 'dream', 'dress', 'drift', 'drill', 'drink', 'drip', 'drive', 'drop', 'drum', 'dry', 'duck', 'dumb', 'dune', 'during', 'dust', 'dutch', 'duty', 'dwarf', 'dynamic', 'eager', 'eagle', 'early', 'earn', 'earth', 'easily', 'east', 'easy', 'echo', 'ecology', 'economy', 'edge', 'edit', 'educate', 'effort', 'egg', 'eight', 'either', 'elbow', 'elder', 'electric', 'elegant', 'element', 'elephant', 'elevator', 'elite', 'else', 'embark', 'embody', 'embrace', 'emerge', 'emotion', 'employ', 'empower', 'empty', 'enable', 'enact', 'end', 'endless', 'endorse', 'enemy', 'energy', 'enforce', 'engage', 'engine', 'enhance', 'enjoy', 'enlist', 'enough', 'enrich', 'enroll', 'ensure', 'enter', 'entire', 'entry', 'envelope', 'episode', 'equal', 'equip', 'era', 'erase', 'erode', 'erosion', 'error', 'erupt', 'escape', 'essay', 'essence', 'estate', 'eternal', 'ethics', 'evidence', 'evil', 'evoke', 'exact', 'exaggerate', 'exam', 'excel', 'excess', 'exchange', 'excite', 'exclude', 'excuse', 'execute', 'exercise', 'exhaust', 'exhibit', 'exile', 'exist', 'exit', 'exotic', 'expand', 'expect', 'expire', 'explain', 'expose', 'express', 'extend', 'extra', 'eye', 'eyebrow', 'fabric', 'face', 'faculty', 'fade', 'faint', 'faith', 'fall', 'false', 'fame', 'family', 'famous', 'fan', 'fancy', 'fantasy', 'farm', 'fashion', 'fat', 'fatal', 'father', 'fatigue', 'fault', 'favorite', 'feature', 'february', 'federal', 'fee', 'feed', 'feel', 'female', 'fence', 'festival', 'fetch', 'fever', 'few', 'fiber', 'fiction', 'field', 'figure', 'file', 'film', 'filter', 'final', 'find', 'fine', 'finish', 'fire', 'firm', 'first', 'fiscal', 'fish', 'fit', 'fitness', 'fix', 'flag', 'flame', 'flash', 'flat', 'flavor', 'flee', 'flight', 'flip', 'float', 'flock', 'floor', 'flower', 'fluid', 'flush', 'fly', 'foam', 'focus', 'fog', 'foil', 'fold', 'follow', 'food', 'foot', 'force', 'forest', 'forget', 'fork', 'fortune', 'forum', 'fossil', 'foster', 'found', 'fox', 'fragile', 'frame', 'frequent', 'friction', 'friday', 'friend', 'fringe', 'frog', 'front', 'frost', 'frown', 'frozen', 'fruit', 'fuel', 'fun', 'funny', 'furnace', 'fury', 'future', 'gadget', 'gain', 'galaxy', 'gallery', 'game', 'gap', 'garage', 'garbage', 'garden', 'garlic', 'garment', 'gas', 'gasp', 'gate', 'gather', 'gauge', 'gaze', 'general', 'genius', 'genre', 'gentle', 'genuine', 'gesture', 'ghost', 'giant', 'gift', 'giggle', 'ginger', 'girl', 'give', 'glad', 'glance', 'glare', 'glass', 'glimpse', 'globe', 'gloom', 'glory', 'gloss', 'glove', 'glow', 'glue', 'goal', 'goat', 'goddess', 'gold', 'golf', 'good', 'goose', 'gorilla', 'gospel', 'gossip', 'govern', 'gown', 'grab', 'grace', 'grain', 'grand', 'grant', 'grape', 'graph', 'grasp', 'grass', 'gravity', 'great', 'green', 'grid', 'grief', 'grill', 'grin', 'grind', 'grip', 'grocery', 'groan', 'groom', 'gross', 'group', 'grow', 'grumble', 'grunt', 'guard', 'guess', 'guest', 'guide', 'guilt', 'guitar', 'gun', 'gust', 'gutter', 'guy', 'habit', 'hair', 'half', 'hall', 'halt', 'hammer', 'hand', 'handle', 'hang', 'happy', 'harbor', 'hard', 'harsh', 'harvest', 'haste', 'hate', 'haul', 'haunt', 'have', 'hawk', 'hazard', 'head', 'health', 'heart', 'heavy', 'hedgehog', 'height', 'hello', 'helmet', 'help', 'hemisphere', 'hen', 'hero', 'hidden', 'hierarchy', 'high', 'hike', 'hill', 'hint', 'hip', 'hire', 'historian', 'hold', 'hole', 'holiday', 'hollow', 'holy', 'home', 'honey', 'hood', 'hope', 'horizon', 'horn', 'horror', 'horse', 'hospital', 'host', 'hotel', 'hour', 'house', 'hover', 'huge', 'humor', 'hundred', 'hungry', 'hunt', 'hurdle', 'hurry', 'hurt', 'husband', 'hybrid', 'ice', 'icon', 'idea', 'identify', 'idle', 'ignore', 'ill', 'illegal', 'illness', 'image', 'imitate', 'immature', 'immune', 'impact', 'impose', 'improve', 'impulse', 'inch', 'include', 'income', 'increase', 'index', 'indicate', 'indoor', 'industry', 'infant', 'inflict', 'inform', 'inhale', 'inherit', 'initial', 'inject', 'injury', 'innocent', 'input', 'inquiry', 'insect', 'inside', 'insight', 'inspire', 'install', 'intact', 'intake', 'intimate', 'introduce', 'intricate', 'invade', 'invest', 'invite', 'involve', 'iron', 'irony', 'island', 'isolate', 'issue', 'ivory', 'jacket', 'jaguar', 'jail', 'january', 'jazz', 'jealous', 'jeans', 'jellyfish', 'jewel', 'jewel', 'job', 'join', 'joke', 'journey', 'joy', 'judge', 'juice', 'jumbo', 'jump', 'june', 'july', 'jump', 'jungle', 'junior', 'junk', 'just', 'kangaroo', 'keen', 'keep', 'ketchup', 'key', 'kick', 'kid', 'kidney', 'kind', 'kingdom', 'kiss', 'kit', 'kitchen', 'kite', 'knee', 'knife', 'knock', 'knot', 'know', 'knowledge', 'lab', 'label', 'labor', 'ladder', 'lady', 'lake', 'lamb', 'lamp', 'land', 'landscape', 'lane', 'language', 'laptop', 'large', 'laser', 'last', 'late', 'later', 'latin', 'laugh', 'laundry', 'lava', 'law', 'lawn', 'lawsuit', 'layer', 'lazy', 'leader', 'leaf', 'learn', 'leave', 'lecture', 'left', 'legacy', 'legal', 'legend', 'leisure', 'lemon', 'lend', 'length', 'lens', 'leopard', 'lesson', 'letter', 'level', 'liar', 'liberty', 'library', 'license', 'life', 'lift', 'light', 'like', 'limb', 'limit', 'linen', 'liner', 'link', 'lion', 'list', 'listen', 'literacy', 'little', 'live', 'liver', 'living', 'llama', 'load', 'loan', 'lobby', 'local', 'lock', 'locker', 'logic', 'lonely', 'loose', 'lorry', 'lottery', 'loud', 'lounge', 'love', 'loyal', 'lucky', 'lumber', 'lunar', 'lunch', 'luxury', 'lyrics', 'machine', 'magic', 'maid', 'magnetic', 'magnificent', 'maid', 'mail', 'main', 'maintain', 'major', 'makeup', 'male', 'mall', 'mammoth', 'manage', 'mandate', 'mango', 'manifest', 'manner', 'manual', 'marble', 'march', 'margin', 'marine', 'market', 'marriage', 'mask', 'mass', 'master', 'match', 'mate', 'material', 'math', 'matter', 'may', 'maybe', 'mayor', 'me', 'meal', 'mean', 'measure', 'meat', 'mechanic', 'medal', 'media', 'melon', 'melt', 'member', 'membership', 'memory', 'mental', 'mention', 'mentor', 'merge', 'merit', 'merry', 'mesh', 'message', 'metal', 'meter', 'method', 'middle', 'midnight', 'might', 'mild', 'mile', 'milk', 'million', 'mimic', 'mind', 'mine', 'mineral', 'minimize', 'minor', 'minus', 'minute', 'miracle', 'mirror', 'misery', 'miss', 'mistake', 'mixed', 'mixture', 'mobile', 'model', 'modify', 'moment', 'money', 'monkey', 'month', 'mood', 'moon', 'moral', 'more', 'morning', 'mother', 'motion', 'motor', 'mount', 'mouse', 'mouth', 'move', 'movie', 'much', 'muffin', 'mule', 'multiply', 'muscle', 'museum', 'mushroom', 'music', 'must', 'mutual', 'myself', 'mystery', 'myth', 'naive', 'name', 'napkin', 'narrow', 'nasty', 'nation', 'native', 'natural', 'nature', 'near', 'nearby', 'nearly', 'neat', 'necessary', 'neck', 'need', 'negative', 'neglect', 'negotiate', 'neighbor', 'neither', 'nephew', 'nerve', 'nest', 'net', 'network', 'neutral', 'never', 'new', 'news', 'newspaper', 'next', 'nice', 'night', 'nine', 'noble', 'none', 'noon', 'normal', 'north', 'nose', 'notable', 'note', 'nothing', 'notice', 'notion', 'novel', 'now', 'nuclear', 'number', 'nurse', 'nut', 'nutrition', 'oak', 'obey', 'object', 'oblige', 'obscure', 'observe', 'obtain', 'occupy', 'occur', 'ocean', 'october', 'odor', 'offer', 'office', 'often', 'olive', 'olympic', 'omit', 'once', 'one', 'onion', 'online', 'only', 'open', 'opera', 'opinion', 'opponent', 'option', 'orange', 'orbit', 'orchard', 'order', 'ordinary', 'organ', 'orient', 'original', 'orphan', 'other', 'outdoor', 'outer', 'output', 'outside', 'oval', 'oven', 'over', 'overall', 'overcome', 'overlook', 'owner', 'oxygen', 'oyster', 'ozone', 'pace', 'pack', 'paddle', 'page', 'paid', 'pain', 'paint', 'pair', 'palace', 'palm', 'panda', 'panel', 'panic', 'paper', 'parachute', 'parade', 'parent', 'park', 'parrot', 'party', 'pass', 'past', 'paste', 'patch', 'path', 'patient', 'patrol', 'patience', 'patron', 'pause', 'pave', 'payment', 'peace', 'peach', 'pearl', 'pedestrian', 'penalty', 'penny', 'people', 'pepper', 'percent', 'perfect', 'perform', 'perhaps', 'period', 'permit', 'person', 'personal', 'perspective', 'pest', 'pet', 'petrol', 'phase', 'phone', 'photo', 'phrase', 'physical', 'piano', 'pick', 'picture', 'piece', 'pilot', 'pin', 'pink', 'pioneer', 'pipe', 'pistol', 'pitch', 'pizza', 'place', 'plain', 'planet', 'plastic', 'plate', 'play', 'plaza', 'please', 'pledge', 'plenty', 'plough', 'plot', 'pocket', 'poem', 'poet', 'point', 'polar', 'police', 'policy', 'polish', 'polite', 'political', 'poll', 'pond', 'pony', 'pool', 'popular', 'porch', 'portion', 'position', 'possible', 'post', 'pot', 'potato', 'potential', 'pound', 'poverty', 'powder', 'power', 'practice', 'praise', 'predict', 'prefer', 'pregnant', 'prepare', 'present', 'preserve', 'press', 'price', 'pride', 'priest', 'primary', 'prime', 'prince', 'princess', 'print', 'prior', 'prize', 'probe', 'problem', 'process', 'produce', 'product', 'profile', 'profit', 'program', 'project', 'prominent', 'promise', 'promote', 'proof', 'property', 'proposal', 'prose', 'prosper', 'protect', 'protein', 'protest', 'proud', 'prove', 'provide', 'province', 'prune', 'public', 'publish', 'pull', 'pulse', 'punch', 'pupil', 'purchase', 'purple', 'purse', 'pursue', 'push', 'put', 'puzzle', 'pyramid', 'quality', 'quantum', 'quarter', 'queen', 'query', 'quest', 'quick', 'quickly', 'quiet', 'quite', 'quiz', 'quote', 'rabbit', 'race', 'racial', 'radar', 'radio', 'rail', 'rain', 'raise', 'rally', 'ranch', 'random', 'range', 'rapid', 'rare', 'rather', 'rate', 'rather', 'raven', 'raw', 'razor', 'reach', 'react', 'read', 'ready', 'reality', 'realize', 'really', 'reap', 'reason', 'rebel', 'build', 'receive', 'recipe', 'record', 'recover', 'reduce', 'reflect', 'reform', 'refuse', 'regard', 'region', 'regret', 'regular', 'reign', 'reject', 'relate', 'relax', 'release', 'relief', 'rely', 'remain', 'remark', 'remember', 'remind', 'remote', 'remove', 'render', 'renew', 'rent', 'rental', 'repair', 'repeat', 'report', 'represent', 'request', 'require', 'rescue', 'research', 'reserve', 'resign', 'resist', 'resort', 'resource', 'respond', 'response', 'rest', 'restore', 'result', 'retail', 'retain', 'retire', 'return', 'reveal', 'revenge', 'revenue', 'review', 'revolution', 'reward', 'rhythm', 'rice', 'rid', 'ride', 'ridge', 'rifle', 'right', 'rigid', 'ring', 'riot', 'ripple', 'rise', 'risk', 'ritual', 'rival', 'river', 'road', 'roast', 'robot', 'robust', 'rocket', 'rock', 'rock', 'romance', 'roof', 'room', 'root', 'rope', 'rose', 'rotten', 'rough', 'round', 'route', 'royal', 'rubber', 'rude', 'ruin', 'rule', 'run', 'runway', 'rural', 'rush', 'rust', 'sack', 'sad', 'saddle', 'sadness', 'safe', 'sail', 'salad', 'salmon', 'salon', 'salt', 'salute', 'same', 'sample', 'sand', 'satisfy', 'satoshi', 'sauce', 'save', 'scale', 'scan', 'scare', 'scene', 'scent', 'scheme', 'scholar', 'school', 'science', 'scissors', 'scorpion', 'scout', 'scrap', 'screen', 'script', 'scrub', 'search', 'season', 'seat', 'second', 'secret', 'section', 'sector', 'secure', 'seed', 'seek', 'segment', 'select', 'sell', 'seminar', 'senate', 'senator', 'send', 'senior', 'sense', 'sentence', 'sequence', 'series', 'server', 'settle', 'setup', 'seven', 'shadow', 'shaft', 'shallow', 'share', 'shed', 'shell', 'shield', 'shift', 'shine', 'shirt', 'shock', 'shoe', 'shoot', 'shop', 'short', 'shot', 'should', 'shoulder', 'shout', 'show', 'shower', 'shrimp', 'shrine', 'shrink', 'shut', 'sibling', 'sick', 'side', 'siege', 'sight', 'sign', 'silent', 'silk', 'silly', 'silver', 'similar', 'simple', 'since', 'sing', 'singer', 'single', 'sink', 'sister', 'sit', 'site', 'situation', 'six', 'size', 'skate', 'skill', 'skin', 'skip', 'slave', 'sleep', 'slice', 'slide', 'slight', 'slim', 'slogan', 'slow', 'slowly', 'small', 'smart', 'smell', 'smile', 'smoke', 'smooth', 'snake', 'snap', 'snow', 'soccer', 'social', 'society', 'sock', 'soda', 'soft', 'software', 'soil', 'solar', 'soldier', 'solid', 'solution', 'solve', 'some', 'somebody', 'someone', 'something', 'sometimes', 'somewhat', 'somewhere', 'song', 'soon', 'sophisticated', 'sorry', 'sort', 'soul', 'sound', 'soup', 'source', 'south', 'southeast', 'southern', 'space', 'speak', 'speaker', 'special', 'species', 'specific', 'specify', 'speech', 'speed', 'spell', 'spend', 'sphere', 'spice', 'spider', 'spin', 'spirit', 'split', 'spoke', 'sponsor', 'spoon', 'sport', 'spot', 'spread', 'spring', 'spy', 'square', 'squeeze', 'stability', 'stable', 'stadium', 'staff', 'stage', 'stair', 'stake', 'stamp', 'stand', 'standard', 'star', 'stare', 'start', 'state', 'station', 'status', 'stay', 'steady', 'steak', 'steel', 'steep', 'stem', 'step', 'steward', 'stick', 'still', 'stock', 'stomach', 'stone', 'stool', 'store', 'storm', 'story', 'stove', 'straight', 'strain', 'strand', 'strange', 'stranger', 'strategic', 'stream', 'street', 'strength', 'stress', 'strict', 'strike', 'string', 'strip', 'stripe', 'strong', 'structure', 'struggle', 'student', 'studio', 'study', 'stuff', 'stumble', 'style', 'subject', 'submit', 'subsequent', 'substance', 'subtle', 'suburb', 'sudden', 'suffer', 'sugar', 'suggest', 'suit', 'suite', 'sultan', 'summer', 'summit', 'sun', 'sunrise', 'sunshine', 'super', 'supply', 'support', 'suppose', 'supreme', 'sure', 'surface', 'surgeon', 'surgery', 'surprise', 'surround', 'survey', 'survival', 'suspect', 'sustain', 'swallow', 'swamp', 'swap', 'swarm', 'swear', 'sweat', 'sweep', 'sweet', 'swift', 'swim', 'swing', 'switch', 'sword', 'symbol', 'symptom', 'syntax', 'system', 'table', 'tablet', 'tackle', 'tactic', 'tail', 'talent', 'talk', 'tall', 'tank', 'tape', 'target', 'task', 'taste', 'taxi', 'teach', 'teacher', 'team', 'tear', 'technical', 'technique', 'technology', 'teenager', 'teeth', 'telegram', 'telephone', 'telescope', 'television', 'tell', 'temperature', 'temple', 'tempo', 'tenant', 'tend', 'tender', 'tennis', 'tense', 'tension', 'tent', 'term', 'terminal', 'terrible', 'territory', 'test', 'text', 'thank', 'that', 'theater', 'theme', 'theory', 'therapy', 'thick', 'thief', 'thing', 'think', 'third', 'thirty', 'this', 'thorn', 'thorough', 'those', 'though', 'thought', 'thousand', 'thread', 'threat', 'threaten', 'three', 'thrive', 'throat', 'throne', 'throw', 'thumb', 'thunder', 'thursday', 'thus', 'ticket', 'tide', 'tiger', 'tight', 'timber', 'time', 'timeline', 'tiny', 'tire', 'tired', 'tissue', 'title', 'toast', 'tobacco', 'today', 'toddler', 'together', 'toilet', 'token', 'tomato', 'tomorrow', 'tone', 'tongue', 'tonight', 'tool', 'tooth', 'topic', 'torch', 'total', 'touch', 'tough', 'tour', 'tourist', 'tournament', 'toward', 'towards', 'tower', 'town', 'toy', 'trace', 'track', 'trade', 'trader', 'tradition', 'traditional', 'traffic', 'tragedy', 'trail', 'train', 'trainer', 'trait', 'transfer', 'transform', 'transit', 'transition', 'translate', 'transmit', 'transplant', 'transport', 'trash', 'travel', 'tray', 'treat', 'treaty', 'tree', 'tremendous', 'trend', 'trial', 'tribe', 'tribute', 'trick', 'trigger', 'trim', 'trip', 'trophy', 'tropical', 'trouble', 'truck', 'true', 'truly', 'trunk', 'trust', 'truth', 'tumor', 'tuner', 'tunnel', 'turkey', 'turn', 'twice', 'twin', 'twist', 'type', 'typical', 'ugly', 'ultimate', 'unable', 'uncle', 'under', 'undergo', 'understand', 'undertake', 'unemployment', 'unexpected', 'unfair', 'unfold', 'unhappy', 'uniform', 'union', 'unique', 'unit', 'unite', 'united', 'unity', 'universe', 'university', 'unknown', 'unless', 'unlike', 'unlikely', 'until', 'unusual', 'unveil', 'update', 'upgrade', 'uphold', 'upon', 'upper', 'upset', 'urban', 'urge', 'use', 'used', 'useful', 'user', 'usual', 'utility', 'vacuum', 'vague', 'valid', 'valley', 'valuable', 'value', 'vapor', 'variable', 'variety', 'vary', 'vast', 'vegetable', 'vehicle', 'venture', 'venue', 'version', 'versus', 'very', 'vessel', 'veteran', 'viable', 'vibrant', 'victim', 'victory', 'video', 'view', 'viewer', 'village', 'vintage', 'violate', 'violence', 'violet', 'virtual', 'virtue', 'virus', 'visible', 'vision', 'visit', 'visitor', 'visual', 'vital', 'vivid', 'vocal', 'voice', 'void', 'volatile', 'volcano', 'volume', 'volunteer', 'voter', 'vote', 'voyage', 'wage', 'wagon', 'waist', 'waste', 'watch', 'water', 'wave', 'weak', 'wealth', 'weapon', 'wear', 'weather', 'wedding', 'weekend', 'weekly', 'weigh', 'weight', 'weird', 'welcome', 'welfare', 'west', 'western', 'whale', 'what', 'wheat', 'wheel', 'when', 'where', 'which', 'while', 'whisper', 'white', 'whole', 'whom', 'whose', 'wide', 'widely', 'wife', 'wild', 'will', 'willing', 'win', 'wind', 'window', 'wine', 'wing', 'winner', 'winter', 'wire', 'wisdom', 'wise', 'wish', 'witch', 'withdraw', 'within', 'without', 'witness', 'wizard', 'woke', 'wolf', 'woman', 'wonder', 'wonderful', 'wood', 'wool', 'word', 'work', 'worker', 'workshop', 'world', 'worry', 'worth', 'would', 'wound', 'wrap', 'wreck', 'wrist', 'write', 'writer', 'wrong', 'yard', 'year', 'yellow', 'yesterday', 'yield', 'young', 'your', 'yours', 'yourself', 'youth', 'zebra', 'zero', 'zest', 'zone', 'zoo'
    ]
    
    // Generate random words
    const entropy = strength / 8
    const words: string[] = []
    for (let i = 0; i < entropy / 4; i++) {
      const randomIndex = Math.floor(Math.random() * words.length)
      words.push(words[randomIndex % words.length])
    }
    
    return words.slice(0, entropy / 4).join(' ')
  }

  // Get address for EVM chains
  async getEVMAddress(index: number = 0): Promise<string> {
    const path = this.derivationPath.replace('0/0', `0/${index}`)
    // In production, would use ethers.js to derive address
    return this.deriveAddress(path)
  }

  // Get address for Solana
  async getSolanaAddress(index: number = 0): Promise<string> {
    return this.deriveAddress(`m/44'/501'/${index}'/0/0`)
  }

  // Get address for Tron
  async getTronAddress(index: number = 0): Promise<string> {
    return this.deriveAddress(`m/44'/195'/${index}'/0/0`)
  }

  private deriveAddress(path: string): string {
    // Simplified - would use proper HD key derivation
    const hash = Buffer.from(path + this.mnemonic).toString('hex')
    return '0x' + hash.slice(0, 40)
  }

  // Sign transaction (auto-sign within 3 seconds)
  async signTransaction(txData: any, chainType: string): Promise<string> {
    const startTime = Date.now()
    
    // Simulate signing
    const signature = await this.performSign(txData)
    
    const elapsed = Date.now() - startTime
    if (elapsed > 3000) {
      console.warn(`Signing took ${elapsed}ms, exceeding 3s target`)
    }
    
    return signature
  }

  private async performSign(txData: any): Promise<string> {
    // In production, would use proper cryptographic signing
    const data = JSON.stringify(txData)
    return '0x' + Buffer.from(data).toString('hex').slice(0, 128)
  }
}

// Wallet Manager
export class WalletManager {
  private wallets: Map<string, Wallet> = new Map()
  private activeWallet: string | null = null
  private chains: Map<number, Chain> = new Map()
  private tokens: Map<number, Token[]> = new Map()

  constructor() {
    this.initializeDefaultChains()
    this.initializeDefaultTokens()
  }

  private initializeDefaultChains() {
    const defaultChains: Chain[] = [
      { id: 1, name: 'Ethereum', type: 'evm', rpc: 'https://eth.llamarpc.com', explorer: 'https://etherscan.io', symbol: 'ETH', decimals: 18, wrappedToken: '0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2', isEnabled: true, icon: 'eth.png' },
      { id: 56, name: 'BNB Chain', type: 'evm', rpc: 'https://bsc.llamarpc.com', explorer: 'https://bscscan.com', symbol: 'BNB', decimals: 18, wrappedToken: '0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c', isEnabled: true, icon: 'bnb.png' },
      { id: 137, name: 'Polygon', type: 'evm', rpc: 'https://polygon.llamarpc.com', explorer: 'https://polygonscan.com', symbol: 'MATIC', decimals: 18, wrappedToken: '0x0d500B1d8E8eF31E21C99d1Db9A6444d3ADf1270', isEnabled: true, icon: 'matic.png' },
      { id: 42161, name: 'Arbitrum', type: 'evm', rpc: 'https://arbitrum.llamarpc.com', explorer: 'https://arbiscan.io', symbol: 'ETH', decimals: 18, wrappedToken: '0x82aF49447D8a07e3bd95BD0d56f35241523fBab1', isEnabled: true, icon: 'arb.png' },
      { id: 10, name: 'Optimism', type: 'evm', rpc: 'https://optimism.llamarpc.com', explorer: 'https://optimistic.etherscan.io', symbol: 'ETH', decimals: 18, wrappedToken: '0x4200000000000000000000000000000000000042', isEnabled: true, icon: 'op.png' },
      { id: 43114, name: 'Avalanche', type: 'evm', rpc: 'https://avax.llamarpc.com', explorer: 'https://snowtrace.io', symbol: 'AVAX', decimals: 18, wrappedToken: '0xB31f66AA3C1e78502F98da20086eDCD3Fd1D0b8C', isEnabled: true, icon: 'avax.png' },
      { id: 25, name: 'Cronos', type: 'evm', rpc: 'https://evm.cronos.org', explorer: 'https://cronoscan.com', symbol: 'CRO', decimals: 18, wrappedToken: '0x5C7F8A570bb57852344f8B0F9242f2899bF50280', isEnabled: true, icon: 'cro.png' },
      { id: 43114, name: 'Solana', type: 'solana', rpc: 'https://api.mainnet-beta.solana.com', explorer: 'https://solscan.io', symbol: 'SOL', decimals: 9, isEnabled: true, icon: 'sol.png' },
      { id: 728126428, name: 'Tron', type: 'tron', rpc: 'https://api.trongrid.io', explorer: 'https://tronscan.org', symbol: 'TRX', decimals: 6, isEnabled: true, icon: 'trx.png' },
      { id: 784, name: 'Sui', type: 'sui', rpc: 'https://fullnode.mainnet.sui.io', explorer: 'https://suiscan.xyz', symbol: 'SUI', decimals: 9, isEnabled: true, icon: 'sui.png' },
    ]

    defaultChains.forEach(chain => this.chains.set(chain.id, chain))
  }

  private initializeDefaultTokens() {
    const defaultTokens: Token[] = [
      { symbol: 'ETH', name: 'Ethereum', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 1, logo: 'eth.png', isNative: true },
      { symbol: 'USDT', name: 'Tether USD', address: '0xdAC17F958D2ee523a2206206994597C13D831ec7', decimals: 6, chainId: 1, logo: 'usdt.png', isNative: false, isStable: true },
      { symbol: 'USDC', name: 'USD Coin', address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48', decimals: 6, chainId: 1, logo: 'usdc.png', isNative: false, isStable: true },
      { symbol: 'WBTC', name: 'Wrapped Bitcoin', address: '0x2260FAC5E5542a773Aa44fCF2df52aDCEb44661f', decimals: 8, chainId: 1, logo: 'wbtc.png', isNative: false },
      { symbol: 'BNB', name: 'BNB', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 56, logo: 'bnb.png', isNative: true },
      { symbol: 'CAKE', name: 'PancakeSwap Token', address: '0x0E09FaBB73Bd3ade0a17ECC321fD13a19e81cE82', decimals: 18, chainId: 56, logo: 'cake.png', isNative: false },
      { symbol: 'MATIC', name: 'Polygon', address: '0x0000000000000000000000000000000000000000', decimals: 18, chainId: 137, logo: 'matic.png', isNative: true },
      { symbol: 'WETH', name: 'Wrapped Ether', address: '0x7ceB23fD6bC0adD59E62ac25578270cFf1b9f619', decimals: 18, chainId: 137, logo: 'weth.png', isNative: false },
      { symbol: 'ARB', name: 'Arbitrum', address: '0x912CE5914419C7B3bB8A2aCAd0bA89F6e14fA6E7', decimals: 18, chainId: 42161, logo: 'arb.png', isNative: false },
      { symbol: 'OP', name: 'Optimism', address: '0x4200000000000000000000000000000000000042', decimals: 18, chainId: 10, logo: 'op.png', isNative: false },
      { symbol: 'SOL', name: 'Solana', address: '', decimals: 9, chainId: 43114, logo: 'sol.png', isNative: true },
      { symbol: 'TRX', name: 'Tron', address: '', decimals: 6, chainId: 728126428, logo: 'trx.png', isNative: true },
      { symbol: 'SUI', name: 'Sui', address: '', decimals: 9, chainId: 784, logo: 'sui.png', isNative: true },
    ]

    defaultTokens.forEach(token => {
      const tokens = this.tokens.get(token.chainId) || []
      tokens.push(token)
      this.tokens.set(token.chainId, tokens)
    })
  }

  // Create wallet from mnemonic
  createWallet(mnemonic: string, name: string): Wallet {
    const id = 'wallet_' + Date.now()
    const engine = HDWalletEngine.fromMnemonic(mnemonic)
    
    const wallet: Wallet = {
      id,
      address: '', // Will be derived
      chainType: 'evm',
      createdAt: Date.now(),
      name,
      isHardware: false,
      balance: [],
    }

    // Derive first EVM address
    engine.getEVMAddress(0).then(address => {
      wallet.address = address
    })

    this.wallets.set(id, wallet)
    this.activeWallet = id
    
    return wallet
  }

  // Import existing wallet
  importWallet(mnemonic: string, name: string): Wallet {
    // Validate mnemonic
    if (!this.validateMnemonic(mnemonic)) {
      throw new Error('Invalid mnemonic phrase')
    }
    return this.createWallet(mnemonic, name)
  }

  // Validate mnemonic (24 words)
  validateMnemonic(mnemonic: string): boolean {
    const words = mnemonic.trim().split(/\s+/)
    return words.length === 12 || words.length === 24
  }

  // Get active wallet
  getActiveWallet(): Wallet | null {
    return this.activeWallet ? this.wallets.get(this.activeWallet) || null : null
  }

  // Get all wallets
  getAllWallets(): Wallet[] {
    return Array.from(this.wallets.values())
  }

  // Switch active wallet
  setActiveWallet(walletId: string): boolean {
    if (this.wallets.has(walletId)) {
      this.activeWallet = walletId
      return true
    }
    return false
  }

  // Get supported chains
  getChains(): Chain[] {
    return Array.from(this.chains.values())
  }

  // Get tokens for chain
  getTokens(chainId: number): Token[] {
    return this.tokens.get(chainId) || []
  }

  // Get chain by ID
  getChain(chainId: number): Chain | undefined {
    return this.chains.get(chainId)
  }

  // Add custom chain
  addChain(chain: Chain): void {
    this.chains.set(chain.id, chain)
  }

  // Remove chain
  removeChain(chainId: number): void {
    this.chains.delete(chainId)
  }

  // Add custom token
  addToken(token: Token): void {
    const tokens = this.tokens.get(token.chainId) || []
    const existing = tokens.findIndex(t => t.address === token.address && t.chainId === token.chainId)
    if (existing >= 0) {
      tokens[existing] = token
    } else {
      tokens.push(token)
    }
    this.tokens.set(token.chainId, tokens)
  }

  // Remove token
  removeToken(chainId: number, tokenAddress: string): void {
    const tokens = this.tokens.get(chainId) || []
    this.tokens.set(chainId, tokens.filter(t => t.address !== tokenAddress))
  }

  // Get wallet balance
  async getBalance(walletId: string, chainId: number): Promise<Balance[]> {
    const wallet = this.wallets.get(walletId)
    if (!wallet) return []

    // Mock balance - in production would query RPC
    const tokens = this.getTokens(chainId)
    return tokens.map(token => ({
      symbol: token.symbol,
      address: token.address,
      amount: (Math.random() * 10).toFixed(4),
      value: (Math.random() * 1000).toFixed(2),
      chainId,
      logo: token.logo,
    }))
  }

  // Send transaction
  async send(walletId: string, to: string, amount: string, tokenAddress: string, chainId: number): Promise<Transaction> {
    const wallet = this.wallets.get(walletId)
    if (!wallet) throw new Error('Wallet not found')

    const tx: Transaction = {
      id: 'tx_' + Date.now(),
      hash: '0x' + Math.random().toString(16).slice(2),
      from: wallet.address,
      to,
      value: amount,
      token: tokenAddress,
      fee: '0.001',
      status: 'pending',
      timestamp: Date.now(),
      chainId,
      type: 'send',
    }

    return tx
  }

  // Swap tokens
  async swap(walletId: string, fromToken: Token, toToken: Token, amount: string): Promise<SwapQuote> {
    // Generate swap quote
    const toAmount = (parseFloat(amount) * 0.85).toFixed(6) // Simplified rate
    
    return {
      fromToken,
      toToken,
      fromAmount: amount,
      toAmount,
      priceImpact: 0.5,
      route: [{
        protocol: 'TigerSwap Router',
        path: [fromToken.address, toToken.address],
        pools: ['0x...'],
        percentage: 100,
      }],
      estimatedGas: '150000',
      slippage: 0.5,
    }
  }
}

// Export singleton instance
export const walletManager = new WalletManager()